#!/bin/bash
# From a laptop: build batnetd for the Pi, copy it and this folder over, run setup.sh.
#   deploy/pi/deploy.sh <ssh-host>            e.g. deploy/pi/deploy.sh my-pi
#   deploy/pi/deploy.sh <ssh-host> --binary   only replace batnetd and restart the hub
set -euo pipefail
host=${1:?usage: deploy.sh <ssh-host> [--binary]}
root=$(cd "$(dirname "$0")/../.." && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
(cd "$root" && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o "$out/batnetd" ./cmd/batnetd)
if [ "${2:-}" = "--binary" ]; then
  scp -q "$out/batnetd" "$host:/tmp/batnetd"
  ssh "$host" 'sudo install -m755 /tmp/batnetd /usr/local/bin/batnetd && rm /tmp/batnetd && sudo systemctl restart batnet-hub && systemctl is-active batnet-hub'
  exit
fi
cp -R "$root/deploy/pi/." "$out/"
ssh "$host" 'rm -rf ~/batnet-deploy && mkdir -p ~/batnet-deploy'
scp -qr "$out/." "$host:batnet-deploy/"
ssh "$host" 'sudo ~/batnet-deploy/setup.sh'
