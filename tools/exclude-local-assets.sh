#!/bin/sh
# Keep original resources and generated build products outside version control.
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
exclude=$(git rev-parse --git-path info/exclude)
for pattern in '/assets/amiga/*' '!/assets/amiga/LOCAL_RESOURCES.txt' '/assets/extracted-images/*' '!/assets/extracted-images/GENERATED.txt' '/.local/' '/dist/' '/previous/' '/new/' '/captures/' '/recordings/' '/android/app/libs/' '/android/app/build/' '/android/.gradle/' '/android/build/' '/android/local.properties' '/android/keystore/' '/.gitignore' '*.adf' '*.p12' '*.jks' '*.keystore' '*.syso'; do
    if ! grep -Fqx "$pattern" "$exclude"; then
        printf '%s\n' "$pattern" >> "$exclude"
    fi
done
for directory in drawable-nodpi mipmap-mdpi mipmap-hdpi mipmap-xhdpi mipmap-xxhdpi mipmap-xxxhdpi; do
    pattern="/android/app/src/main/res/$directory/*.png"
    if ! grep -Fqx "$pattern" "$exclude"; then
        printf '%s\n' "$pattern" >> "$exclude"
    fi
done
printf '%s\n' 'Original resources, generated artwork and local builds are excluded from Git.'
