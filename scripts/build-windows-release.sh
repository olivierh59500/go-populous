#!/bin/sh
set -eu
umask 077
export LC_ALL=C

usage() {
    printf '%s\n' \
        'Usage: scripts/build-windows-release.sh' \
        'Builds and verifies optimized Windows GUI executables for x86, x64, and ARM64.' \
        'Outputs are published under dist/windows/; an existing different release is archived.' \
        'No Windows code-signing certificate is required or used.'
}

case "${1:-}" in
    "") ;;
    --help|-h) usage; exit 0 ;;
    *) printf 'Unknown argument: %s\n' "$1" >&2; usage >&2; exit 1 ;;
esac
[ "$#" -le 1 ] || { usage >&2; exit 1; }

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version_file="$project_root/VERSION"
resource_config="$project_root/build/windows/winres.json"
resource_prefix="$project_root/cmd/populous/rsrc"
dist_dir="$project_root/dist/windows"
winres_version=v0.3.3
release_work=
remove_resources=false

resource_386="${resource_prefix}_windows_386.syso"
resource_amd64="${resource_prefix}_windows_amd64.syso"
resource_arm64="${resource_prefix}_windows_arm64.syso"

fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }

cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if [ "$remove_resources" = true ]; then
        rm -f -- "$resource_386" "$resource_amd64" "$resource_arm64"
    fi
    if [ -n "$release_work" ] && [ -d "$release_work" ]; then
        rm -rf -- "$release_work"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

for command_name in go git file awk sed grep cmp mktemp env wc cat mkdir cp chmod mv rm rmdir; do
    command -v "$command_name" >/dev/null 2>&1 || fail "Missing tool: $command_name"
done
if command -v shasum >/dev/null 2>&1; then
    sha256_command=shasum
elif command -v sha256sum >/dev/null 2>&1; then
    sha256_command=sha256sum
else
    fail 'Missing SHA-256 tool: install shasum or sha256sum.'
fi

[ -f "$version_file" ] || fail 'Missing canonical VERSION file.'
version=$(sed -n '1p' "$version_file")
[ "$(wc -l < "$version_file" | awk '{print $1}')" -eq 1 ] || fail 'VERSION must contain exactly one line.'
case "$version" in
    ''|*[!0-9.]*) fail 'VERSION must use the numeric X.Y.Z format.' ;;
esac
printf '%s\n' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || fail 'VERSION must use the numeric X.Y.Z format.'

[ -f "$resource_config" ] || fail "Missing Windows resource configuration: $resource_config"
[ -f "$project_root/android/app/src/main/res/mipmap-xxxhdpi/ic_launcher.png" ] || fail 'Missing source icon for Windows resources.'
grep -Fq '"ProductVersion": "'"$version"'"' "$resource_config" || fail 'Windows ProductVersion does not match VERSION.'
grep -Fq '"file_version": "'"$version"'.0"' "$resource_config" || fail 'Windows file_version does not match VERSION.'
grep -Fq '"product_version": "'"$version"'.0"' "$resource_config" || fail 'Windows product_version does not match VERSION.'

cd "$project_root"
[ "$(go env GOMOD)" = "$project_root/go.mod" ] || fail 'Run this script from the go-populous module.'
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail 'A Git worktree is required for -buildvcs=true release metadata.'
git_revision=$(git rev-parse HEAD)
if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
    git_state=modified
else
    git_state=clean
fi
for resource_file in "$resource_386" "$resource_amd64" "$resource_arm64"; do
    [ ! -e "$resource_file" ] || fail "Refusing to replace an existing generated resource: $resource_file"
done

release_work=$(mktemp -d "${TMPDIR:-/tmp}/populous-windows-release.XXXXXX")
output_dir="$release_work/output"
cache_dir="$release_work/go-build-cache"
mkdir -p "$output_dir" "$cache_dir"

