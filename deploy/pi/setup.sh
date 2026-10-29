#!/bin/bash
# Turns a Raspberry Pi OS (trixie, arm64) Pi into a batnet hub: cleans out
# services batnet does not use, installs meshtasticd for a USB Meshstick,
# batman-adv tools, the kiosk for the HDMI screen, and the batnet services.
# Safe to run again. Run as root on the Pi: sudo ./setup.sh
# (deploy.sh copies this folder and the batnetd binary over and runs it.)
set -euo pipefail
cd "$(dirname "$0")"
export DEBIAN_FRONTEND=noninteractive
APT="apt-get -y -q -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold"
say() { printf '\n== %s\n' "$*"; }
# Memory decides what a Pi gets: the hub takes about 200 MB, the kiosk's
# Chromium about 350 MB and the translator about 400 MB (bench, 2026-10-21).
# A Pi under 1 GB, like a Zero 2 W (416 MB usable), gets neither; a 1 GB Pi,
# like a Pi 3 (which reports about 920 MB), gets the kiosk but no translator.
# BATNET_SMALL=1 leaves both out, BATNET_SMALL=0 puts both in.
mem=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
noscreen=${BATNET_SMALL:-$((mem < 800000))}
notrans=${BATNET_SMALL:-$((mem < 1500000))}

say "Clean-up: services a batnet hub does not use"
# Printing, modems (ModemManager also grabs USB serial devices), NFS, VNC and
# remote access, Bluetooth, auto-mounting, PackageKit, first-boot cloud-init.
$APT purge cups cups-browsed cups-daemon modemmanager wayvnc rpi-connect packagekit >/dev/null || true
for u in rpcbind.service rpcbind.socket nfs-blkmap.service bluetooth.service udisks2.service \
         dnsmasq.service; do
  systemctl disable --now "$u" 2>/dev/null || true
done
touch /etc/cloud/cloud-init.disabled
$APT autoremove --purge >/dev/null
dpkg -l | awk '/^rc/{print $2}' | xargs -r dpkg --purge >/dev/null
# Files from the Pi's earlier use stay in place: nothing of theirs starts on boot.
# A desktop image adds the 32-bit (armhf) package lists, which no hub uses:
# 55 MB on the card and a slower apt on a small Pi.
if dpkg --print-foreign-architectures | grep -qx armhf &&
   [ "$(dpkg-query -W -f='${Architecture}\n' | grep -cx armhf)" = 0 ]; then
  dpkg --remove-architecture armhf
fi

say "Packages"
# A fresh image's package lists are out of date: their files are gone from the mirror.
apt-get update -q >/dev/null
$APT install batctl iw curl jq pipx alsa-utils espeak-ng nftables polkitd >/dev/null
if [ "$noscreen" = 1 ]; then echo "Under 1 GB of memory: no screen kiosk."
else $APT install cage chromium fonts-noto-color-emoji wlr-randr >/dev/null; fi

say "meshtasticd (Meshtastic's Linux daemon) from its Debian 13 repository"
if [ ! -f /etc/apt/sources.list.d/meshtastic.sources ]; then
  curl -fsSL https://download.opensuse.org/repositories/network:Meshtastic:beta/Debian_13/Release.key \
    | gpg --dearmor --yes -o /usr/share/keyrings/meshtastic.gpg
  cat >/etc/apt/sources.list.d/meshtastic.sources <<'EOF'
Types: deb
URIs: https://download.opensuse.org/repositories/network:/Meshtastic:/beta/Debian_13/
Suites: /
Signed-By: /usr/share/keyrings/meshtastic.gpg
EOF
  apt-get update -q >/dev/null
fi
$APT install meshtasticd >/dev/null
# The Elecrow Meshstick (CH341 USB-to-SPI with an SX1262), official in meshtasticd.
ln -sf /etc/meshtasticd/available.d/lora-usb-meshstick-1262.yaml /etc/meshtasticd/config.d/lora-usb-meshstick-1262.yaml
# Meshtastic's own web client on https://<pi>:9443 (a self-signed certificate).
sed -i -e 's|^#  Port: 9443|  Port: 9443|' -e 's|^#  RootPath: /usr/share/meshtasticd/web|  RootPath: /usr/share/meshtasticd/web|' /etc/meshtasticd/config.yaml
install -d /etc/systemd/system/meshtasticd.service.d
install -m644 systemd/meshtasticd-override.conf /etc/systemd/system/meshtasticd.service.d/batnet.conf

say "batman-adv and the USB WiFi stick"
echo batman-adv >/etc/modules-load.d/batnet.conf
modprobe batman-adv || true
# The built-in WiFi (wlan0) stays with NetworkManager; the USB stick is batman's.
install -m644 networkmanager-batnet.conf /etc/NetworkManager/conf.d/batnet.conf
install -d /usr/local/lib/batnet
install -m755 batnet-mesh /usr/local/lib/batnet/batnet-mesh

