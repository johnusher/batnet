---
type: Hardware
title: pz7, the second bench hub (a Pi Zero 2 W)
description: The Raspberry Pi Zero 2 W with an Elecrow Meshstick that became batnet's second bench hub on 2026-10-23 - a small hub with no screen kiosk and no translator.
tags: [bench, raspberry-pi, zero-2-w, hub, meshtasticd, meshstick, small-pi]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-23T05:30:00Z }
verified: { by: process:2026-10-23-pz7-deploy, at: 2026-10-23T05:20:00Z }
stale_after: 2027-04-23T00:00:00Z
---

"pz7" (Pi Zero 7) is the project's name for this board. It had an earlier use
in another project; its files from then stay in its home folder and nothing of
theirs starts on boot. Its address, login, MACs and radio node ID are private
and stay out of this repo ([secret hygiene](/security/secret-hygiene.md)).

# The machine

| Part | What |
|---|---|
| Board | Raspberry Pi Zero 2 W rev 1.0, 512 MB (416 MB usable), swap on zram |
| Storage | 8 GB microSD: 6.8 GB root. 1.1 GB free after set-up on the desktop image; 3.2 GB free once the desktop was removed |
| OS | Raspberry Pi OS (Debian 13 trixie, arm64), kernel 6.12. Came as the desktop image, booting to text; the desktop was removed on 2026-10-23 (below) |
| LoRa | Elecrow **Meshstick** (CH341, USB ID `1a86:5512`) on the Zero's one micro-USB data port; a USB WiFi stick for batman-adv would need a hub as well. Found by meshtasticd as soon as it was plugged in |
| WiFi | built-in only, on the home network |
| Screen | none: no kiosk on a small Pi |

# What runs

`setup.sh` sees under 1 GB of memory and takes its small-Pi path: no cage,
Chromium or emoji font, no `batnet-screen.service` (so batnetd never starts a
kiosk) and no offline translator. Everything else is as on the
[first bench hub](/hardware/bench-pi7.md): meshtasticd with the Meshstick
config, `batnet-hub.service` (the app on `http://<pi>/`, the panel on
`http://<pi>/hub/`), batman-adv tools, the power-cut kit, watchdogs, the
Meshtastic command-line tool and pmtiles. `BATNET_SMALL=0` or `1` overrides the
guess.

Measured on 2026-10-23: 158 MB of 416 MB in use with batnetd and no radio,
152 MB with the radio running, 264 MB available. With no Meshstick, meshtasticd
stops with "Could not initialize CH341 device!" and systemd starts it again
every 5 s; the panel shows no radio.

# Radio settings

A new Meshstick has no region and will not send ("lora tx disabled: Region
unset"). Set on 2026-10-23 with the Meshtastic command-line tool, as on the
first hub: EU_868, MediumFast, hop limit 3, transmit power the region's default;
node name "lorabatnet2", short name LB2. It heard 16 packets and 6
radios in its first 90 s. The same second channel as the first hub,
**BerlinMesh**, with the key copied from it (not in this repo). No place is set,
so it shares no position.

The two hubs hear each other straight. On 2026-10-23 a private message from
this hub to the first, sent with the Meshtastic command-line tool, arrived and
was acknowledged (PKI: each had the other's public key from its
name card).

# One village with the first hub

`village.key` was copied from the first hub on 2026-10-23 (by hand, B37), so
both hubs seal and open the same village's log. The key it had made for itself
and its empty log went into a dated folder beside them in `/var/lib/batnet/`;
its own `node.key` stayed.

# The desktop removed

The card came with Raspberry Pi OS's desktop image: Chromium, Firefox, the
desktop, VLC, Thonny, Geany, RealVNC, Imager and the user guide, about 650
packages with what only they needed, 2.2 GB. A hub never starts any of it, so
on 2026-10-23 they were purged, after marking what the hub needs as wanted
(NetworkManager, ssh, avahi, polkit, meshtasticd, Python and pipx, every WiFi
firmware for a USB stick, the kernels, git). Everything ran on and the Pi came
back by itself after a reboot.

# Limits

* No translator and no screen ([L26](/known-problems.md)).
* An 8 GB card leaves room for few offline maps above the 2 GB a hub keeps free
  ([0032](/decisions/0032-offline-maps.md)). Berlin's (87 MB) was fetched on
  2026-10-23 once the desktop was gone, and the hub serves it with range
  requests like the first hub; 3.2 GB stayed free.

# Rebuilding it

`deploy/pi/deploy.sh <ssh-host>` from a laptop, as for the first hub. The first
run on this card failed on out-of-date package lists; `setup.sh` now runs
`apt-get update` first.