# Network peers must reject binaries built from different simulation sources,
# even when their human-facing VERSION is identical. Hash the same sorted input
# set once, before architecture-specific builds, so all three targets interoperate.
cat > "$release_work/source-fingerprint.go" <<'GOEOF'
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "source fingerprint: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		fatal("usage: source-fingerprint module-root")
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		fatal("resolve module root: %v", err)
	}
	var paths []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".git" || rel == "previous" || rel == "dist" || rel == "recordings" ||
				strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, "previous/") ||
				strings.HasPrefix(rel, "dist/") || strings.HasPrefix(rel, "recordings/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		include := rel == "go.mod" || rel == "go.sum" || strings.HasSuffix(rel, ".go")
		if strings.HasPrefix(rel, "assets/") && !strings.HasPrefix(filepath.Base(rel), ".") {
			include = true
		}
		if include {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		fatal("walk module: %v", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		fatal("no source inputs found")
	}
	hash := sha256.New()
	hash.Write([]byte("go-populous-network-source-v1\x00"))
	var size [8]byte
	for _, rel := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			fatal("read %s: %v", rel, err)
		}
		binary.LittleEndian.PutUint64(size[:], uint64(len(rel)))
		hash.Write(size[:])
		hash.Write([]byte(rel))
		binary.LittleEndian.PutUint64(size[:], uint64(len(content)))
		hash.Write(size[:])
		hash.Write(content)
	}
	fmt.Printf("%x\n", hash.Sum(nil))
}
GOEOF
network_fingerprint=$(GOCACHE="$cache_dir" go run "$release_work/source-fingerprint.go" "$project_root")
printf '%s\n' "$network_fingerprint" | grep -Eq '^[0-9a-f]{64}$' || fail 'Invalid deterministic network source fingerprint.'
linker_flags="-s -w -H=windowsgui -X go-populous/internal/game.networkReleaseFingerprint=$network_fingerprint"

# go-winres must place COFF resources next to the main package for go build to
# select the architecture-specific file. These exact temporary files did not
# exist above and are always removed by the EXIT trap.
remove_resources=true
printf '%s\n' "Generating pinned Windows resources (go-winres $winres_version)..."
GOCACHE="$cache_dir" go run "github.com/tc-hib/go-winres@$winres_version" make \
    --in "$resource_config" --arch 386,amd64,arm64 --out "$resource_prefix"
for resource_file in "$resource_386" "$resource_amd64" "$resource_arm64"; do
    [ -s "$resource_file" ] || fail "Windows resource generation failed: $resource_file"
done

cat > "$release_work/verify-pe.go" <<'GOEOF'
package main

