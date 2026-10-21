---
type: Test Report
title: "2026-10-21: pi7 hub bring-up"
description: First real hub - a Pi 4 with a USB Meshstick runs meshtasticd, batnetd -hub and the HDMI kiosk; the radio, the panel, mesh mode switching, the watchdogs and a cold boot were checked.
tags: [bench, hub, meshtasticd, meshstick, watchdog, kiosk, systemd]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T14:45:00Z }
---

# Setup

One [bench hub](/hardware/bench-pi7.md): Raspberry Pi 4 (2 GB), Raspberry Pi
OS trixie, one Elecrow Meshstick (EU868) on USB, no USB WiFi stick yet, no
monitor attached. The second Meshstick is not in use yet. Set up with
`deploy/pi/setup.sh`.

# Results

| Check | Result |
|---|---|
| meshtasticd 2.7.26 drives the Meshstick (`lora-usb-meshstick-1262.yaml`) | `sx1262 init success` |
| Region unset after install | Meshtastic refuses to transmit ("lora tx disabled: Region unset") until the region is set: safe by default |
| After `--set lora.region EU_868` | 869.525 MHz, LongFast, 27 dBm configured |
| batnet's own Meshtastic client (`internal/meshtastic`, hand-decoded protobuf) against the real daemon | node number, firmware, region, preset, power, hop limit, channels and the node list all read correctly |
| Radios heard | none but itself (no other Meshtastic node in range of the bench) |
| Panel from the Pi (loopback) | shows the hub and admin PINs; buttons work without a PIN |
| Panel from the LAN | PINs not sent; buttons answer "The hub PIN is shown on the hub's screen" without the hub PIN |
| Mesh mode off → mock → real → off from the panel | each switch re-runs batnetd in place in about a second; mock: four pretend houses sync the log with the hub (17 entries everywhere within 10 s); real without a WiFi stick: "Waiting for the USB WiFi stick" |
| systemd watchdog: batnetd frozen (`kill -STOP`) | restarted by systemd within about 30 s |
| meshtasticd killed (`kill -9`) | back in 5 s; batnetd reconnected by itself |
| Cold reboot | all three services up, 24 s to multi-user; hardware watchdog armed (15 s); no failed units; auto-login still off |
| Memory with everything running | 1.3-1.5 GB free of 1.8 GB; Chromium ~350 MB, batnetd ~35 MB |
| Kiosk rendering (cage + Chromium on a virtual display) | panel renders in Nunito Sans; the HDMI ports read "disconnected", so the real screen is not yet seen |

# What it settles

* A 2 GB Pi 4 is enough for a milestone 1 hub, kiosk included.
* meshtasticd's TCP API can be read and written from pure Go without generated
  protobuf code; the fields batnet needs are few.
* Mode switching by re-running in place works under systemd `Type=notify`
  (after a fix: batnetd must not send `STOPPING=1` before re-running, or systemd
  marks the service as deactivating).

# Not tested yet

* The real HDMI screen and keyboard; whether cage picks up a monitor plugged in
  after boot.
* Real mode with a USB WiFi stick: IBSS, batman-adv, bat0, sync with a second
  Pi ([B34](/known-problems.md)).
* Any transmission heard by another radio: needs the second Meshstick.
