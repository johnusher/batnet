---
type: Hardware
title: WiFi radios - built-in, USB dongles, point-to-point links
description: Which radio carries the mesh, which carries the hotspot, and outdoor point-to-point units for links beyond a few hundred metres.
tags: [wifi, dongle, cpe, mesh, hotspot]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
---

# Plan

* House Pi: built-in WiFi for one job, a USB dongle for the other (mesh vs hotspot; which way round is a bench test).
* Dongle candidates: chipsets with good mainline support for IBSS/802.11s/AP (ath9k_htc, mt76). **To test**: [WiFi mesh modes](/research/wifi-mesh-modes.md).
* Within mt76 the chip matters: MT7610U (ALFA AWUS036ACHM) and MT7612U (AWUS036ACM) allow AP, IBSS and mesh; **MT7921AU (AWUS036AXM/AXML) does not** - its mainline driver offers station, AP and P2P only (checked in the kernel source, 2026-10-15). USB mt76x0u/x2u allow at most two interfaces on one channel: fine with built-in WiFi on the hotspot and the dongle on the mesh. Prices: [EU shopping list](/hardware/shopping-list-eu.md).
* Outdoor point-to-point pairs (e.g. TP-Link CPE210, ~€40 each) link houses 1–3 km apart over Ethernet; batman-adv runs over the Ethernet interface, no batnet code needed.
