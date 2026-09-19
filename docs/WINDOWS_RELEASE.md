# Windows release

The macOS/Linux release pipeline cross-compiles three self-contained Windows
executables from the same source and embedded game data:

| File suffix | Go target | Intended systems |
| --- | --- | --- |
| `x86` | `windows/386`, `GO386=sse2` | 32-bit Windows 10 on Intel/AMD |
| `x64` | `windows/amd64`, `GOAMD64=v1` | 64-bit Windows 10/11 on Intel/AMD |
| `arm64` | `windows/arm64`, `GOARM64=v8.0` | Windows 10/11 on ARM64 |

Run the following command at the repository root:

```sh
./scripts/build-windows-release.sh
```

The first invocation may download the pinned
`github.com/tc-hib/go-winres@v0.3.3` resource generator through the Go module
proxy. The build itself uses `CGO_ENABLED=0`; no Windows compiler, SDK, Wine, or
virtual machine is required. `VERSION` is the canonical release name and the
resource metadata in `build/windows/winres.json` must match it. When publishing
a new version, also replace the three exact Windows executable allow-list
entries in `.gitignore`; older and intermediate binaries intentionally remain
ignored.

## Published files

For version `1.1.0`, the script atomically publishes individually usable files
under `dist/windows/`:

```text
populous-windows-x86-1.1.0.exe
populous-windows-x64-1.1.0.exe
populous-windows-arm64-1.1.0.exe
SHA256SUMS
RELEASE.txt
```

The executables use the Windows GUI subsystem, so opening one does not create a
second console window. They contain the application icon, a Windows 10
manifest, version information, Ebitengine, and all required game assets. They
are portable: there is no installer and no external data directory to copy.
The desktop save file `go-populous.sav` is written in the process working
directory, as with other desktop builds.

The script never silently discards a different release with the same name. If
current destination files differ, it copies them into a unique
`dist/windows/previous.*` directory before publishing the verified build. It
also refuses to replace pre-existing `.syso` files in `cmd/populous/`; its own
temporary resources and build cache are removed on normal exit or interruption.

## Verification performed

Each target is compiled with:

```text
CGO_ENABLED=0 -trimpath -buildvcs=true -ldflags "-s -w -H=windowsgui -X ...networkReleaseFingerprint=<SHA-256>"
```

Before compiling, a small temporary Go helper hashes a sorted, length-delimited
set containing every Go source file in the module, `go.mod`, `go.sum`, and the
non-hidden contents of `assets/`. It excludes `.git`, `previous/`, `dist/`, and
`recordings/`. The resulting source fingerprint is injected identically into
all three architectures and recorded in `RELEASE.txt`. Multiplayer therefore
accepts x86, x64, and ARM64 peers built from the same simulation sources while
rejecting a peer built from divergent source or game data, even if both builds
use the same display version.

Before publication, the script verifies:

- that no selected package contains a cgo source dependency;
- the PE32/PE32+ machine type and Windows GUI subsystem;
- the application icon, manifest, and version-resource section;
- that the static PE import table contains only the Go runtime's
  `kernel32.dll` dependency;
- Go build metadata (`windows`, architecture, executable mode, cgo disabled)
  and the injected network source fingerprint;
- representative embedded level, sprite, and title assets;
- all three SHA-256 checksums.

`file`, Git, Go, and either `shasum` or `sha256sum` are required on the build
host. The release is cross-compiled and structurally inspected on macOS/Linux;
graphics, sound, input, save/load, TCP multiplayer, and display scaling must
still be smoke-tested on real Windows systems for each architecture before a
public release.

The 20 September 2026 rebuild retains version `1.1.0` and the tracked filenames,
but includes the current strategic AI. All three targets passed the structural
and checksum checks above. Their common network source fingerprint is
`c3dfe61306ee299cf36b60e2cf6c1699d4779cb0c69d97a9866e41f2e077c876`.
The previous files are retained in an ignored local archive. This rebuild was
not executed on Windows hardware or in a Windows VM.

## Signing and SmartScreen

These binaries are **not Authenticode-signed** because the project has no
Windows code-signing certificate. Their embedded version information and
SHA-256 checksums provide identity and integrity, but they do not establish a
trusted publisher. Windows Defender SmartScreen may therefore warn testers on
the first launch. Do not describe the files as signed, and publish
`SHA256SUMS` alongside them.

If an Authenticode certificate is obtained later, sign each already-verified
`.exe`, verify the signatures on Windows, and regenerate `SHA256SUMS` for the
signed bytes. Never commit a private signing key or its password.

The source-code license does not grant rights to the original Populous assets
or trademarks; consult the repository README before redistributing binaries.
