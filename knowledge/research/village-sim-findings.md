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

# Added 2026-10-15

* **Priority** ([0015](/decisions/0015-message-priority.md)): stress scenario,
  6 h, 5% of messages High and 12.5% Low: slowest 5% of LoRa deliveries 21 min
  for High against 131 min for Normal. In quiet villages every level arrives in
  about 2 s; priority cannot create airtime.
* **Meshtastic users** now hear every packet in range: they read public posts
  and pass batnet's encrypted packets on. In the 12-house village over 6 h each
  heard about 40 public posts and relayed about 680 batnet packets.
* About one message in six is a voice message (M2 preview); over LoRa it costs
  the same as its transcript.

# Real code, modelled WiFi (2026-10-15)

`batnetd -sim` runs batnet's own log and sync code for every house in virtual
time over a Go model of the WiFi mesh (ranges per Pi and radio, batman-adv
flooding, one channel per WiFi group at the 1 Mbit/s broadcast rate, loss
growing with load). No LoRa yet.

| Scenario | Result |
|---|---|
| street (3 houses, a power cut, a small burst) | pass; median delivery 2 ms |
| storm (8 houses in a line, three power cuts) | pass; posts sent during a split arrive soon after it heals |
| bang (40 houses, everyone near the school posts within 30 s) | **fails** once repair pays for airtime: median delivery 41 s, worst 4.7 min, 45 posts never arrive. With 30 s heartbeats instead of 5 s it passes (median 70 ms). An earlier run that passed modelled repair as free (review, 2026-10-15) |

**Finding: flooded heartbeats do not scale, and they break the bang scenario.** With no messages at all, the 5 s
heartbeats (flooded to every node by batman-adv) keep the channel 5% busy at
10 houses, 10% at 20, 41% at 40 and 100% at 80 (quiet scenarios, 5 min). Spec
section 5 needs heartbeats that stay one hop (neighbours compare and repair
hop by hop), or a longer interval that grows with the mesh. To decide.
