---
type: Research Note
title: Scanning for other people on the air with an SDR
description: What a cheap RTL-SDR on a house Pi can detect (PMR446, ham, CB, 868 MHz LoRa, towers) and the legal line between activity and content.
tags: [sdr, scanner, pmr446, ham, legal]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
---

# What it can see

* An RTL-SDR v4 (~€35) tunes roughly 0.5 MHz–1.7 GHz (HF via direct sampling).
* Targets: PMR446 (446.0–446.2 MHz), freenet, 2 m / 70 cm amateur, CB 27 MHz,
  airband, marine, 868 MHz LoRa of any system, cell and broadcast towers coming back.
* WiFi (2.4/5 GHz) is out of range: the hotspot dongle's WiFi scan covers it.
* Detection: repeated sweeps (`rtl_power`-style), flag bursts that start and stop
  like speech, label by band plan and bandwidth. Modulation recognition later.

# Legal

Listening to content not meant for you is restricted in several countries,
Germany included. batnet reports activity only: [decision 0009](/decisions/0009-scanner-activity-only.md).
Check local rules per country before anyone enables more.

# Open questions

* Can the SX1262 do channel-activity detection on other frequencies while meshtasticd owns it? (Assumed no.)
