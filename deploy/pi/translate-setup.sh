#!/bin/bash
# Installs batnet's offline translator on a Pi: a Python venv with
# CTranslate2 and SentencePiece in /opt/batnet-translate, Argos Translate's
# model packages for German-English and English-German (plus any pairs given
# as arguments) in /var/lib/batnet-translate/models/<pair>/, and the helper
# batnetd runs, /usr/local/lib/batnet/batnet-translate.
# Safe to run again: what is already there is not downloaded again.
# Run as root on the Pi: sudo ./translate-setup.sh [uk-en en-uk ...]
# Needs the internet once (PyPI and argos-net.com); about 0.5 GB on disk.
set -euo pipefail
cd "$(dirname "$0")"
say() { printf '\n== %s\n' "$*"; }

VENV=/opt/batnet-translate
DATA=/var/lib/batnet-translate
MODELS=$DATA/models
HELPER=/usr/local/lib/batnet/batnet-translate
INDEX_URL=https://raw.githubusercontent.com/argosopentech/argospm-index/main/index.json
PACKAGES_URL=https://argos-net.com/v1
# Versions checked on a Pi 4 (Debian 13, Python 3.13, arm64) on 2026-10-21.
PIP_PINS="ctranslate2==4.8.3 sentencepiece==0.2.2 numpy==2.5.4 pyyaml==6.0.3"
CT2_VERSION=4.8.3

pairs=(de-en en-de "$@")
for p in "${pairs[@]}"; do
  [[ $p =~ ^[a-z]{2,3}-[a-z]{2,3}$ ]] || { echo "not a language pair: $p (like de-en)" >&2; exit 2; }
done

say "Python venv with CTranslate2 and SentencePiece ($VENV)"
if ! "$VENV/bin/python3" -I -c "import ctranslate2, sentencepiece, sys; sys.exit(ctranslate2.__version__ != '$CT2_VERSION')" 2>/dev/null; then
  python3 -m venv --help >/dev/null 2>&1 || apt-get -y -q install python3-venv >/dev/null
  python3 -m venv "$VENV"
  "$VENV/bin/pip" install -q --no-cache-dir --disable-pip-version-check --only-binary=:all: $PIP_PINS
fi

say "Translation models ($MODELS)"
install -d -m755 "$DATA" "$MODELS"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
index=""
# Prints the index's download link for a pair, or nothing.
link_for() {
  if [ -z "$index" ]; then
    curl -fsSL "$INDEX_URL" -o "$tmp/index.json"
    index=$tmp/index.json
  fi
  "$VENV/bin/python3" -I - "$index" "${1%-*}" "${1#*-}" <<'EOF'
import json, sys
for p in json.load(open(sys.argv[1])):
    if p.get("from_code") == sys.argv[2] and p.get("to_code") == sys.argv[3]:
        print(next((l for l in p.get("links", []) if l.startswith("https://")), ""))
        break
EOF
}
# Unpacks the model, sentencepiece.model and metadata from an Argos package
# into a pair folder; fails (status 3) if the package's tokenizer is not
# SentencePiece (newer OPUS-MT based packages use BPE with Moses).
unpack() {
  "$VENV/bin/python3" -I - "$1" "$2" <<'EOF'
import os, shutil, sys, zipfile
src, dst = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(src) as z:
    names = z.namelist()
    root = names[0].split("/")[0] + "/"
    want = {}
    for n in names:
        rel = n[len(root):] if n.startswith(root) else None
        if rel in ("sentencepiece.model", "metadata.json", "README.md") or (
                rel and rel.startswith("model/") and "/" not in rel[6:] and rel[6:]):
            want[n] = rel
    if "sentencepiece.model" not in want.values():
        sys.exit(3)
    os.makedirs(os.path.join(dst, "model"), exist_ok=True)
    for n, rel in want.items():
        with z.open(n) as f, open(os.path.join(dst, rel), "wb") as out:
            shutil.copyfileobj(f, out)
EOF
}
for p in "${pairs[@]}"; do
  dir=$MODELS/$p
  if [ -f "$dir/sentencepiece.model" ] && [ -f "$dir/model/model.bin" ]; then
    echo "$p: already there"
    continue
  fi
  url=$(link_for "$p")
  [ -n "$url" ] || { echo "$p: Argos has no model for this pair" >&2; continue; }
  # The index's package first; if it is not SentencePiece, the other versions
  # argos-net.com keeps (uk-en 1.9 is BPE, uk-en 1.4 is SentencePiece).
  file=${url##*/}
  name=${file%-*}          # translate-uk_en
  latest=${file%.argosmodel}; latest=${latest##*-}   # 1_9
  candidates=("$url")
  for v in 1_9 1_8 1_7 1_6 1_5 1_4 1_3 1_2 1_1 1_0; do
    [ "$v" = "$latest" ] || candidates+=("$PACKAGES_URL/$name-$v.argosmodel")
  done
  rm -rf "$dir.new"
  done_=""
  for u in "${candidates[@]}"; do
    curl -fsSL "$u" -o "$tmp/pkg" 2>/dev/null || continue
    if unpack "$tmp/pkg" "$dir.new"; then
      rm -rf "$dir"; mv "$dir.new" "$dir"
      echo "$p: ${u##*/}"
      done_=1
      break
    fi
    rm -rf "$dir.new"
  done
  rm -f "$tmp/pkg"
  [ -n "$done_" ] || echo "$p: no Argos package with a SentencePiece tokenizer found" >&2
done
# Readable by everyone, the batnet user included; written only by root.
chown -R root:root "$DATA"
chmod -R u=rwX,go=rX "$DATA"

say "The helper ($HELPER)"
install -d /usr/local/lib/batnet
sed "1s|.*|#!$VENV/bin/python3 -I|" batnet-translate >"$tmp/helper"
install -m755 "$tmp/helper" "$HELPER"
echo '{"id":0,"op":"pairs"}' | "$HELPER" --models "$MODELS" 2>/dev/null
