#!/bin/sh
# Recreate local build resources from a user-supplied original Populous ADF.
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
if [ "$#" -eq 0 ]; then
    printf '%s\n' 'Usage: scripts/prepare-assets.sh -adf /path/to/Populous.adf' >&2
    exit 1
fi
sh tools/exclude-local-assets.sh
go run ./cmd/import-assets "$@"
./scripts/prepare-local-build-assets.sh
