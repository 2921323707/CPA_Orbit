#!/usr/bin/env bash
set -euo pipefail

WAILS_VERSION="v2.13.0"
APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$APP_DIR/.." && pwd)"

for command_name in go node npm ditto hdiutil lipo shasum; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$command_name" >&2
    exit 1
  fi
done
if [[ "$(uname -s)" != "Darwin" ]]; then
  printf 'The macOS app must be built on macOS.\n' >&2
  exit 1
fi

if [[ -n "${CPA_ORBIT_MAC_ARCH:-}" ]]; then
  arch="$CPA_ORBIT_MAC_ARCH"
else
  case "$(uname -m)" in
    arm64) arch="arm64" ;;
    x86_64) arch="amd64" ;;
    *) printf 'Unsupported macOS architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
  esac
fi
case "$arch" in
  arm64|amd64|universal) ;;
  *) printf 'CPA_ORBIT_MAC_ARCH must be arm64, amd64, or universal.\n' >&2; exit 1 ;;
esac

cd "$APP_DIR"
go run "github.com/wailsapp/wails/v2/cmd/wails@$WAILS_VERSION" build \
  -clean -trimpath -skipbindings -platform "darwin/$arch" -ldflags "-s -w"

output_dir="$APP_DIR/build/bin"
app_path="$output_dir/CPA Orbit.app"
if [[ ! -d "$app_path" ]]; then
  printf 'Expected app bundle was not produced: %s\n' "$app_path" >&2
  exit 1
fi
if [[ -n "${CPA_ORBIT_CODESIGN_IDENTITY:-}" ]]; then
  codesign --force --deep --options runtime --timestamp \
    --sign "$CPA_ORBIT_CODESIGN_IDENTITY" "$app_path"
fi

executable_path="$app_path/Contents/MacOS/CPAOrbit"
if [[ ! -x "$executable_path" ]]; then
  printf 'Expected app executable was not produced: %s\n' "$executable_path" >&2
  exit 1
fi
executable_archs="$(lipo -archs "$executable_path")"
case "$arch" in
  arm64)
    [[ "$executable_archs" == "arm64" ]] || {
      printf 'Expected an arm64 executable, found: %s\n' "$executable_archs" >&2
      exit 1
    }
    ;;
  amd64)
    [[ "$executable_archs" == "x86_64" ]] || {
      printf 'Expected an x86_64 executable, found: %s\n' "$executable_archs" >&2
      exit 1
    }
    ;;
  universal)
    [[ " $executable_archs " == *" arm64 "* && " $executable_archs " == *" x86_64 "* ]] || {
      printf 'Expected a universal executable, found: %s\n' "$executable_archs" >&2
      exit 1
    }
    ;;
esac

cp "$APP_DIR/cpa-orbit.config.example.json" "$output_dir/"
cp "$REPO_ROOT/LICENSE" "$output_dir/LICENSE.txt"
cp "$REPO_ROOT/docs/THIRD_PARTY_NOTICES.md" "$output_dir/"
archive="$output_dir/CPAOrbit-macos-$arch.zip"
dmg="$output_dir/CPAOrbit-macos-$arch.dmg"
rm -f "$archive" "$dmg"
ditto -c -k --sequesterRsrc --keepParent "$app_path" "$archive"

dmg_staging_dir="$(mktemp -d "${TMPDIR:-/tmp}/cpa-orbit-dmg.XXXXXX")"
trap 'rm -rf "$dmg_staging_dir"' EXIT
ditto "$app_path" "$dmg_staging_dir/CPA Orbit.app"
ln -s /Applications "$dmg_staging_dir/Applications"
hdiutil create -volname "CPA Orbit" -srcfolder "$dmg_staging_dir" \
  -ov -format UDZO "$dmg"
hdiutil verify "$dmg"

(
  cd "$output_dir"
  shasum -a 256 "$(basename "$archive")" "$(basename "$dmg")" > CHECKSUMS-SHA256.txt
)
printf 'Build complete (%s): %s\n' "$executable_archs" "$output_dir"
