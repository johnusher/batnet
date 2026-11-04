---
type: Test Report
title: "2026-11-04: a week on the roof, day by day"
description: Three weekly snapshots and 134 more hours of the roof hub's log. The hub ran five and a half days with no reboot, the memory guard restarting meshtasticd 13 times. Packets read fell from 4,616 a day to about 3,400, and radios a day from about 155 to 130, from the day after a strong relay went silent; a second, the strongest, went silent three days later. Its signal had swung by 9 dB while the others held, so that was at its end, not the hub's. A new farthest radio, about 36 km.
tags: [field, roof, statistics, signal, meshtasticd, zero-2-w]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-04T10:40:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), MediumFast, unchanged since the
[memory guard](/tests/2026-10-29-roof-hub-reboots-and-signal.md) went in on
29 October. From `tools/meshstats` (skill `weekly-mesh-stats`) and the
snapshots' journals, split by day. Counts only; the radios stay in `private/`.

# Over the weeks

| Snapshot | Radios | Heard straight | Readable | Noise floor | Packets/h | Passed on/h | For one radio | Busy waits/h |
|---|---|---|---|---|---|---|---|---|
| 25 Oct | 161 | 7 | 36 % | -110.2 dBm | 299 | | | |
| 29 Oct | 211 | 13 | 36 % | -110.2 dBm | 491 | | 29 % | 71 |
| 4 Nov | 232 | 10 | 37 % | -110.2 dBm | 432 | 186 | 28 % | 60 |

# Day by day

| Day | Packets read | Unreadable | Radios read | Passed on | Busy waits | Public texts |
|---|---|---|---|---|---|---|
| 27 Oct | 3,816 | 7,350 | 158 | 4,652 | 1,663 | 38 |
| 28 Oct | 4,378 | 7,043 | 150 | 5,100 | 1,553 | 76 |
| 29 Oct | 4,466 | 7,955 | 163 | 5,360 | 1,838 | 111 |
| 30 Oct | 4,616 | 7,785 | 156 | 5,325 | 1,541 | 123 |
| 31 Oct | 4,024 | 6,821 | 153 | 4,483 | 1,505 | 91 |
| 1 Nov | 3,627 | 6,270 | 130 | 4,363 | 1,427 | 29 |
| 2 Nov | 3,237 | 5,577 | 128 | 3,887 | 1,309 | 51 |
| 3 Nov | 3,458 | 5,873 | 132 | 4,146 | 1,300 | 32 |
| 4 Nov (to 11:13) | 1,787 | 3,216 | 119 | 2,113 | 668 | 8 |

26 October is missing: the hub's journal was short then
([B78](/known-problems.md)). Packets by the hour of day: about 40 % more from
22:00 to 06:00 than from 11:00 to 20:00.

# What changed

* **Two relays went silent.** Both stopped sending their own packets and
  passing on others', so the two signs agree. One, about 11 % of the copies
  the snapshot before, stopped on the evening of 31 October. The radios and packets read
  a day fell from the next day. That fits, but a week does not prove it, and
  1 November was a Sunday. The other, the strongest neighbour (22 %), stopped on the
  morning of 3 November; 4 November so far looks no different.
* **The strongest neighbour's signal had moved by itself.** Its median went
  from -1.5 dB to about -10 dB on 29 October, stayed there for three days, came
  back to about -3 dB on 2 November, with some hours off the air on 30 October.
  The summary's "5.8 dB weaker" was a figure between those. Most other relays
  held within about 2 dB or got a little stronger, one swung by about 4 dB,
  and the noise floor did not move: the change was at that radio, not the hub.
* **Farthest radio heard:** about 36 km (its place known to about 3 km),
  through 7 relays; before, about 30 km.
* **Stories:** none. No word was used by more than 3 people on one day.

# The hub

* No reboot from 29 October 21:18 to at least 4 November 11:18 (5 days 14 h),
  against three in the three days before. No undervoltage, 36 °C.
* The guard restarted meshtasticd 13 times, about every 11 to 12 hours lately,
  each at 104 to 105 MB: the leak ([B79](/known-problems.md)) runs at about
  9 MB an hour at this traffic. Some 15 s off the air each time.

# Conclusions

* The guard does its job until meshtasticd 2.8.
* The hub's reach depends on a few neighbours it does not control. Two of its
  three busiest relays stopped within a week and it kept working, with
  fewer radios heard.
* The summary still listed the two silent radios as heard straight and did not
  say they had stopped ([B82](/known-problems.md)); the daily split showed it.
