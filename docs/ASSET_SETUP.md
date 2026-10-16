# Preparing the original game data

The original Populous disk and locally generated game resources are not
distributed in this repository. Supply a compatible ADF before playing or
packaging the desktop and Android versions.

## Supported disk

The [Planet Emulation Amiga ADF catalogue, letter P](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=P)
lists **Populous v2.7 (1989-03-17), Electronic Arts**. Select the main game,
rather than The Promised Lands, The Final Frontier, the world editor or
Populous II.

The validated AmigaDOS OFS image has volume name `Populous` and SHA-256:

```text
55f74cacf20baca2ff23d7543617d90739c3fb277d670ef9725424b7843a07da
```

## Import and run

```sh
sh tools/exclude-local-assets.sh
./scripts/prepare-assets.sh -adf "/path/to/Populous.adf"
go run ./cmd/populous
```

The preparation script imports and verifies the 15 required files, exports the
illustrated screens and generates launcher icons. Preview an import without
writing files with:

```sh
go run ./cmd/import-assets -adf "/path/to/Populous.adf" -dry-run
```

The importer checks every required file fingerprint. A different whole-disk
hash can be accepted only when all required resources are identical.

Original and generated resources can remain outside the checkout by using
`POPULOUS_AMIGA_DIR` and `POPULOUS_EXTRACTED_IMAGE_DIR`. Android and standalone
desktop builds must embed a prepared local asset set. The exclusion helper
writes local rules to `.git/info/exclude`; no project `.gitignore` is used.
