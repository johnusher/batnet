---
type: Research Note
title: Existing off-grid mesh and walkie-talkie projects
description: Meshtastic, MeshCore, Reticulum and Pi walkie-talkie projects - what each does and what batnet takes from them.
tags: [meshtastic, meshcore, reticulum, prior-art]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: chaosmesh
    resource: https://potatomesh.net/pages/about
    title: Berlin Chaos Mesh - the settings Berlin's LoRa community publishes for Meshtastic, MeshCore and Reticulum
  - id: mesh-linux
    resource: https://meshtastic.org/docs/hardware/devices/linux-native-hardware
    title: Meshtastic Linux-native hardware
  - id: ret-vs-mesh
    resource: https://blog.erethon.com/blog/2024/01/31/comparing-reticulum-and-meshtastic/
    title: Comparing Reticulum and Meshtastic
  - id: meshcore
    resource: https://simeononsecurity.com/articles/meshcore-vs-meshtastic-comparison-guide/
    title: MeshCore vs Meshtastic
  - id: ret-go
    resource: https://pkg.go.dev/git.quad4.io/Networks/Reticulum-Go
    title: Reticulum-Go
  - id: go-ret
    resource: https://pkg.go.dev/github.com/svanichkin/go-reticulum
    title: go-reticulum
  - id: talkiepi
    resource: https://www.raspberrypi.com/news/talkiepi-raspberry-pi-walkie-talkie/
    title: talkiepi
  - id: walkie
    resource: https://pypi.org/project/walkie/
    title: walkie (LAN Opus push-to-talk)
  - id: meshcore-roles
    resource: https://littlebirdelectronics.com.au/blogs/news/meshcore-vs-meshtastic-what-s-actually-different-and-which-should-you-flash
    title: MeshCore vs Meshtastic, what's actually different
  - id: lxmf
    resource: https://github.com/markqvist/LXMF
    title: LXMF, propagation nodes
  - id: briar
    resource: https://briarproject.org/
    title: Briar
  - id: bridgefy
    resource: https://bridgefy.me/
    title: Bridgefy
---

# Projects

* **Meshtastic** — LoRa text mesh, largest community. `meshtasticd` runs natively on Pi Zero–Pi 5 with SPI HATs.[^mesh-linux] Broadcasts flood, 7-hop cap; since 2.6 DMs use next-hop routing.[^meshcore] **batnet uses it** for the long-range layer.
* **MeshCore** — LoRa with planned repeaters and source routing, up to 64 hops, smaller community.[^meshcore] Roles are distinct firmware jobs: *companions* (a person's radio, paired to the phone app; they do not relay), *repeaters* (the backbone, sited on hills and roofs) and *room servers* (keep message history for people who were offline).[^meshcore-roles] Same boards as Meshtastic, a re-flash away.
* **Reticulum** — encrypted, transport-agnostic network stack across LoRa, WiFi, serial; LXMF messaging, LXST voice, Sideband app.[^ret-vs-mesh] LXMF *propagation nodes* store and forward for offline recipients.[^lxmf] Go ports: Reticulum-Go[^ret-go] and go-reticulum[^go-ret], both young. A candidate future interop target.
* **All three side by side in one city:** Berlin's community runs Meshtastic on MediumFast (869.525 MHz, 250 kHz), MeshCore on its EU narrow setting (869.618 MHz, 62.5 kHz) and Reticulum (869.475 MHz, 125 kHz), each in the same 869.4–869.65 MHz slice.[^chaosmesh] See [presets in Berlin](/research/meshtastic-presets-berlin.md).
* **Phone-to-phone apps** — Briar (Android; Bluetooth, WiFi, Tor)[^briar] and Bridgefy (Bluetooth)[^bridgefy] pass messages between phones with no hardware. Reach is tens of metres and everyone must install the same app.
* **talkiepi** — Pi push-to-talk over Mumble; its Go fork is unmaintained.[^talkiepi]
* **walkie** — Python Opus push-to-talk over LAN broadcast.[^walkie]

# The gap

Nothing found combines a batman-adv mesh, browser-only WhatsApp-like UX,
on-device speech-to-text for LoRa, and a radio scanner. That is batnet.

# What batnet adds, as told to the public

The README's section *How batnet differs from Meshtastic and the others*
(2026-10-15) puts it in seven points: no radio or app per person (a node per
house serves phones over open WiFi); calls, voice and photos between nearby
houses over the WiFi mesh, where the LoRa messengers carry text only; accounts,
approval, a directory and groups where the others have channels and keys;
every node keeps the whole history and fills gaps (MeshCore's room servers and
Reticulum's propagation nodes do this, but as extra boxes); speech crossing
the radio as text (M2); no-phone mode; the scan page (M4). It also says where
batnet is weaker: not finished, a Pi needs more power than a radio, and a
village needs a node per house.
