---
type: Decision
title: "0012: Pocket node for portable use (draft)"
description: A Pi Zero 2 W with a LoRa HAT and a power bank serves a few phones on its hotspot and links to other nodes over LoRa, for festivals, camps and hikes.
tags: [portable, pi-zero, festival, hardware]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:33:29Z }
---

# Context

The user wants batnet to work away from home too: festivals (Burning Man,
Glastonbury), camps, hikes. A house Pi (Pi 4/5 with two WiFi radios, decision
[0003](/decisions/0003-house-node-browser-app.md)) needs mains or a big battery
and a case of its own. People should be able to carry a node.

# Decision (proposed)

A **pocket node** is the `house` role on a Pi Zero 2 W:

* its single built-in WiFi radio runs the hotspot only (no WiFi mesh);
* an SX1262 HAT links it to other nodes over LoRa, so between pocket nodes
  everything is text;
* a USB power bank powers it; a 3D-printed case holds Pi, HAT and antenna.

It serves a handful of phones: the [village simulator](/research/village-sim-findings.md)
assumes about 8 for a Zero 2 W. Speech-to-text on it is unmeasured: Moonshine Tiny
(about 35–40 MB) may fit, slower than real time, which is acceptable for
store-and-forward voice messages ([speech pipeline](/research/speech-pipeline.md)).

# Open

* Battery life on a typical 10,000 mAh bank, measured, not guessed.
* Whether a USB WiFi dongle on the Zero's OTG port can add the WiFi mesh when
  two camps are close.
* Busy LoRa airtime when many camps share one channel; the stress scenario
  shows a single gateway hitting the 10% limit.
