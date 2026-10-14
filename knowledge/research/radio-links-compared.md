---
type: Research Note
title: Radio links compared
description: Range, usable rate and what each link can carry - Pi WiFi with batman-adv, LoRa, WiFi HaLow, point-to-point WiFi.
tags: [radio, wifi, lora, halow, range]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: halow-pi5
    resource: https://github.com/buildwithparallel/openwrt-morse-rpi5
    title: OpenWrt with Morse Micro HaLow on Raspberry Pi 5 (community)
  - id: ttn-sf
    resource: https://thethingsnetwork.org/docs/lorawan/spreading-factors
    title: The Things Network - spreading factors
  - id: halow-dev
    resource: https://morsemicro.com/products/development-resources
    title: Morse Micro HaLow development resources
    author: vendor:morse-micro
---

# Summary

| Link | Range per hop | Usable rate | Carries |
|---|---|---|---|
| Pi built-in WiFi, IBSS + batman-adv | ~50–150 m line of sight; the Zero is weaker | Mbps | everything |
| USB WiFi dongle with an external antenna | ~300–500 m line of sight, much less through walls and trees | Mbps | everything |
| Outdoor point-to-point WiFi pair (e.g. TP-Link CPE210), Ethernet into the Pi | 1–3 km+ line of sight | tens of Mbps | everything; batman-adv runs over the Ethernet |
| LoRa sub-GHz (SX1262) | 1 km easy, 5–10 km+ line of sight | 0.3–5.5 kbit/s raw[^ttn-sf], 10% airtime in the EU 869.4–869.65 MHz sub-band | text, telemetry, tiny images |
| WiFi HaLow 802.11ah | ~1 km (vendor claims) | Mbps | everything, but HATs are pricey and drivers young[^halow-dev][^halow-pi5] |

# Notes

* Ranges are rules of thumb, not measurements. Replace with field tests (`verified: process:field-test-…`).
* batman-adv needs an Ethernet-like interface, so LoRa is a separate layer: [decision 0001](/decisions/0001-two-layer-network.md).
