---
type: Hardware
title: LoRa radios for Meshtastic on a Pi
description: SPI SX1262 HATs for meshtasticd (the default) and USB Meshtastic boards (the alternative), with what to watch for when buying.
tags: [lora, sx1262, meshtastic, hat]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: mesh-linux
    resource: https://meshtastic.org/docs/hardware/devices/linux-native-hardware
    title: Meshtastic Linux-native hardware
  - id: rak6421
    resource: https://store.rakwireless.com/products/meshtastic-raspberry-pi-hat-rak6421
    title: RAK WisMesh Pi HAT (RAK6421)
    author: vendor:rakwireless
  - id: meshpi
    resource: https://lectronz.com/products/mesh-pi
    title: Mesh PI Tiny
    author: vendor:lectronz-seller
---

# SPI HATs (default, [decision 0002](/decisions/0002-sx1262-spi-hat.md))

* Meshtastic lists Pi Zero, Zero 2, 3, 4, 400 and 5 as tested; only SPI radios work; UART HATs and SX1302/SX1303 concentrators do not.[^mesh-linux]
* Candidates: Waveshare SX126X (SPI version only)[^mesh-linux], RAK WisMesh Pi HAT RAK6421[^rak6421], Mesh PI Tiny (E22-900MM22S, Pi Zero size, up to 22 dBm)[^meshpi].
* Pick the 868 MHz variant in the EU.
* Update 2026-10-15: meshtasticd's hardware configs mark **RAK6421 + RAK13300 as `official`** and the Waveshare SPI LoRaWAN HAT as **deprecated** (CRC errors on longer messages). Waveshare's similarly named "SX1262 LoRa HAT" is the UART version and does not work. The Elecrow **Meshstick** (USB, CH341 to SPI SX1262) is supported and sold at Berrybase. Recommended: RAK6421 + RAK13300 (~€25) for a HAT, Meshstick (~€25) for USB. See the [EU shopping list](/hardware/shopping-list-eu.md).

# USB boards (alternative)

ESP32 + SX1262 boards (Heltec V3, LilyGo T-Beam) flashed with Meshtastic, on USB serial.

# Bench kit

2026-10-15: two Elecrow Meshsticks (USB, EU868) bought for the first bench test: two Pis with meshtasticd, one stick each. Results go in [tests](/tests/).

2026-10-21: the first Meshstick runs on the [bench hub](/hardware/bench-pi7.md) with meshtasticd 2.7.26 and its `lora-usb-meshstick-1262.yaml` config ([test](/tests/2026-10-21-pi7-hub-bring-up.md)).
