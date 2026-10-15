---
type: Hardware
title: EU shopping list with prices (October 2026)
description: Parts for a house node, a house with long-range radio and a pocket node, with Berrybase and other EU shop article numbers and euro prices.
tags: [shopping, prices, berrybase, eu, hardware]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:46:20Z }
stale_after: 2027-01-15T00:00:00Z
sources:
  - id: berrybase
    resource: https://www.berrybase.de/
    title: Berrybase product pages (Pi boards, power supplies, SD cards, cases, Meshstick, antennas)
    author: vendor:berrybase
  - id: wlanshop
    resource: https://www.wlan-shop24.de/alfa-network-awus036achm-wlan-usb-adapter-802-11ac-dual-band
    title: ALFA AWUS036ACHM at wlan-shop24
    author: vendor:wlan-shop24
  - id: hexaspot
    resource: https://hexaspot.com/products/wismesh-pi-hat-rak6421
    title: RAK6421 WisMesh Pi HAT at Hexaspot
    author: vendor:hexaspot
  - id: hexaspot-13300
    resource: https://hexaspot.com/products/module-for-lorawan-with-semtech-lora-chip-sx1262-rak13300
    title: RAK13300 SX1262 module at Hexaspot
    author: vendor:hexaspot
  - id: meshtasticd-hw
    resource: https://meshtastic.org/docs/hardware/devices/linux-native-hardware
    title: Meshtastic Linux-native hardware
---

Prices include VAT, exclude shipping, seen 2026-10-15. Several Pis were sold out
at Berrybase that day. Prices and stock change: re-check before publishing totals.

# Parts

| Part | Shop | Article no. | Maker part / EAN | € | Notes |
|---|---|---|---|---|---|
| Raspberry Pi 5, 4 GB | Berrybase | RPI5-4GB | SC1111 / 5056561803319 | 118.50 | sold out on the day; 8 GB (RPI5-8GB) 184.90 |
| Raspberry Pi 4 B, 4 GB | Berrybase | RPI4B-4GB | 0765756931182 | 105.90 | sold out; 3 GB (RPI4B-3GB, SC2337) 90.90 in stock |
| Raspberry Pi Zero 2 W | Berrybase | RPIZ-2W | SC0510 / 5056561800004 | 18.90 | pocket node or relay |
| 27 W USB-C power supply (Pi 5) | Berrybase | RPI5NT5AW | SC1152 | 12.40 | |
| 15 W USB-C power supply (Pi 4) | Berrybase | RPI4NT3AW | KSA-15E-051300-HE | 7.90 | |
| Micro-USB power supply (Zero) | Berrybase | RPIMNT25W | SC0624 | 7.90 | or a power bank |
| SanDisk Ultra microSD 32 GB A1 | Berrybase | 619659184155 | SDSQUA4-032G-GN6MA | 17.90 | |
| Official Pi 5 case (with fan) | Berrybase | RPI5-CASE-RE | SC1159 | 9.90 | no separate cooler needed |
| Official Pi 4 case | Berrybase | RPI4-CASE-RE | 0644824914916 | 4.90 | |
| ALFA AWUS036ACHM WiFi stick | wlan-shop24 | ART7230 | AWUS036ACHM | 42.99 | MT7610U: AP, IBSS and 802.11s in mainline |
| RAK6421 WisMesh Pi HAT | Hexaspot | RAK-610012 | 610012 | 14.42 | carrier; meshtasticd support `official`[^meshtasticd-hw] |
| RAK13300 SX1262 module, EU868 | Hexaspot | RAK-105006 | 105006 | 10.48 | plugs into the RAK6421; antenna included |
| Elecrow Meshstick USB SX1262, EU868 | Berrybase | EL-LMM15202D | LMM15202D | 24.90 | USB alternative to the HAT, supported by meshtasticd |
| 868 MHz 5 dBi antenna, magnetic foot | Berrybase | WS-24412 | | 4.90 | optional, SMA |
| TP-Link CPE210 outdoor link | reichelt | TPLINK CPE210 | CPE210 | 41.99 each | optional, buy in pairs, 1–3 km |

# Do not buy

* **MT7921AU sticks** (ALFA AWUS036AXM, AWUS036AXML): the mainline driver allows
  station, AP and P2P only, no IBSS or 802.11s, so no WiFi mesh.
* **Realtek sticks** (most cheap dongles, all of Berrybase's own WiFi range):
  out-of-tree drivers.
* **Waveshare "SX1262 LoRa HAT"** (Berrybase RPI-868LORA): UART version,
  meshtasticd does not support it. The SPI "LoRaWAN HAT" works but meshtasticd
  marks it deprecated (CRC problems with longer messages).[^meshtasticd-hw]

# Totals per node

| Node | Parts | ≈ € |
|---|---|---|
| House, Pi 5 | Pi 5 4 GB, 27 W supply, SD, case, ACHM | 200 |
| House, Pi 4 | Pi 4 4 GB, 15 W supply, SD, case, ACHM | 180 |
| + long-range radio | RAK6421 + RAK13300 (or Meshstick) | +25 |
| Pocket node | Zero 2 W, SD, RAK6421 + RAK13300, printed case; power bank not included | 65 |

A full order spans three shops (Berrybase, wlan-shop24, Hexaspot).
