#!/bin/sh
# Vendors the DeckCraft and PdfCraft web builds pinned in crafts.lock.
#   scripts/crafts.sh fetch [dest]  download, verify sha256, unpack into <dest>/<app>/ (default static/vendor)
#   scripts/crafts.sh bump          point crafts.lock at each app's latest upstream release
set -eu
cd "$(dirname "$0")/.."
LOCK=crafts.lock

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }
apps() { grep -v '^#' "$LOCK" | grep -v '^[[:space:]]*$'; }

fetch() {
  dest=${1:-static/vendor}
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  apps | while read -r app ver sum; do
    zip="$tmp/$app.zip"
    curl -fsSL -o "$zip" "https://github.com/storytold/$app/releases/download/v$ver/$app-web-$ver.zip"
    got=$(sha256 "$zip")
    if [ "$got" != "$sum" ]; then echo "$app $ver: sha256 $got does not match crafts.lock" >&2; exit 1; fi
    rm -rf "${tmp:?}/x" "$dest/$app" && mkdir -p "$tmp/x" "$dest"
    unzip -q "$zip" -d "$tmp/x"
    mv "$tmp/x/$app-web-$ver" "$dest/$app"
    echo "$ver" > "$dest/$app/VERSION"
    # Precompressed once here so the server sends the .gz instead of gzipping 30-60 MB per request.
    find "$dest/$app" -type f \( -name '*.wasm' -o -name '*.js' \) -exec gzip -9 -k -f {} \;
    echo "vendored $app $ver -> $dest/$app"
  done
}

bump() {
  out=$(mktemp)
  grep '^#' "$LOCK" > "$out"
  apps | while read -r app _ _; do
    # The releases/latest page redirects to .../tag/v<version>; no API call, so no API rate limit.
    tag=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/storytold/$app/releases/latest" | sed -n 's#.*/tag/v\{0,1\}##p')
    [ -n "$tag" ] || { echo "$app: no latest release found" >&2; exit 1; }
    sum=$(curl -fsSL "https://github.com/storytold/$app/releases/download/v$tag/SHA256SUMS.txt" | awk -v f="$app-web-$tag.zip" '$2 == f || $2 == "*"f {print $1}')
    [ -n "$sum" ] || { echo "$app $tag: no checksum for the web zip" >&2; exit 1; }
    printf '%-12s %-8s %s\n' "$app" "$tag" "$sum" >> "$out"
  done
  cat "$out" > "$LOCK" && rm -f "$out"  # keeps the lock's file mode
  cat "$LOCK"
}

case "${1:-fetch}" in
  fetch) shift 2>/dev/null || true; fetch "$@" ;;
  bump) bump ;;
  *) echo "usage: $0 fetch [dest] | bump" >&2; exit 2 ;;
esac
