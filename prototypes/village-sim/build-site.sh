#!/bin/sh
# Builds the simulator site into a folder: the page, the real engine
# (batnet.wasm), the scenarios and the real app for the phone.
#
#   sh prototypes/village-sim/build-site.sh _site
#   cd _site && python3 -m http.server     # then open http://localhost:8000
#
# GitHub Pages runs the same script (.github/workflows/pages.yml).
set -eu
mkdir -p "$1"
out=$(cd "$1" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
mkdir -p "$out/scenarios" "$out/app"
cp "$here/sim.js" "$here/real.js" "$here/shim.js" "$here/sim-app.html" "$out/"
cp "$repo/internal/web/app/app.js" "$repo/internal/web/app/app.css" "$out/app/"
cp "$repo"/scenarios/*.json "$out/scenarios/"
(cd "$repo" && GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o "$out/batnet.wasm" ./cmd/batnetwasm)
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out/"
# index.html is written without a document skeleton (the artifact host adds one), so add it here.
{
  echo '<!doctype html>'
  echo '<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">'
  cat "$here/index.html"
} > "$out/index.html"
