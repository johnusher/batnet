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

# Can this laptop open the hub by its name, quickly? A slow or failed name
# lookup looks like a dead hub (B84), so say so now rather than later.
name=$(ssh "$host" hostname).local
read -r code dns <<<"$(curl -s -o /dev/null -m 15 -w '%{http_code} %{time_namelookup}' "http://$name/hub/" || true)"
if [ "${code:-000}" != 200 ]; then
  echo "WARNING: http://$name/hub/ did not open from this laptop (HTTP ${code:-000}). Same network? Try the hub's address in numbers."
elif awk -v d="$dns" 'BEGIN { exit !(d > 1) }'; then
  echo "WARNING: finding $name took ${dns} s; browsers may give up. Does the hub's network have IPv6 (setup.sh turns local IPv6 on)?"
else
  echo "http://$name/hub/ opens from this laptop (name found in ${dns} s)"
fi
