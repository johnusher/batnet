---
type: Glossary
title: Glossary
description: Mesh, LoRa, radio and web terms used across the bundle.
tags: [glossary]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
---

| Term | Meaning |
|---|---|
| batman-adv | Linux kernel mesh routing at layer 2; nodes appear on one Ethernet-like interface, `bat0` |
| IBSS | WiFi ad-hoc mode: radios talk directly without an access point |
| 802.11s | The WiFi mesh standard; an alternative to IBSS under batman-adv |
| TQ | batman-adv's link-quality metric (transmit quality, 0–255) |
| LoRa | Long-range, low-rate chirp-spread-spectrum radio in sub-GHz bands |
| SF | Spreading factor: higher = longer range, slower, more airtime |
| Duty cycle | Share of time a device may transmit; 1% or 10% in EU868 sub-bands |
| LongFast | Meshtastic's default modem preset (SF11, 250 kHz in the EU) |
| meshtasticd | Meshtastic running natively on Linux, driving an SPI LoRa HAT |
| PSK | Pre-shared key; a Meshtastic channel's encryption key |
| Village key | batnet's one shared secret per village; seals node traffic and derives the private channel PSK |
| House / relay / gateway | batnet node roles: hotspot + app / mesh only / LoRa bridge |
| HLC | Hybrid logical clock: wall time plus a counter, so ordering survives bad clocks |
| Version vector | Per origin, the highest sequence number a node holds; compared to find gaps |
| Captive portal | The page a phone pops up after joining a hotspot |
| Secure context | HTTPS or localhost; required for microphone access in browsers |
| WebRTC | Browser real-time audio/video; Pion is a pure-Go implementation |
| SDR | Software-defined radio receiver, e.g. RTL-SDR |
| PMR446 | Licence-free EU walkie-talkie band, 446.0–446.2 MHz |
| OKF | Open Knowledge Format: this bundle's format |
