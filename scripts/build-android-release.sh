#!/bin/sh
set -eu
umask 077

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
init_key=false
case "${1:-}" in
    "") ;;
    --init-key) init_key=true ;;
    --help)
        printf '%s\n' 'Usage: scripts/build-android-release.sh [--init-key]' \
            'Builds and verifies a signed ARM64 release without installing it.' \
            '--init-key creates the private signing key only if no key/password exists.'
        exit 0 ;;
    *) printf 'Unknown argument: %s\n' "$1" >&2; exit 1 ;;
esac
if [ "$#" -gt 1 ]; then
    printf '%s\n' 'Only one optional argument is supported.' >&2
    exit 1
fi

android_sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}
java_home_path=${JAVA_HOME:-}
ebiten_version=v2.9.11
android_api=23
compile_sdk=36
build_tools_version=36.0.0
ndk_version=28.2.13676358
application_id=com.olivierh.populous
key_dir="$project_root/android/keystore"
key_path="$key_dir/populous-release.p12"
password_path="$key_dir/populous-release.password"
key_alias=populous
dist_dir="$project_root/dist/android"

fail() { printf '%s\n' "$*" >&2; exit 1; }

if [ -z "$android_sdk" ] && [ -d /opt/homebrew/share/android-commandlinetools ]; then
    android_sdk=/opt/homebrew/share/android-commandlinetools
fi
if [ -z "$android_sdk" ] && [ -d "$HOME/Library/Android/sdk" ]; then
    android_sdk=$HOME/Library/Android/sdk
fi
if [ -z "$java_home_path" ] && [ -d /opt/homebrew/opt/openjdk@17 ]; then
    java_home_path=/opt/homebrew/opt/openjdk@17
fi
if [ -z "$java_home_path" ] && [ -d /Applications/Android\ Studio.app/Contents/jbr/Contents/Home ]; then
    java_home_path=/Applications/Android\ Studio.app/Contents/jbr/Contents/Home
fi
[ -n "$android_sdk" ] || fail 'Set ANDROID_HOME or ANDROID_SDK_ROOT to the Android SDK.'
[ -f "$android_sdk/platforms/android-$compile_sdk/android.jar" ] || fail "Android SDK Platform $compile_sdk is required."
[ -d "$android_sdk/ndk/$ndk_version" ] || fail "Android NDK $ndk_version is required."
[ -n "$java_home_path" ] && [ -x "$java_home_path/bin/java" ] || fail 'Java 17 is required; set JAVA_HOME.'
"$java_home_path/bin/java" -version 2>&1 | grep -q 'version "17\.' || fail 'JAVA_HOME must select Java 17.'
for command_name in go openssl unzip awk shasum; do
    command -v "$command_name" >/dev/null 2>&1 || fail "Missing tool: $command_name"
done
for android_tool in aapt apksigner zipalign; do
    [ -x "$android_sdk/build-tools/$build_tools_version/$android_tool" ] || fail "Android Build Tools $build_tools_version are required."
done
case "$(uname -s)" in
    Darwin) ndk_host=darwin-x86_64 ;;
    Linux) ndk_host=linux-x86_64 ;;
    *) fail 'This script supports the Android NDK on macOS and Linux.' ;;
esac
readelf_path="$android_sdk/ndk/$ndk_version/toolchains/llvm/prebuilt/$ndk_host/bin/llvm-readelf"
[ -x "$readelf_path" ] || fail "Missing NDK tool: $readelf_path"
[ -x "$project_root/android/gradlew" ] || fail 'Missing Android Gradle wrapper.'

export ANDROID_HOME="$android_sdk" ANDROID_SDK_ROOT="$android_sdk"
export ANDROID_NDK_HOME="$android_sdk/ndk/$ndk_version"
export JAVA_HOME="$java_home_path"
export PATH="$java_home_path/bin:$PATH"
# The maximum page size alone does not guarantee a 16 KiB RELRO boundary.
# Pass both flags through cgo: ebitenmobile appends its own -extldflags for SONAME.
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384"
cd "$project_root"
module_ebiten_version=$(go list -m -f '{{.Version}}' github.com/hajimehoshi/ebiten/v2)
[ "$module_ebiten_version" = "$ebiten_version" ] || fail "Ebitengine version mismatch: $module_ebiten_version / $ebiten_version"

