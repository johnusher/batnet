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

# USB boards (alternative)

ESP32 + SX1262 boards (Heltec V3, LilyGo T-Beam) flashed with Meshtastic, on USB serial.
