---
type: Research Note
title: What the village simulator shows
description: Findings from the in-browser village model (prototypes/village-sim) - LoRa duty limits at busy gateways, WiFi groups with no LoRa radio, and recovery traffic.
tags: [simulation, lora, capacity, sync, prototype]
status: draft
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T13:45:00Z }
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

**Resolved (2026-10-15, [decision 0021](/decisions/0021-one-hop-heartbeats.md)):**
heartbeats now go one hop. Heartbeats alone use 1% of the channel at 10, 20
and 40 houses and 2% at 80; bang passes with median delivery 10 ms, worst 48 s,
nothing lost, with repair paying for its airtime.

# Real code, modelled WiFi and LoRa (2026-10-15, plan 3)

The LoRa bridge (`internal/lora`) now runs in the simulator: priority queues,
the 10% duty limit, fragments, gateways sharing the sending by suppression,
beacons with recovery, and Meshtastic's managed flooding by every radio.

| Scenario | Result |
|---|---|
| village (12 houses, 3 hamlets 1-2 km apart, 4 gateways, 2 Meshtastic users, a 10 min power cut) | pass: median delivery 3.5 s, slowest 5% 59 s; 368 LoRa packets, no collisions, busiest radio 40% of its hourly limit |
| farms (8 LoRa-only farms up to 5 km apart) | **known failure**: 2,133 collisions at receivers (hidden terminals), 1 of 50 posts missing, 1 late (27 min) |
| bang, storm, street (WiFi only) | pass, unchanged |

What the scenarios forced, each with a test:

* **Catch-up after an outage** was too slow (8 posts per 15 min beacon): a
  gateway now beacons again 2 min after receiving recovered posts, and within a
  minute of a restart.
* **Radios waiting for a busy channel** started together and collided: a 0-2 s
  random backoff; rebroadcasts now wait for the channel instead of being dropped;
  rebroadcast delay depends on distance (far radios first, as Meshtastic's
  contention window), so near radios usually stand down.
* Compared with the JavaScript model's 12-house village (81% within 10 min),
  the real code's village delivers everything within 15 min with fewer
  gateways' airtime; the two models differ in layout and traffic, so this is a
  sanity check, not a like-for-like comparison.
* A signed entry is about 100 bytes before its text (32-byte origin, 64-byte
  signature): a 100-character post takes two LoRa packets, about 2.5 s of
  LongFast airtime - double the JavaScript model's estimate.

# Review of 2026-10-15: what loses posts over LoRa, and the fix

Run with the real code over seven seeds of each LoRa scenario (the CI seed
and seeds 1-6; `batnetd -sim` on copies of the file with another seed).

**Where the lates came from.** Tracing one late post in the village (seed 7)
showed a two-fragment post whose first fragment collided at every receiver;
the receivers held the second fragment for ten minutes and then dropped it,
and the post came back only with the next quarter-hour beacon. On the
LoRa-only farms (hidden terminals) this happened to a post every few minutes.

**Three changes, measured one at a time:**

| Change | farms, 6 seeds (missing / late) | festival, 7 seeds | village, 7 seeds |
|---|---|---|---|
| before | 2 / 26 | – | 36 missing (seed 1), 24 late |
| Meshtastic's 3-hop limit on rebroadcasts (B15) | 3 / 23 | – | same |
| + a partial message nudges the beacon ([0024](/decisions/0024-ask-soon-for-what-was-lost.md), first half) | 0 / 3 | 6 / 10 | same |
| + Meshtastic's contention window in the model (below) | 0 / 1 | 6 / 5 | 36 missing, 10 late |
| + a beacon ahead of my log nudges my beacon (0024, second half) | 0 / 1 | 0 / 0 | 0 missing, 6 late (all in seed 2) |

**The contention window.** The model's rebroadcast delay was a fixed 0.2-2.2 s
by distance plus 0.3 s of jitter, which put six pocket nodes within 300 ms of
each other and made them collide. Meshtastic's firmware waits a random number
of slots (about 70 ms each on LongFast) from a window of 4 slots for a weak
signal to 128 for a strong one, so the farthest radio usually goes first and
the near ones stand down. The model does that now, with distance standing in
for SNR (L13). It cut the festival's collisions from 2,040 to 695 per 45 min.

**The delivered check counts connected time.** A post's clock used to run
from the first moment a house was connected to a holder, through a holder's
power cut. It now counts only time connected (the holders dark for the rest
of the run still count on the clock, so a sender that goes dark can't hide a
lost post). That is what spec §13 means by "messages wait and arrive on
return"; it took the village's power-cut lates from 7 to 0 on the CI seed.

**Festival** (new scenario): six Pi Zero 2 W pocket nodes on a field 600 m
across, 33 people, 4 posts per person per hour, a lost-child High post and a
burst as the headline act ends. All by LoRa, every radio hearing every other.
Passes on seven seeds: median delivery 7 s, busiest radio 55% of its hourly
limit, 955 collisions at receivers in 45 min (the close radios still collide
inside the carrier-sense time).

**What remains** (B32, B33): village seed 2 after its gateway's 10 min power
cut (6 late: the backlog, catch-up beacons every 2 min and the others'
recoveries share one busy channel), and a post whose every frame was lost at
a receiver that hears no beacon afterwards.
