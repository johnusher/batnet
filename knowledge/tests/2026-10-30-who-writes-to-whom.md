---
type: Test Report
title: "2026-10-30: 48 hours of who writes to whom around the roof hub"
description: A packet trace on the roof hub, 28 to 30 October - 12,373 packets read whole, 11 % for one radio; 592 pairs of radios wrote to each other, 120 both ways, most pairs only once; four pairs around one home base station and two bots carried a fifth of it, by script. Private messages came at every hour, a little more in the early afternoon. Contents unreadable; pairs counted, never named.
tags: [field, roof, meshtastic, privacy, metadata, berlin]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-31T06:50:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), MediumFast, from 2026-10-28 07:26
to 2026-10-30 07:26: meshtasticd's `TraceFile` wrote every packet it read
whole with its bare header (sender, recipient, packet ID, hops, channel hash,
signal), and the type when it could read the contents. Packets it heard
garbled are not in it; neither are the hours it was down (two reboots and a
restart on 29 October, [B79](/known-problems.md)). Out of interest in how much
the headers show ([what Meshtastic hides](/research/meshtastic-security.md)).
The trace names real radios: it stays in `private/`, deleted on the hub;
here only counts.

# What it shows

| | |
|---|---|
| Packets read whole (distinct) | 12,373 |
| To everyone | 10,951 (89 %) |
| To one radio | 1,422 (11 %) |
| of which private, contents unreadable (messages and their receipts) | 869 |
| of which name cards, places and route checks asked of one radio | 553 |
| Pairs of radios that wrote to each other | 592 (120 both ways) |
| Senders / recipients | 142 / 269 |

* **Most pairs wrote once.** 418 of 712 sender-recipient pairs carried a
  single packet; 7 carried ten or more.
* **A few machines carry much of it.** The busiest four pairs, all around
  one home base station (its owner's own radios and two community bots),
  carried 285 packets, a fifth of everything to one radio, at set minutes
  past the hour: a script checking that radios answer. The top ten pairs
  carried 22 %.
* **Every hour of the day.** Private packets came at 19 to 62 an hour,
  most from 13:00 to 14:00, fewest at 05:00 and 23:00.
* **From near and far.** By the fewest relays a copy took: 54 straight,
  362 through one, 303 through two, 269 through three, 426 through four to
  seven.
* **Fewer than the log suggested.** The radio log's garbled headers had put
  the share for one radio at about 30 %; packets read whole put it at 11 %:
  a packet for one radio is often heard only garbled, or many times, as its
  receipts and repeats go back and forth.

# Conclusions

* The bare headers show who writes to whom, how often and when, for anyone
  with a radio on a roof: 592 pairs in two days around one hub. The contents
  stay unreadable. A hub keeps this to itself; batnet's tools count it and
  name no one in anything shared.
* What looks like a lively conversation is often a machine: receipts make
  every message two-way, and scripts that check radios make the busiest
  pairs.
