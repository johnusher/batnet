---
type: Hardware
title: pi7, the first bench hub
description: The Raspberry Pi 4 (2 GB) with an Elecrow Meshstick that is batnet's first real hub from 2026-10-21 - what it is, what runs on it, how it starts and how to rebuild it.
tags: [bench, raspberry-pi, hub, meshtasticd, meshstick, batman-adv, kiosk]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T14:45:00Z }
verified: { by: process:2026-10-21-pi7-hub-bring-up, at: 2026-10-21T14:40:00Z }
stale_after: 2027-04-21T00:00:00Z
---

"pi7" is the project's name for this board: it used to be a development Pi in
another project and was wiped of its services for batnet on 2026-10-21. Its
address, login, MACs and radio node ID are private and stay out of this repo
(secret hygiene); find them in the owner's notes.

# The machine

| Part | What |
|---|---|
| Board | Raspberry Pi 4 Model B rev 1.5, **2 GB** RAM (1.8 GB usable) |
| Storage | 32 GB microSD, about 18 GB free after set-up |
| OS | Raspberry Pi OS (Debian 13 trixie, arm64, the desktop image), kernel 6.18 |
| LoRa | Elecrow **Meshstick**: USB, a CH341 USB-to-SPI bridge (USB ID `1a86:5512`) driving an SX1262; EU868; its own antenna |
| WiFi | built-in (brcmfmac) on the home network for now; a USB WiFi stick for batman-adv still to be plugged in (it becomes `wlan1`) |
| Screen | a small HDMI display, 800 × 480 (its EDID also offers 1080p, but 800 × 480 is native), on HDMI 0, and a USB keyboard. The panel's screen layout fits it without scrolling |

# Is a 2 GB Pi 4 enough for a hub?

Yes for milestone 1, measured: the whole hub (meshtasticd, batnetd, Chromium
in the kiosk) leaves about 1.3 GB of the 1.8 GB free; Chromium is the biggest
user at about 350 MB. batnetd itself is 35 MB. Speech-to-text (milestone 2)
runs Moonshine Tiny in a few hundred MB on a Pi 4 ([0014](/decisions/0014-speech-pipeline.md));
a Pi 5 is only needed for the larger speech model. The SD card has room for
years of text log. Watch for: the USB WiFi stick on a USB 3 (blue) port can
disturb 2.4 GHz WiFi, so use a black USB 2 port or a short extension; and a
good 3 A supply (the stick and the Meshstick draw from the Pi).

# What runs (and in what order)

```
power on
  └─ meshtasticd.service       the LoRa radio (Meshtastic's Linux daemon), TCP API on 4403,
  │                            Meshtastic's own web client on https://<pi>:9443
  └─ batnet-hub.service        batnetd -hub: the house app on http://<pi>/,
  │    (after meshtasticd)     the control panel on http://<pi>/hub/, the WiFi mesh
  └─ batnet-screen.service     cage + Chromium kiosk on tty1: the panel full screen on HDMI
       (after batnet-hub;      (waits up to a minute for the panel to answer first)
        Ctrl+Alt+F2 gives a normal login prompt)
```

Ready about 25 s after power on. Every service restarts by itself after a crash
(`Restart=always`, no start limit). batnetd tells systemd it is alive every
10 s, but only while its own panel answers; 30 s of silence and systemd
restarts it. The Pi's hardware watchdog reboots the board if systemd itself
hangs for 15 s. The kiosk's browser profile lives in memory and is new on every start (a profile left from before a hostname change once stopped Chromium). All of this was tried on 2026-10-21 (see the [test](/tests/2026-10-21-pi7-hub-bring-up.md)).

# Radio settings

EU_868, LongFast (869.525 MHz) until the evening of 2026-10-21, then MediumFast (also 869.525 MHz) to compare, since much of Berlin uses it ([presets in Berlin](/research/meshtastic-presets-berlin.md)); hop limit 3, 27 dBm configured (the
Meshstick's SX1262 tops out at 22 dBm; the band allows 27 dBm ERP), node name
"batnet hub 1", short name BH1. Meshtastic's firmware keeps the EU868 duty
cycle; the panel adds a 5 s gap between sends; the log says hello every 5 minutes, or as `helloMinutes` in `hub.json` says (60 at home overnight; negative: listen only).

# Rebuilding it

From a laptop with the repo: `deploy/pi/deploy.sh <ssh-host>` builds batnetd
for arm64, copies it and `deploy/pi/` over and runs `setup.sh`, which is safe to
run again. It removes printing, ModemManager (it grabs USB serial devices),
NFS, VNC and remote access, Bluetooth, auto-mounting, PackageKit and
cloud-init; installs meshtasticd from Meshtastic's Debian 13 repository with the
Meshstick config, batctl, iw, cage, Chromium and the Meshtastic command-line
tool; sets up batman-adv for the USB stick, the watchdogs and the three
services. The region and node name are set once with
`meshtastic --host localhost --set lora.region EU_868` and `--set-owner`.
`deploy.sh <ssh-host> --binary` only replaces batnetd.

**The log** (panel: Start logging): every packet heard, every packet heard but too weak to read (from meshtasticd's journal), a sample every 30 s and a hello every 5 minutes, as JSON lines in `/var/lib/batnet/logs/`; it stays on through power cuts until stopped. Each line has the Pi's time `t` and `up`, seconds since boot. With no internet the Pi's clock starts from the time saved by `batnet-clock.timer` (every minute), so it lags by the time without power; `up` plus the moment of power-on (a phone GPS track started then) gives exact times. First used for a bike ride on 2026-10-21.

Data lives in `/var/lib/batnet/`: `house.db` (the log), `node.key`,
`village.key` (copy it to another Pi to put both in one village, until the QR
join of plan 5), `hub-pin`, `admin-pin` and `hub.json` (the mesh mode and the
house name).
