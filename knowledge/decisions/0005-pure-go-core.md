---
type: Decision
title: "0005: One pure-Go daemon, roles from config, heavy work in helpers"
description: batnetd is a single CGo-free Go binary on every node; speech, codecs and SDR run as separate helper processes.
tags: [go, architecture, cross-compile]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Decision

* **batnetd**, pure Go (`CGO_ENABLED=0`), cross-compiled from a laptop for
  every Pi including the ARMv6 Zero W (`GOARM=6`) and arm64.
* Role from config: `house` (hotspot, web app, accounts), `relay` (mesh only),
  `gateway` (LoRa bridge; combinable with `house`).
* Speech-to-text, text-to-speech, audio codecs and the SDR sweep run as
  **helper processes** over a local socket (e.g. the Wyoming protocol, which
  already wraps Piper, Whisper and Vosk). batnetd stays portable.
* A **mock village mode** runs several fake houses on one laptop, like the
  author's earlier `-no-batman` mock, so most development needs no hardware.
* Storage: SQLite via `modernc.org/sqlite` (pure Go).
