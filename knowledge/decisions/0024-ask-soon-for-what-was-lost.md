---
type: Decision
title: "0024: Ask soon for what was lost on air"
description: A gateway that sees it is missing something over LoRa (part of a message never completed, or another gateway's beacon shows entries it lacks) beacons within two minutes instead of at its next quarter-hour; made the LoRa-only farms and a festival of pocket nodes pass in the simulator. Refines 0022.
tags: [lora, gateway, beacon, recovery, collisions]
status: stable
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T13:40:00Z }
---

# Context

Over LoRa a post is sent once (plus Meshtastic's rebroadcasts). When its frames
collide at a receiving gateway, that gateway gets the post back only through
recovery: its own beacon, every 15 minutes, answered by a gateway that holds
it. The LoRa-only farms scenario (8 farms up to 5 km apart, hidden terminals)
lost or delayed posts this way on every seed: with a 3-hop rebroadcast limit
in the model, 3 of 320 posts missing and 23 late against a 20 min limit over
six seeds.

# Decision

A gateway's next beacon comes within **2 minutes** (the same catch-up wait it
already uses after receiving recovered posts) when either:

* it holds **part of a message** that has not completed after **60 s**: the
  rest was lost on air. Each partial message nudges once; it is dropped after
  the 10 min reassembly time as before.
* **another gateway's beacon** shows a version vector ahead of its own for any
  origin: that gateway holds entries this one lacks.

Beacons stay 15 minutes apart otherwise; after a beacon the next is 15 min
away again, so a nudge never produces more than one beacon per 2 minutes.

# Evidence (simulator, model numbers)

| Scenario | Before | After |
|---|---|---|
| farms, seed 13 (CI) | 0 missing, 4 late | 0 missing, 0 late |
| farms, seeds 1–6 | 3 missing, 23 late | 0 missing, 1 late |
| festival (6 pocket nodes, LoRa only), seed 21 and 1–6 | 6 missing, 10 late | 0 missing, 0 late |
| village, seed 1 | 36 missing | 0 missing, 0 late |
| village, seeds 2–7 | 24 late | 6 late (seed 2), 0 elsewhere |

The farms scenario's known-failure mark is removed. Tests:
`TestStalledPartialPullsTheBeaconForward` (lora), the scenario files in CI.

Measured in the same session as two model changes that also moved these
numbers: Meshtastic's hop limit of 3 on rebroadcasts (B15) and its contention
window for rebroadcasts (a random number of slots, more for a strong signal)
in place of a fixed delay; and the delivered check now counts only time
connected to a holder ([findings](/research/village-sim-findings.md)).