if [ ! -f "$key_path" ] || [ ! -f "$password_path" ]; then
    [ ! -e "$key_path" ] && [ ! -e "$password_path" ] || fail 'Incomplete signing material: restore the existing key/password; no files were replaced.'
    [ "$init_key" = true ] || fail 'No signing key. Run once with --init-key, then back up android/keystore/ securely.'
    mkdir -p "$key_dir"
    chmod 700 "$key_dir"
    openssl rand -base64 -out "$password_path" 48
    "$java_home_path/bin/keytool" -genkeypair -noprompt \
        -keystore "$key_path" -storetype PKCS12 -alias "$key_alias" \
        -keyalg RSA -keysize 3072 -sigalg SHA256withRSA -validity 18263 \
        -dname 'CN=Populous Android' \
        -storepass:file "$password_path" -keypass:file "$password_path"
    printf '%s\n' 'Signing key created. Back up android/keystore/ before distributing the APK.'
fi
chmod 700 "$key_dir"
chmod 600 "$key_path" "$password_path"
# Check the key before spending time compiling; only file paths enter argv.
"$java_home_path/bin/keytool" -list -keystore "$key_path" -alias "$key_alias" \
    -storepass:file "$password_path" >/dev/null

mkdir -p "$project_root/android/app/libs" "$dist_dir"
release_work=$(mktemp -d "${TMPDIR:-/tmp}/populous-release.XXXXXX")
# Only the directory created by mktemp above is removed, never key/dist files.
trap 'rm -rf -- "$release_work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

printf '%s\n' 'Building the stripped ARM64 Go/Ebitengine library...'
go run "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@$ebiten_version" bind \
    -target android/arm64 -androidapi "$android_api" \
    -javapkg "$application_id" -trimpath -ldflags '-s -w' \
    -o android/app/libs/populous.aar ./mobile

printf '%s\n' 'Building the optimized unsigned release...'
"$project_root/android/gradlew" -p "$project_root/android" --console=plain assembleRelease
unsigned_apk="$project_root/android/app/build/outputs/apk/release/app-release-unsigned.apk"
[ -f "$unsigned_apk" ] || fail "Missing unsigned release: $unsigned_apk"
aapt_path="$android_sdk/build-tools/$build_tools_version/aapt"
apksigner_path="$android_sdk/build-tools/$build_tools_version/apksigner"
zipalign_path="$android_sdk/build-tools/$build_tools_version/zipalign"
badging=$("$aapt_path" dump badging "$unsigned_apk")
printf '%s\n' "$badging" | grep -q "package: name='$application_id'" || fail 'Wrong application ID.'
printf '%s\n' "$badging" | grep -q "sdkVersion:'$android_api'" || fail 'Wrong minimum Android API.'
printf '%s\n' "$badging" | grep -q "targetSdkVersion:'$compile_sdk'" || fail 'Wrong target Android API.'
printf '%s\n' "$badging" | grep -q "native-code: 'arm64-v8a'$" || fail 'The release must contain ARM64 only.'
if printf '%s\n' "$badging" | grep -q '^application-debuggable'; then
    fail 'Refusing to distribute a debuggable application.'
fi
version_name=$(printf '%s\n' "$badging" | sed -n "s/^package:.*versionName='\([^']*\)'.*/\1/p")
version_code=$(printf '%s\n' "$badging" | sed -n "s/^package:.*versionCode='\([^']*\)'.*/\1/p")
case "$version_name" in
    ''|*[!0-9A-Za-z._-]*) fail 'Invalid release versionName for the output filename.' ;;
esac
apk_name="populous-android-$version_name-arm64.apk"
signed_apk="$release_work/$apk_name"

"$zipalign_path" -f -P 16 4 "$unsigned_apk" "$release_work/aligned.apk"
# PKCS12 uses the store password for this key. apksigner tries it automatically;
# specifying the same password file twice would consume a nonexistent second line.
"$apksigner_path" sign --ks "$key_path" --ks-key-alias "$key_alias" \
    --ks-pass "file:$password_path" \
    --v4-signing-enabled false --out "$signed_apk" "$release_work/aligned.apk"
