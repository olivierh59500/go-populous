#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
android_sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}
java_home_path=${JAVA_HOME:-}
ebiten_version=v2.9.11
android_api=23
compile_sdk=36
build_tools_version=36.0.0
ndk_version=28.2.13676358
application_id=com.olivierh.populous

if [ -z "$android_sdk" ] && [ -d /opt/homebrew/share/android-commandlinetools ]; then
    android_sdk=/opt/homebrew/share/android-commandlinetools
fi
if [ -z "$android_sdk" ] && [ -d "$HOME/Library/Android/sdk" ]; then
    android_sdk=$HOME/Library/Android/sdk
fi
if [ -z "$java_home_path" ] && [ -d /Applications/Android\ Studio.app/Contents/jbr/Contents/Home ]; then
    java_home_path=/Applications/Android\ Studio.app/Contents/jbr/Contents/Home
fi
if [ -z "$java_home_path" ] && [ -d /opt/homebrew/opt/openjdk@17 ]; then
    java_home_path=/opt/homebrew/opt/openjdk@17
fi

if [ -z "$android_sdk" ] || [ ! -x "$android_sdk/platform-tools/adb" ]; then
    echo "SDK Android introuvable. Définissez ANDROID_HOME ou ANDROID_SDK_ROOT." >&2
    exit 1
fi
if [ ! -f "$android_sdk/platforms/android-$compile_sdk/android.jar" ]; then
    echo "SDK Platform android-$compile_sdk introuvable dans $android_sdk." >&2
    exit 1
fi
if [ ! -x "$android_sdk/build-tools/$build_tools_version/aapt" ] ||
    [ ! -x "$android_sdk/build-tools/$build_tools_version/apksigner" ] ||
    [ ! -x "$android_sdk/build-tools/$build_tools_version/zipalign" ]; then
    echo "Android Build Tools $build_tools_version introuvables dans $android_sdk." >&2
    exit 1
fi
if [ ! -d "$android_sdk/ndk/$ndk_version" ]; then
    echo "Android NDK $ndk_version introuvable dans $android_sdk." >&2
    exit 1
fi
if [ -z "$java_home_path" ] || [ ! -x "$java_home_path/bin/java" ]; then
    echo "Java 17 introuvable. Définissez JAVA_HOME." >&2
    exit 1
fi
if ! "$java_home_path/bin/java" -version 2>&1 | grep -q 'version "17\.'; then
    echo "Java 17 est requis (JAVA_HOME=$java_home_path)." >&2
    "$java_home_path/bin/java" -version >&2
    exit 1
fi
if ! command -v go >/dev/null 2>&1; then
    echo "Go est introuvable dans PATH." >&2
    exit 1
fi
if [ ! -x "$project_root/android/gradlew" ]; then
    echo "Wrapper Gradle absent dans android/." >&2
    exit 1
fi

export ANDROID_HOME="$android_sdk"
export ANDROID_SDK_ROOT="$android_sdk"
export JAVA_HOME="$java_home_path"
export PATH="$java_home_path/bin:$android_sdk/platform-tools:$PATH"

cd "$project_root"
module_ebiten_version=$(go list -m -f '{{.Version}}' github.com/hajimehoshi/ebiten/v2)
if [ "$module_ebiten_version" != "$ebiten_version" ]; then
    echo "Version Ebitengine incohérente : go.mod=$module_ebiten_version, ebitenmobile=$ebiten_version." >&2
    exit 1
fi

mkdir -p "$project_root/android/app/libs"

echo "→ Génération de la bibliothèque Go/Ebitengine (arm64-v8a)"
go run "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@$ebiten_version" \
    bind \
    -target android/arm64 \
    -androidapi "$android_api" \
    -javapkg "$application_id" \
    -o android/app/libs/populous.aar \
    ./mobile

echo "→ Compilation de l’APK de débogage"
"$project_root/android/gradlew" -p "$project_root/android" --console=plain assembleDebug

adb_path="$android_sdk/platform-tools/adb"
apk_path="$project_root/android/app/build/outputs/apk/debug/app-debug.apk"
aapt_path="$android_sdk/build-tools/$build_tools_version/aapt"
apksigner_path="$android_sdk/build-tools/$build_tools_version/apksigner"
zipalign_path="$android_sdk/build-tools/$build_tools_version/zipalign"

echo "→ Vérification de l’APK"
if [ ! -f "$apk_path" ]; then
    echo "APK absent après le build : $apk_path" >&2
    exit 1
fi
apk_badging=$("$aapt_path" dump badging "$apk_path")
printf '%s\n' "$apk_badging" | grep -q "package: name='$application_id'"
printf '%s\n' "$apk_badging" | grep -q "sdkVersion:'$android_api'"
printf '%s\n' "$apk_badging" | grep -q "targetSdkVersion:'$compile_sdk'"
printf '%s\n' "$apk_badging" | grep -q "launchable-activity: name='$application_id.MainActivity'"
printf '%s\n' "$apk_badging" | grep -q "native-code: 'arm64-v8a'"
"$apksigner_path" verify "$apk_path"
"$zipalign_path" -c -P 16 4 "$apk_path"

device_count=$("$adb_path" devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')
if [ "$device_count" -ne 1 ]; then
    echo "Un seul appareil Android autorisé est requis (détectés : $device_count)." >&2
    echo "Déverrouillez le Pixel, activez le débogage USB et acceptez son empreinte RSA." >&2
    "$adb_path" devices -l >&2
    exit 1
fi

echo "→ Installation sur le Pixel"
"$adb_path" install -r "$apk_path"

echo "→ Lancement de Populous"
"$adb_path" shell am force-stop "$application_id"
"$adb_path" shell am start -n "$application_id/.MainActivity"