say "The power-cut kit: own WiFi, power modes, terminal, sound, LED (plan 5a)"
# What batnetd may do as root, through polkit (decision 0031).
install -m644 50-batnet.rules /etc/polkit-1/rules.d/50-batnet.rules
install -m755 batnet-power batnet-kiosk /usr/local/lib/batnet/
# The hub's own WiFi open to neighbours: a firewall that leaves only the app,
# and every name pointing at the hub (a captive portal) (decision 0028).
install -m644 own-wifi-open.nft /usr/local/lib/batnet/own-wifi-open.nft
install -m755 90-batnet-own-wifi /etc/NetworkManager/dispatcher.d/90-batnet-own-wifi
install -d /etc/NetworkManager/dnsmasq-shared.d
install -m644 dnsmasq-batnet.conf /etc/NetworkManager/dnsmasq-shared.d/batnet.conf
# The text login the Terminal button switches to says how to come back.
install -d /etc/issue.d
install -m644 batnet.issue /etc/issue.d/batnet.issue

say "Watchdogs"
# The Pi's hardware watchdog reboots it if the system hangs for 15 s.
install -d /etc/systemd/system.conf.d
install -m644 systemd/watchdog.conf /etc/systemd/system.conf.d/batnet-watchdog.conf

say "Logs on the card (at most 400 MB), not only in memory"
install -d /etc/systemd/journald.conf.d
install -m644 systemd/journald.conf /etc/systemd/journald.conf.d/50-batnet.conf
systemctl restart systemd-journald
journalctl --flush

say "batnet users and services"
id batnet >/dev/null 2>&1 || useradd --system --home-dir /var/lib/batnet --shell /usr/sbin/nologin batnet
id batnet-screen >/dev/null 2>&1 || useradd --system --create-home --home-dir /var/lib/batnet-screen \
  --shell /usr/sbin/nologin --groups video,render,input,audio batnet-screen
if [ -f batnetd ]; then install -m755 batnetd /usr/local/bin/batnetd; fi
install -m644 systemd/batnet-hub.service systemd/batnet-clock.service systemd/batnet-clock.timer \
  systemd/batnet-power@.service systemd/batnet-console.service /etc/systemd/system/
# Without the screen unit batnetd never starts a kiosk.
if [ "$noscreen" = 1 ]; then
  systemctl disable --now batnet-screen.service 2>/dev/null || true
  rm -f /etc/systemd/system/batnet-screen.service
else install -m644 systemd/batnet-screen.service /etc/systemd/system/; fi
systemctl daemon-reload
systemctl enable --now batnet-clock.timer >/dev/null
systemctl enable meshtasticd.service >/dev/null
systemctl restart meshtasticd.service
if [ -x /usr/local/bin/batnetd ]; then
  systemctl enable batnet-hub.service >/dev/null
  systemctl restart batnet-hub.service
  if [ "$noscreen" != 1 ]; then
    systemctl enable batnet-screen.service >/dev/null
    # Only with a screen plugged in (batnetd starts and stops it as screens come and go).
    if grep -qsx connected /sys/class/drm/card*-HDMI-A-*/status; then systemctl restart batnet-screen.service || true; fi
  fi
fi

say "Meshtastic command-line tool for the pi user (meshtastic --host localhost ...)"
user=$(getent passwd 1000 | cut -d: -f1)
sudo -u "$user" pipx install meshtastic >/dev/null 2>&1 || sudo -u "$user" pipx upgrade meshtastic >/dev/null 2>&1 || true

say "pmtiles, which fetches the offline maps (decision 0032)"
PMTILES_VERSION=1.31.2
PMTILES_SHA256=f8bd47e7ea866863489cad588fbaf2f31f42e5821f7a03f009b3769f05801cb1 # Linux arm64, from the release
if ! pmtiles version 2>/dev/null | grep -q "$PMTILES_VERSION"; then
  tmp=$(mktemp -d)
  if curl -fsSL -o "$tmp/p.tar.gz" "https://github.com/protomaps/go-pmtiles/releases/download/v$PMTILES_VERSION/go-pmtiles_${PMTILES_VERSION}_Linux_arm64.tar.gz" &&
    echo "$PMTILES_SHA256  $tmp/p.tar.gz" | sha256sum -c --quiet &&
    tar -xzf "$tmp/p.tar.gz" -C "$tmp" pmtiles; then
    install -m 755 "$tmp/pmtiles" /usr/local/bin/pmtiles
  else
    echo "pmtiles did not install (it needs the internet once); the map uses online tiles until it does."
  fi
  rm -rf "$tmp"
fi

say "Offline translator (German and English; decision 0030)"
if [ "$notrans" = 1 ]; then echo "Skipped: it needs a Pi with 2 GB or more."
else ./translate-setup.sh || echo "The translator did not install (it needs the internet once); the hub works without it."; fi

say "Done"
systemctl --no-pager --lines=0 status meshtasticd batnet-hub $([ "$noscreen" = 1 ] || echo batnet-screen) | grep -E "●|Active:" || true