"$apksigner_path" verify --verbose --print-certs "$signed_apk" > "$release_work/SIGNATURE.txt"
"$zipalign_path" -c -P 16 4 "$signed_apk"

printf '%s\n' 'Checking every native library: uncompressed packaging and 16 KiB ELF segments...'
unzip -lv "$signed_apk" 'lib/*/*.so' | awk '
    $8 ~ /^lib\/.*\.so$/ { count++; if ($2 != "Stored") bad=1 }
    END { if (!count || bad) exit 1 }
' || fail 'Native libraries must be stored uncompressed in the APK.'
unzip -Z1 "$signed_apk" | awk '/^lib\/.*\.so$/ { print }' > "$release_work/native-files.txt"
[ -s "$release_work/native-files.txt" ] || fail 'No native library found.'
while IFS= read -r library; do
    unzip -p "$signed_apk" "$library" > "$release_work/library.so"
    "$readelf_path" -Wl "$release_work/library.so" > "$release_work/elf.txt"
    awk '
        function hex(s, n,i,d) {
            sub(/^0x/, "", s); n=0
            for (i=1; i<=length(s); i++) {
                d=index("0123456789abcdef", tolower(substr(s,i,1)))-1
                if (d < 0) return -1
                n=n*16+d
            }
            return n
        }
        $1 == "LOAD" {
            count++
            if (hex($NF) < 16384 || hex($2)%16384 != hex($3)%16384) bad=1
        }
        $1 == "GNU_RELRO" { if ((hex($3)+hex($6))%16384 != 0) bad=1 }
        END { if (!count || bad) exit 1 }
    ' "$release_work/elf.txt" || fail "Non-16-KiB-compatible ELF segments: $library"
    printf '%s: 16 KiB LOAD/RELRO alignment verified\n' "$library"
done < "$release_work/native-files.txt"

# Publish only verified output. Retain an older binary if its contents changed.
if [ -f "$dist_dir/$apk_name" ] && ! cmp -s "$signed_apk" "$dist_dir/$apk_name"; then
    archive_dir=$(mktemp -d "$dist_dir/previous.XXXXXX")
    cp "$dist_dir/$apk_name" "$archive_dir/$apk_name"
fi
cp "$signed_apk" "$dist_dir/$apk_name"
cp "$release_work/SIGNATURE.txt" "$dist_dir/SIGNATURE.txt"
"$java_home_path/bin/keytool" -exportcert -rfc -keystore "$key_path" -alias "$key_alias" \
    -storepass:file "$password_path" -file "$dist_dir/populous-release-cert.pem"
(
    cd "$dist_dir"
    shasum -a 256 "$apk_name" > SHA256SUMS
)
{
    printf 'Populous Android release\nApplication ID: %s\nVersion: %s (%s)\n' "$application_id" "$version_name" "$version_code"
    printf 'ABI: arm64-v8a\nMinimum API: %s\nTarget API: %s\n' "$android_api" "$compile_sdk"
    printf 'Ebitengine: %s\nNDK: %s\nBuild Tools: %s\n' "$ebiten_version" "$ndk_version" "$build_tools_version"
    go version
    printf 'Built UTC: %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf 'Optimizations: Go -trimpath -ldflags "-s -w"; R8; resource shrinking; ARM64 only\n'
    printf 'Native linker: max-page-size=16384; common-page-size=16384\n'
    printf 'Verified: APK signature; non-debuggable; native uncompressed; ZIP/ELF 16 KiB alignment\n'
    printf 'Installation: signed release APK; cannot update a differently signed debug installation.\n'
} > "$dist_dir/RELEASE.txt"
chmod 644 "$dist_dir/$apk_name" "$dist_dir/SHA256SUMS" "$dist_dir/SIGNATURE.txt" \
    "$dist_dir/RELEASE.txt" "$dist_dir/populous-release-cert.pem"
printf '\nVerified APK: %s\n' "$dist_dir/$apk_name"
printf '%s\n' 'No device was modified. Distribute the APK with SHA256SUMS; retain the signing key privately.'