import (
	"bytes"
	"debug/buildinfo"
	"debug/pe"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	machineI386  = 0x014c
	machineAMD64 = 0x8664
	machineARM64 = 0xaa64
	guiSubsystem = 2
	executable   = 0x0002
	dll          = 0x2000
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "PE verification: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 5 {
		fatal("usage: verifier executable goarch network-fingerprint embedded-asset...")
	}
	path, arch, fingerprint := os.Args[1], os.Args[2], os.Args[3]
	wantMachine := map[string]uint16{"386": machineI386, "amd64": machineAMD64, "arm64": machineARM64}[arch]
	if wantMachine == 0 {
		fatal("unsupported architecture %q", arch)
	}
	f, err := pe.Open(path)
	if err != nil {
		fatal("open %s: %v", path, err)
	}
	defer f.Close()
	if f.Machine != wantMachine {
		fatal("%s machine is %#x, expected %#x", filepath.Base(path), f.Machine, wantMachine)
	}
	if f.Characteristics&executable == 0 || f.Characteristics&dll != 0 {
		fatal("%s is not a standalone executable", filepath.Base(path))
	}
	subsystem := uint16(0)
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		subsystem = h.Subsystem
	case *pe.OptionalHeader64:
		subsystem = h.Subsystem
	default:
		fatal("%s has no supported PE optional header", filepath.Base(path))
	}
	if subsystem != guiSubsystem {
		fatal("%s subsystem is %d, expected Windows GUI (%d)", filepath.Base(path), subsystem, guiSubsystem)
	}
	resourceFound := false
	for _, section := range f.Sections {
		if section.Name == ".rsrc" && section.Size > 0 {
			resourceFound = true
		}
	}
	if !resourceFound {
		fatal("%s has no non-empty .rsrc section", filepath.Base(path))
	}
	importedSymbols, err := f.ImportedSymbols()
	if err != nil {
		fatal("read imports from %s: %v", filepath.Base(path), err)
	}
	libraries := make(map[string]struct{})
	for _, symbol := range importedSymbols {
		separator := strings.LastIndexByte(symbol, ':')
		if separator < 0 || separator == len(symbol)-1 {
			fatal("%s has malformed imported symbol %q", filepath.Base(path), symbol)
		}
		libraries[symbol[separator+1:]] = struct{}{}
	}
	if len(libraries) != 1 {
		fatal("%s has unexpected static DLL imports: %v", filepath.Base(path), libraries)
	}
	for library := range libraries {
		if !strings.EqualFold(library, "kernel32.dll") {
			fatal("%s has unexpected static DLL import: %s", filepath.Base(path), library)
		}
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		fatal("read Go build information from %s: %v", filepath.Base(path), err)
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	for key, want := range map[string]string{"GOOS": "windows", "GOARCH": arch, "CGO_ENABLED": "0", "-buildmode": "exe"} {
		if settings[key] != want {
			fatal("%s build setting %s is %q, expected %q", filepath.Base(path), key, settings[key], want)
		}
	}
	binary, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", filepath.Base(path), err)
	}
	if !bytes.Contains(binary, []byte(fingerprint)) {
		fatal("%s does not contain injected network fingerprint %s", filepath.Base(path), fingerprint)
	}
	for _, assetPath := range os.Args[4:] {
		asset, err := os.ReadFile(assetPath)
		if err != nil {
			fatal("read embedded-asset reference %s: %v", assetPath, err)
		}
		if len(asset) == 0 || !bytes.Contains(binary, asset) {
			fatal("%s does not contain embedded asset %s", filepath.Base(path), assetPath)
		}
	}
	fmt.Printf("%s: PE machine, GUI subsystem, resources, imports, build metadata, and embedded assets verified\n", filepath.Base(path))
}
GOEOF

asset_level="$project_root/assets/amiga/level.dat"
asset_sprites="$project_root/assets/amiga/sprites0.dat"
asset_title="$project_root/assets/extracted-images/load.pic.png"
for asset_file in "$asset_level" "$asset_sprites" "$asset_title"; do
    [ -s "$asset_file" ] || fail "Missing embedded-asset reference: $asset_file"
done

printf '%s\n' 'Building stripped Windows GUI executables...'
for goarch in 386 amd64 arm64; do
    case "$goarch" in
        386)
            label=x86
            expected_file='PE32 executable (GUI) Intel 80386'
            architecture_env=GO386=sse2
            ;;
        amd64)
            label=x64
            expected_file='PE32+ executable (GUI) x86-64'
            architecture_env=GOAMD64=v1
            ;;
        arm64)
            label=arm64
            expected_file='PE32+ executable (GUI) Aarch64'
            architecture_env=GOARM64=v8.0
            ;;
    esac
    executable_name="populous-windows-$label-$version.exe"
    executable_path="$output_dir/$executable_name"
    cgo_raw_report="$release_work/cgo-$goarch.raw.txt"
    cgo_report="$release_work/cgo-$goarch.txt"

    if ! env CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" "$architecture_env" GOCACHE="$cache_dir" \
        go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}: {{join .CgoFiles ","}}{{end}}' \
        ./cmd/populous > "$cgo_raw_report"; then
        fail "Could not inspect cgo dependencies for windows/$goarch."
    fi
    sed '/^[[:space:]]*$/d' "$cgo_raw_report" > "$cgo_report"
    if [ -s "$cgo_report" ]; then
        sed 's/^/  /' "$cgo_report" >&2
        fail "Unexpected cgo dependencies for windows/$goarch."
    fi

    env CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" "$architecture_env" GOCACHE="$cache_dir" \
        go build -trimpath -buildvcs=true -ldflags "$linker_flags" \
        -o "$executable_path" ./cmd/populous
    [ -s "$executable_path" ] || fail "Missing executable: $executable_path"
    file_description=$(file -b "$executable_path")
    case "$file_description" in
        *"$expected_file"*) ;;
        *) fail "$executable_name has unexpected file type: $file_description" ;;
    esac
    GOCACHE="$cache_dir" go run "$release_work/verify-pe.go" "$executable_path" "$goarch" "$network_fingerprint" \
        "$asset_level" "$asset_sprites" "$asset_title"
