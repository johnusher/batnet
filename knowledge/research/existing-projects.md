---
type: Research Note
title: Existing off-grid mesh and walkie-talkie projects
description: Meshtastic, MeshCore, Reticulum and Pi walkie-talkie projects - what each does and what batnet takes from them.
tags: [meshtastic, meshcore, reticulum, prior-art]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
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
---

# Projects

* **Meshtastic** — LoRa text mesh, largest community. `meshtasticd` runs natively on Pi Zero–Pi 5 with SPI HATs.[^mesh-linux] Broadcasts flood, 7-hop cap; since 2.6 DMs use next-hop routing.[^meshcore] **batnet uses it** for the long-range layer.
* **MeshCore** — LoRa with planned repeaters and source routing, up to 64 hops, smaller community.[^meshcore]
* **Reticulum** — encrypted, transport-agnostic network stack across LoRa, WiFi, serial; LXMF messaging, LXST voice, Sideband app.[^ret-vs-mesh] Go ports: Reticulum-Go[^ret-go] and go-reticulum[^go-ret], both young. A candidate future interop target.
* **talkiepi** — Pi push-to-talk over Mumble; its Go fork is unmaintained.[^talkiepi]
* **walkie** — Python Opus push-to-talk over LAN broadcast.[^walkie]

# The gap

Nothing found combines a batman-adv mesh, browser-only WhatsApp-like UX,
on-device speech-to-text for LoRa, and a radio scanner. That is batnet.
