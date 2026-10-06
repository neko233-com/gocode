#!/bin/sh
set -eu
task_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$task_root"
task_version=${1:-$(cat VERSION)}
task_output=${2:-"$task_root/dist"}
case "$task_version" in *[!0-9.]*|'') echo 'Invalid package version' >&2; exit 1;; esac
task_commit=$(git rev-parse HEAD)
if [ -n "$(git status --porcelain)" ]; then
  if [ "${GOCODE_ALLOW_DIRTY:-0}" != 1 ]; then echo 'Release packaging requires committed source' >&2; exit 1; fi
  task_commit="$task_commit-dirty"
fi
task_arch=$(go env GOARCH)
case "$task_arch" in amd64|arm64) ;; *) echo 'Unsupported macOS architecture' >&2; exit 1;; esac
mkdir -p "$task_root/.cache" "$task_output"
task_output=$(CDPATH='' cd -- "$task_output" && pwd)
task_stage=$(mktemp -d "$task_root/.cache/macos-package-XXXXXX")
task_payload="$task_stage/payload"
mkdir -p "$task_payload/versions/$task_version" "$task_output"
GOWORK=off CGO_ENABLED=1 GOEXPERIMENT=cgocheck2 go build -trimpath -ldflags="-s -w -X main.version=$task_version -X main.sourceCommit=$task_commit" -o "$task_payload/versions/$task_version/gocode-app" .
GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$task_payload/gocode" ./cmd/gocode-launcher
cp "$task_payload/gocode" "$task_payload/gocode-launch"
printf '{"owner":"neko233-com/gocode","schema":1,"version":"%s","source":"%s"}\n' "$task_version" "$task_commit" > "$task_payload/base.json"
cp LICENSE "$task_payload/LICENSE.txt"
cp README.md "$task_payload/README.md"
cp assets/code-oss/LICENSE.txt "$task_payload/CODE-OSS-LICENSE.txt"
cp assets/fsnotify/LICENSE.txt "$task_payload/FSNOTIFY-LICENSE.txt"
task_app="$task_stage/gocode.app"
mkdir -p "$task_app/Contents/MacOS" "$task_app/Contents/Resources"
cp -R "$task_payload/." "$task_app/Contents/MacOS/"
cp assets/code-oss/code.icns "$task_app/Contents/Resources/code.icns"
cat > "$task_app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>gocode-launch</string>
<key>CFBundleIdentifier</key><string>com.neko233.gocode</string>
<key>CFBundleName</key><string>gocode</string>
<key>CFBundleDisplayName</key><string>gocode</string>
<key>CFBundleShortVersionString</key><string>$task_version</string>
<key>CFBundleVersion</key><string>$task_version</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIconFile</key><string>code</string>
<key>LSMinimumSystemVersion</key><string>13.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
EOF
task_zip="$task_output/gocode-$task_version-darwin-$task_arch.zip"
task_tar="$task_output/gocode-$task_version-darwin-$task_arch.tar.gz"
if [ -e "$task_zip" ] || [ -e "$task_tar" ]; then echo 'Package output already exists' >&2; exit 1; fi
(cd "$task_payload" && zip -q -r "$task_zip" .)
task_probe=true
case "$task_commit" in *-dirty) task_probe=false;; esac
GOWORK=off CGO_ENABLED=0 go run ./cmd/gocode-packagecheck -archive "$task_zip" -platform "darwin/$task_arch" -version "$task_version" -source "${task_commit%-dirty}" -probe="$task_probe"
tar -C "$task_stage" -czf "$task_tar" gocode.app
"$task_app/Contents/MacOS/gocode" -version
plutil -lint "$task_app/Contents/Info.plist"
printf 'App bundle: %s\n' "$task_app"
shasum -a 256 "$task_zip" "$task_tar"
