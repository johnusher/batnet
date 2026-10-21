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

say "Packages"
$APT install batctl iw cage chromium curl jq pipx fonts-noto-color-emoji >/dev/null

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

say "Watchdogs"
# The Pi's hardware watchdog reboots it if the system hangs for 15 s.
install -d /etc/systemd/system.conf.d
install -m644 systemd/watchdog.conf /etc/systemd/system.conf.d/batnet-watchdog.conf

say "batnet users and services"
id batnet >/dev/null 2>&1 || useradd --system --home-dir /var/lib/batnet --shell /usr/sbin/nologin batnet
id batnet-screen >/dev/null 2>&1 || useradd --system --create-home --home-dir /var/lib/batnet-screen \
  --shell /usr/sbin/nologin --groups video,render,input,audio batnet-screen
if [ -f batnetd ]; then install -m755 batnetd /usr/local/bin/batnetd; fi
install -m644 systemd/batnet-hub.service systemd/batnet-screen.service systemd/batnet-clock.service systemd/batnet-clock.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now batnet-clock.timer >/dev/null
systemctl enable meshtasticd.service >/dev/null
systemctl restart meshtasticd.service
if [ -x /usr/local/bin/batnetd ]; then
  systemctl enable batnet-hub.service batnet-screen.service >/dev/null
  systemctl restart batnet-hub.service batnet-screen.service
fi

say "Meshtastic command-line tool for the pi user (meshtastic --host localhost ...)"
user=$(getent passwd 1000 | cut -d: -f1)
sudo -u "$user" pipx install meshtastic >/dev/null 2>&1 || sudo -u "$user" pipx upgrade meshtastic >/dev/null 2>&1 || true

say "Done"
systemctl --no-pager --lines=0 status meshtasticd batnet-hub batnet-screen | grep -E "●|Active:"
