#!/bin/sh
set -eu
[ "$(uname -s)" = Darwin ] || { echo 'This installer requires macOS' >&2; exit 1; }
task_version='0.19.0'
task_route=${GOCODE_UPDATE_MODE:-auto}
task_mirror=${GOCODE_UPDATE_MIRROR:-}
case "$(uname -m)" in arm64) task_arch=arm64; task_hash='0f559afcad7e9d87caffc13f7c76dcff3b25b1b1904e1952e4163d7707fddb40';; x86_64) task_arch=amd64; task_hash='d6b680c3792ac788a3a0624a56868c696f9751c5fd8cb373dd8dadb878815d88';; *) echo 'Unsupported macOS architecture' >&2; exit 1;; esac
task_destination="$HOME/Applications/gocode.app"
task_command="$task_destination/Contents/MacOS/gocode"
if [ -e "$task_destination" ]; then
  if [ ! -x "$task_command" ]; then echo 'Existing application is not a gocode versioned install' >&2; exit 1; fi
  "$task_command" -configure-updates -update-mode "$task_route" -update-mirror "$task_mirror"
  exec "$task_command" -update
fi
task_stage=$(mktemp -d "${TMPDIR:-/tmp}/gocode-install-XXXXXX")
# Only the new installer-owned temporary directory is removed.
trap 'rm -rf "$task_stage"' EXIT HUP INT TERM
task_url="https://github.com/neko233-com/gocode/releases/download/v$task_version/gocode-$task_version-darwin-$task_arch.tar.gz"
task_ok=0
case "$task_route" in
  direct) task_routes="$task_url";;
  mirror) case "$task_mirror" in https://*) ;; *) echo 'Manual route needs an HTTPS mirror prefix' >&2; exit 1;; esac; task_routes="${task_mirror%/}/$task_url";;
  auto) task_routes="$task_url https://ghfast.top/$task_url https://gh-proxy.com/$task_url";;
  *) echo 'Mode must be auto/direct/mirror' >&2; exit 1;;
esac
for task_url in $task_routes; do
  if curl --fail --location --proto '=https' --connect-timeout 5 --max-time 180 --max-filesize 536870912 "$task_url" -o "$task_stage/gocode.tar.gz" &&
     [ "$(shasum -a 256 "$task_stage/gocode.tar.gz" | cut -d ' ' -f 1)" = "$task_hash" ]; then task_ok=1; break; fi
done
if [ "$task_ok" != 1 ]; then echo 'No route returned the pinned package bytes' >&2; exit 1; fi
tar -xzf "$task_stage/gocode.tar.gz" -C "$task_stage"
"$task_stage/gocode.app/Contents/MacOS/gocode" -version
mkdir -p "$HOME/Applications" "$HOME/.local/bin"
if [ -e "$HOME/.local/bin/gocode" ] || [ -L "$HOME/.local/bin/gocode" ]; then echo 'Existing command path retained; choose a different location' >&2; exit 1; fi
mv "$task_stage/gocode.app" "$task_destination"
ln -s "$task_command" "$HOME/.local/bin/gocode"
"$task_command" -configure-updates -update-mode "$task_route" -update-mirror "$task_mirror"
printf 'Installed gocode %s. Add ~/.local/bin to PATH, then run gocode or open ~/Applications/gocode.app.\n' "$task_version"
