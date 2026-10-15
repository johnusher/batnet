#!/bin/sh
# Screenshot the village simulator in headless Chrome after running some JavaScript in the page.
#
#   sh prototypes/village-sim/shoot.sh OUT.png 'JS' [light|dark] [phone|full]
#
# JS runs inside the page's own scope (through window.__simEval), so it can use
# sim, S, app, setPersona, renderPhone and the rest. Examples:
#   sh shoot.sh scan.png "setPersona(S.houses[0].admin); app.tab='scan'; renderPhone(true);"
#   sh shoot.sh map.png "sim.step(S.now + 3*3600000);" light full
#
# A script element appended to the page cannot reach those names: the page code
# lives inside an IIFE, so use this hook. Errors in JS end up in the output's
# title and are printed. "phone" crops to the phone column (1440x1000 layout).
set -eu
out=$1; js=$2; scheme=${3:-light}; crop=${4:-phone}
here=$(cd "$(dirname "$0")" && pwd)
chrome=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
[ -x "$chrome" ] || chrome=$(command -v chromium || command -v google-chrome || true)
[ -n "$chrome" ] || { echo "shoot.sh: no Chrome or Chromium found (set CHROME)" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jsjson=$(printf '%s' "$js" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')
cat > "$tmp/h.html" <<EOF
<!doctype html><meta charset="utf-8">
<iframe id=f src="file://$here/index.html" style="width:1440px;height:1000px;border:0;position:absolute;left:0;top:0"></iframe>
<script>
f.onload = () => setTimeout(() => {
  try { f.contentWindow.__simEval($jsjson); } catch (e) { document.title = 'ERR ' + e.stack; }
}, 400);
</script>
EOF
scheme_n=1; [ "$scheme" = dark ] && scheme_n=0
"$chrome" --headless=new --allow-file-access-from-files --virtual-time-budget=6000 --dump-dom "file://$tmp/h.html" 2>/dev/null | grep -o '<title>ERR[^<]*' && exit 1 || true
"$chrome" --headless=new --hide-scrollbars --allow-file-access-from-files --blink-settings=preferredColorScheme=$scheme_n \
  --screenshot="$tmp/full.png" --window-size=1440,1000 --virtual-time-budget=6000 "file://$tmp/h.html" 2>/dev/null
if [ "$crop" = phone ]; then
  sips -c 790 400 --cropOffset 210 1035 "$tmp/full.png" --out "$out" >/dev/null 2>&1 || cp "$tmp/full.png" "$out"
else
  cp "$tmp/full.png" "$out"
fi
echo "$out"
