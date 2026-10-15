---
type: Decision
title: "0019: Location: off by default, per message, from the phone or the node's GPS"
description: Location is attached per message (asked, pre-ticked, on High), from the phone's GPS or a GPS dongle on the node; tracking every 2 min over WiFi but only the latest position every 15 min over LoRa; zones later.
tags: [location, gps, privacy, lora, time]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:58:54Z }
verified: { by: human:johnusher, at: 2026-10-15T06:58:54Z }
---

# Decision

* **Off by default.** A 📍 button in the message box attaches a location to one
  message. High messages ask "Add your location?", pre-ticked. Precision: exact
  or approximate (~200 m).
* **Source, chosen in Settings:** *this phone* (browser Geolocation, works offline;
  first fix can take a minute) or *the node's GPS* (a USB GPS dongle on the Pi:
  for a fixed house that is the house's position; for a pocket node, the
  carrier's).
* **A GPS on any one node sets the village's time** (gpsd), which fixes the
  wrong-clock problem at its source.
* **Shown as** "📍 340 m north-east of you" with coordinates and a simple map of
  the houses; offline street maps later.
* **Over LoRa** a position costs ~8 bytes in a message.
* **Tracking (M2):** "Share my location" sends a position every 2 min over WiFi.
  Over LoRa only the latest position goes, every 15 min or after moving 200 m:
  every 2 min would let 17 people use up a radio's whole hour (30-byte packet,
  0.68 s at LongFast). A phone tracks only while batnet is open; a pocket node
  with a GPS dongle tracks reliably.
* **Zones (later, to do):** "message Ben when Anna enters or leaves this area",
  checked on the house Pi from location updates; the tracked person must agree.
