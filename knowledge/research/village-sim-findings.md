---
type: Research Note
title: What the village simulator shows
description: Findings from the in-browser village model (prototypes/village-sim) - LoRa duty limits at busy gateways, WiFi groups with no LoRa radio, and recovery traffic.
tags: [simulation, lora, capacity, sync, prototype]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T19:05:00Z }
stale_after: 2027-04-14T00:00:00Z
---

# What it is

`prototypes/village-sim/` is a discrete-event model of a village (houses, relays,
Meshtastic users, WiFi links, one shared LoRa channel) with the web app mocked
beside it. `sim.js` has no DOM and runs in node. Its assumptions are listed in the
page and come from [LoRa airtime](/research/lora-airtime-eu868.md) and
[radio links compared](/research/radio-links-compared.md). Model output, not measurement.

# Findings (6 simulated hours, seed 7)

| Scenario | Delivered within 10 min | LoRa median / slowest 5% | Notes |
|---|---|---|---|
| Village · 12 houses, 46 people, 3 msg/person/h | 81% | 1.9 s / 2.7 s | the misses are a WiFi group with **no LoRa radio**: nothing reaches it from outside |
| Street · 6 houses, all WiFi | 100% | – | LoRa idle |
| Farms · 8 houses, LoRa only | 98% | 1.9 s / 89 s | many re-sends after losses at 3 km |
| Stress · 40 houses, 168 people, 8 msg/person/h | 78% | 3.2 s / ~2 h | one gateway hits its **360 s/hour** limit; ~1,600 messages queue behind it while the channel is only 16% busy |
| Storm · power cuts | 80% | 1.9 s / 3.2 s | catch-up after outages works; offline houses dominate the backlog |

# Design consequences to carry into the spec

* **Per-radio duty cycle, not channel capacity, is the first LoRa limit** for a busy
  WiFi group. Options: more than one gateway per WiFi group sharing the outbound
  load, priority for DMs over everyone-channel posts, a faster preset for the private channel ([0008](/decisions/0008-longfast-single-radio.md)).
* **Every WiFi group needs a LoRa radio**, or the app must say plainly which houses
  are cut off. The installer should warn when a group has none.
* The model's queue had a head-of-line bug (one blocked radio stalled all others);
  batnetd's LoRa scheduler must keep a queue per radio.