done

(
    cd "$output_dir"
    if [ "$sha256_command" = shasum ]; then
        shasum -a 256 \
            "populous-windows-x86-$version.exe" \
            "populous-windows-x64-$version.exe" \
            "populous-windows-arm64-$version.exe" > SHA256SUMS
        shasum -a 256 -c SHA256SUMS >/dev/null
    else
        sha256sum \
            "populous-windows-x86-$version.exe" \
            "populous-windows-x64-$version.exe" \
            "populous-windows-arm64-$version.exe" > SHA256SUMS
        sha256sum -c SHA256SUMS >/dev/null
    fi
)

{
    printf 'Populous Windows release\n'
    printf 'Version: %s\n' "$version"
    printf 'Source revision: %s (%s worktree)\n' "$git_revision" "$git_state"
    printf 'Network source fingerprint: %s\n' "$network_fingerprint"
    printf 'Targets: windows/386 GO386=sse2; windows/amd64 GOAMD64=v1; windows/arm64 GOARM64=v8.0\n'
    printf 'Go: %s\n' "$(go version)"
    printf 'Resource generator: github.com/tc-hib/go-winres@%s\n' "$winres_version"
    printf 'Optimizations: CGO_ENABLED=0; -trimpath; -buildvcs=true; stripped Windows GUI subsystem\n'
    printf 'Verified: PE machine; GUI subsystem; .rsrc; only kernel32.dll statically imported; Go build metadata and network fingerprint; embedded assets; SHA-256\n'
    printf 'Authenticode: unsigned (no private Windows code-signing certificate configured)\n'
    printf 'Runtime compatibility: Windows 10 or later, selected architecture\n'
} > "$output_dir/RELEASE.txt"

mkdir -p "$dist_dir"
publish_dir=$(mktemp -d "$dist_dir/.publish.XXXXXX")
publish_names="populous-windows-x86-$version.exe populous-windows-x64-$version.exe populous-windows-arm64-$version.exe SHA256SUMS RELEASE.txt"
for output_name in $publish_names; do
    cp "$output_dir/$output_name" "$publish_dir/$output_name"
    chmod 644 "$publish_dir/$output_name"
done

different=false
existing=false
for output_name in $publish_names; do
    if [ -f "$dist_dir/$output_name" ]; then
        existing=true
        cmp -s "$publish_dir/$output_name" "$dist_dir/$output_name" || different=true
    else
        different=true
    fi
done
if [ "$existing" = true ] && [ "$different" = true ]; then
    archive_dir=$(mktemp -d "$dist_dir/previous.XXXXXX")
    for output_name in $publish_names; do
        if [ -f "$dist_dir/$output_name" ]; then
            cp "$dist_dir/$output_name" "$archive_dir/$output_name"
            chmod 644 "$archive_dir/$output_name"
        fi
    done
    printf 'Previous differing release archived in: %s\n' "$archive_dir"
fi
for output_name in $publish_names; do
    mv -f "$publish_dir/$output_name" "$dist_dir/$output_name"
done
rmdir "$publish_dir"
(
    cd "$dist_dir"
    if [ "$sha256_command" = shasum ]; then
        shasum -a 256 -c SHA256SUMS >/dev/null
    else
        sha256sum -c SHA256SUMS >/dev/null
    fi
)

printf '\nVerified Windows release %s:\n' "$version"
for output_name in $publish_names; do
    printf '  %s/%s\n' "$dist_dir" "$output_name"
done
printf '%s\n' 'The executables are unsigned; see docs/WINDOWS_RELEASE.md before distribution.'
