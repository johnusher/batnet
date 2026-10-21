#!/bin/sh
# Screenshots of a real hub's panel for the README, with ?redact: real radio
# names, node IDs, places, words and the home WiFi are swapped for
# placeholders in the page before it draws (panel.js, "?redact"). Look at
# every picture before committing it all the same.
#
#   sh docs/img/hub/shoot.sh <hub-address>     e.g. sh docs/img/hub/shoot.sh my-hub.local
set -eu
host=${1:?usage: shoot.sh <hub-address>}
here=$(cd "$(dirname "$0")" && pwd)
chrome=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
[ -x "$chrome" ] || chrome=$(command -v chromium || command -v google-chrome || true)
[ -n "$chrome" ] || { echo "shoot.sh: no Chrome or Chromium found (set CHROME)" >&2; exit 1; }
shot() { # name, view, width, height, extra query, colour scheme (1 light, 0 dark)
  "$chrome" --headless=new --hide-scrollbars --blink-settings=preferredColorScheme=$6 \
    --window-size=$3,$4 --virtual-time-budget=10000 --screenshot="$here/$1.png" \
    "http://$host/hub/?redact=1$5#$2" 2>/dev/null
  if command -v pngquant >/dev/null; then pngquant --force --skip-if-larger --quality 70-95 --output "$here/$1.png" "$here/$1.png" || true; fi
}
shot home home 1280 760 "" 1
shot radios radios 1280 760 "" 1
shot map map 1280 760 "" 1
# The map's street tiles are photos at heart: much smaller as a JPEG.
sips -s format jpeg -s formatOptions 82 "$here/map.png" --out "$here/map.jpg" >/dev/null && rm "$here/map.png"
shot traffic traffic 1280 900 "" 1
shot system system 1280 900 "" 1
shot screen home 800 480 "&screen=1" 0   # the hub's own 800 x 480 screen
ls -l "$here"/*.png "$here"/*.jpg
