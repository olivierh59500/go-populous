# Preparing original game data

The repository contains Go sources, extraction tools, build scripts and
documentation. Original Amiga resource files, decoded artwork, launcher icons
and compiled releases remain local. Supply a compatible Populous Amiga disk
image before running or packaging the game.

The application uses an independent Go engine. The importer reads the disk's
AmigaDOS filesystem and extracts only the game's resource files; it does not
import or execute an Amiga program. The import and image export tools require
Go, with no Amiga emulator or system extraction utility.

## Identify the disk

The [Planet Emu Amiga ADF catalogue, letter P](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=P)
is a useful reference for identifying **Populous v2.7 (1989-03-17), Electronic
Arts**. The original game is a single-disk title in this catalogue. Select the
main game, rather than **The Promised Lands**, **The Final Frontier**, the world
editor or **Populous II**.

Extract the `.adf` from its archive before importing it. The catalogue includes
many modified editions: a matching title alone does not guarantee compatible
resources. The importer checks every required file against the fingerprints in
[the resource manifest](../internal/assetimport/manifest.go). A different
whole-disk fingerprint can still contain identical supported resources.

The disk used for validation is an AmigaDOS OFS image with the volume name
`Populous` and this SHA-256 fingerprint:

```text
55f74cacf20baca2ff23d7543617d90739c3fb277d670ef9725424b7843a07da
```

No disk image or game data is downloaded by the tools.

## Import and run

From the project directory:

```sh
sh tools/exclude-local-assets.sh
./scripts/prepare-assets.sh -adf "/path/to/Populous.adf"
go run ./cmd/populous
```

The preparation script runs the checked importer, verifies its output,
exports the screen PNGs and generates the Android/Windows launcher icons.
The equivalent individual commands are:

```sh
go run ./cmd/import-assets -adf "/path/to/Populous.adf"
go run ./cmd/import-assets -verify
go run ./cmd/export-images -amiga assets/amiga -out assets/extracted-images
go run ./cmd/android-icon
```

The importer extracts 15 required files into `assets/amiga/`. These provide the
four terrain sets and their rules, the 495 campaign records, sprites, font,
screens, music, sound effects and speech samples. Unrelated operating-system
files and the original executable are left on the disk. Existing identical
files are reused; a differing destination file is rejected rather than
overwritten. All required files are checked before a new import is written.

The image exporter recreates `demo.pic.png`, `qaz.pic.png`, `lord.pic.png` and
`load.pic.png` in `assets/extracted-images/`. It decodes the native bitplanes and
the palettes stored with the illustrated screens. No pre-extracted PNG is
required.

Preview an import without writing files with:

```sh
go run ./cmd/import-assets -adf "/path/to/Populous.adf" -dry-run
```

For disks whose required resources are split across images, repeat `-adf`.
The same fingerprint checks apply to the combined resources.

## External installation

The prepared data can also remain outside the checkout:

```sh
go run ./cmd/import-assets \
  -adf "/path/to/Populous.adf" \
  -output "/path/to/private/populous-data"
go run ./cmd/import-assets -verify -output "/path/to/private/populous-data"
go run ./cmd/export-images \
  -amiga "/path/to/private/populous-data" \
  -out "/path/to/private/populous-images"
POPULOUS_AMIGA_DIR="/path/to/private/populous-data" \
POPULOUS_EXTRACTED_IMAGE_DIR="/path/to/private/populous-images" \
  go run ./cmd/populous
```

Desktop asset discovery uses these environment variables, then local project
directories, then data embedded when the executable was built. A clean clone
can compile without original resources, but cannot play until compatible data
has been prepared. Missing resources produce an error with the setup steps.

## Android and Windows builds

For standalone desktop executables and Android packages, import into the
default project directories before building. Go embeds the locally prepared
resource files and extracted screen images, so the resulting application does
not need an ADF or an asset directory on the target device. External desktop
environment variables do not populate an Android package.

Recreate the launcher artwork from the imported title screen with:

```sh
go run ./cmd/android-icon
```

The Windows resource configuration uses the generated Android icon too. The
Android shell, Windows resource configuration and all packaging scripts stay
in Git; generated icons, AARs, APKs and executables stay local. Use the
[Android release script](../scripts/build-android-release.sh) or follow the
[Windows release guide](WINDOWS_RELEASE.md) for packaging and validation.

The exclusion helper adds local Git rules without distributing a `.gitignore`.
It preserves locally imported data and generated artwork when repository
changes are committed. Run it after cloning and before importing resources.
