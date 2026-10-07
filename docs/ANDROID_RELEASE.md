# Distributable Android APK

The release script builds a signed ARM64 APK for direct installation from a
website. It does not install anything on the Pixel. A private local application
key signs a self-signed certificate and is reused for subsequent updates.

## Prepare game resources

The repository includes sources and packaging tools. Original Amiga resources,
decoded artwork, generated launcher icons and compiled APKs remain local.
Prepare a compatible original Populous disk before building:

```sh
sh tools/exclude-local-assets.sh
./scripts/prepare-assets.sh -adf "/path/to/Populous.adf"
```

Preparation extracts and verifies the 15 resource files, exports the original
screen images and generates the launcher icons. No emulator or original Amiga
executable is required. [Preparing original game data](ASSET_SETUP.md) identifies
the supported single-disk game, provides the Planet Emu catalogue hint and
explains the individual import commands.

The Android build embeds the locally prepared data in the application. Setting
`POPULOUS_AMIGA_DIR` to an external desktop installation does not populate an
Android package; prepare `assets/amiga/` before compiling it. A source checkout
can compile without original data, but a playable APK requires that data.

## Build

Development and release builds use Go compatible with `go.mod`, Java 17,
SDK Platform 36, Build Tools 36.0.0, NDK 28.2.13676358, Gradle 8.11.1 through
the wrapper, and Ebitengine/ebitenmobile 2.9.11. The script accepts
`ANDROID_HOME` or `ANDROID_SDK_ROOT`, and `JAVA_HOME`. It passes the chosen NDK
to gomobile. `openssl`, `unzip` and `shasum` must be available.

For the first release, explicitly create the signing key:

```sh
./scripts/build-android-release.sh --init-key
```

For subsequent releases, reuse that key:

```sh
./scripts/build-android-release.sh
```

Do not run this script and `scripts/run-android.sh` simultaneously: both
regenerate `android/app/libs/populous.aar`. Release builds use
`-trimpath -ldflags '-s -w'` for Go, R8 for Java, and resource shrinking. Java
names and callbacks used by Go/JNI are preserved. Only `arm64-v8a` is included.
The build verifies local resource fingerprints and regenerates the icons.

The current version is `1.1.0`, Android version code `4`. `VERSION` defines the
shared version name. Increment the Android code in `android/app/build.gradle`
for each distributed update.

Outputs are generated in `dist/android/` and excluded from Git:

- `populous-android-1.1.0-arm64.apk`: signed application;
- `SHA256SUMS`: downloadable file fingerprint;
- `SIGNATURE.txt` and `populous-release-cert.pem`: public certificate and verification;
- `RELEASE.txt`: tool versions and build properties.

The script publishes the APK only after verification. If an older APK with the
same name differs, it preserves a local copy in a `previous.*` subdirectory.
Removing compiled releases from Git does not delete these local files. Pinned
tools do not imply that two builds produce byte-identical APKs.

## Signing key and updates

`android/keystore/` is excluded from Git and private (`0700`). It contains a
3072-bit RSA key (`populous-release.p12`) and its random password
(`populous-release.password`), each with `0600` permissions. Tools receive the
password through a file; the script does not print it. The certificate uses
`CN=Populous Android` and a 50-year validity period.

Back up this directory privately before distributing a build. The script reuses
the existing key and refuses to replace an incomplete key/password pair. Do not
distribute the private key or password. Future updates require a compatible
signature; see the [Android signing documentation](https://developer.android.com/studio/publish/app-signing).

A release APK cannot replace the installed debug application when the keys
differ, even with a higher version code. The script therefore never uninstalls
the Pixel development app or modifies its save. Any migration must preserve
application data separately. Subsequent release updates reuse the release key.

## Downloads and compatibility

Host the APK over HTTPS with its SHA-256 fingerprint. Android may ask the user
to allow application installation for the selected browser or file manager.
The local certificate does not replace Android's installation checks.
Application signing and developer identity verification are separate matters;
consult the current [Android developer verification documentation](https://developer.android.com/developer-verification)
before broader distribution.

The APK targets API 36, supports API 23 and newer, and contains ARM64 code.
The Pixel 10a is the project's test device; that does not validate every other
Pixel model or a future Android version.

The build checks the signature with
[`apksigner`](https://developer.android.com/tools/apksigner), 16 KiB ZIP alignment,
and every native ELF `LOAD`/`GNU_RELRO` segment. Native linker settings
`max-page-size=16384` and `common-page-size=16384` are passed through
`CGO_LDFLAGS`, including the trailing `GNU_RELRO` alignment. Native libraries
remain uncompressed for direct loading from the APK. Static alignment checks
are distinct from running the game on a 16 KiB device; see the
[Android page-size guide](https://developer.android.com/guide/practices/page-sizes).

A locally built APK contains the original resources imported by its builder.
Building and signing the application does not grant permission to redistribute
those resources.

## Validation on 19 September 2026

Version `1.0.0 (2)` ran on the Pixel 10a with Android 17. The introduction,
menus, beginning of a match, two-finger navigation, power panel and loading of
the existing save were checked. To preserve the development installation and
its data, this test used the existing debug signature with the same optimized
release content. All 13 application entries outside signatures, including the
manifest, DEX, native library, resources and metadata, matched the release APK
byte for byte. The tested application remained non-debuggable and R8-optimized.

The APK under `dist/android/` retained its separate private release signature.
The test Pixel used 4 KiB pages. ZIP and ELF 16 KiB alignment was checked, but
the application was not run on a device configured for 16 KiB pages.

Version `1.1.0 (3)` added local Bluetooth multiplayer. Its debug build was
installed on the Pixel 10a. Nearby-device permissions, visibility, secure RFCOMM
service registration, host waiting, service closure, client search, bonded
devices, and cancellation/resumption of the device picker were checked. Save
and preference fingerprints remained unchanged. A complete match still needs
a second physical device; see the [Bluetooth guide](ANDROID_BLUETOOTH.md).

The optimized release content was temporarily signed with the installed debug
key so it could run without uninstalling the application or losing data. Its
11 non-signature application entries matched the release APK byte for byte,
and the non-debuggable application started and rendered the new menu. The
final debug build was then restored, both data fingerprints were checked again,
and the temporary resigned copy was removed.

## Rebuild on 20 September 2026

The displayed version remained `1.1.0` and the Android version code became `4`.
The release APK included the new strategic AI and retained its existing signing
key. Signature, absence of the debug flag, ARM64 ABI, 16 KiB ZIP/ELF alignment
and SHA-256 were verified. The release filename remained unchanged.

The Pixel 10a received development variant `1.1.0 (4)` with its existing
signature, without uninstallation. Its native Go library had the same SHA-256
fingerprint as the optimized release APK. Activity startup and rendering of
the automatic demo were checked on the phone. Existing save and preference
fingerprints remained unchanged; the data was backed up before installation.
This validates startup on that Pixel, rather than a complete campaign or a new
two-device Bluetooth session.
