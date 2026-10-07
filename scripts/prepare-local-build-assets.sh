#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

if ! command -v go >/dev/null 2>&1; then
    printf '%s\n' 'Go is required to validate and rebuild local game resources.' >&2
    exit 1
fi

# Validate before writing images or launcher artwork, and before any signing
# material is created by a release script. Original files are never downloaded.
if ! GOWORK=off go run ./cmd/import-assets -verify; then
    printf '%s\n' \
        'Import your original Populous ADF with scripts/prepare-assets.sh first.' \
        'See docs/ASSET_SETUP.md for resource setup and supported disk images.' >&2
    exit 1
fi

GOWORK=off go run ./cmd/export-images -amiga assets/amiga -out assets/extracted-images
GOWORK=off go run ./cmd/android-icon
printf '%s\n' 'Local screen images and launcher artwork are ready.'
