---
type: Test Report
title: "2026-11-10: a second week on the roof"
description: Fourth weekly snapshot, 143 hours. Eleven and a half days with no reboot; the guard now restarts meshtasticd every 8 to 10 hours, as traffic rose. The strongest relay that went silent on 3 November came back on 6 November and packets read a day rose with it, to about 4,350 on Sunday. Two relays the summary called 5 dB weaker had only gone back to their usual level after a strong week; one distant relay has grown weaker for four weeks running. Public messages fell to about 20 a day, two thirds of them from bots.
tags: [field, roof, statistics, signal, meshtasticd, zero-2-w]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-11-10T09:45:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), MediumFast, unchanged since
29 October. `tools/meshstats` (skill `weekly-mesh-stats`) from 4 November
11:13 to 10 November 10:29, and `talk.py`. Counts only; the radios stay in
`private/`. The week before: [a week on the roof](/tests/2026-11-04-a-week-on-the-roof.md).

# Over the weeks

| Snapshot | Radios | Heard straight | Readable | Noise floor | Packets/h | Passed on/h | For one radio | Busy waits/h |
|---|---|---|---|---|---|---|---|---|
| 25 Oct | 161 | 7 | 36 % | -110.2 dBm | 299 | | | |
| 29 Oct | 211 | 13 | 36 % | -110.2 dBm | 491 | | 29 % | 71 |
| 4 Nov | 232 | 10 | 37 % | -110.2 dBm | 432 | 186 | 28 % | 60 |
| 10 Nov | 235 | 14 | 35 % | -110.2 dBm | 402 | 178 | 26 % | 59 |

Busy waits per packet passed on: 0.33, the usual. Copies through the internet
(MQTT) and back onto the air: 8. Packets on channels the hub has no key for:
51 % of the whole ones.

# Day by day

| Day | Packets read | Radios read | Busy waits | Private messages and receipts | Public texts |
|---|---|---|---|---|---|
| 4 Nov (from 11:13) | 1,635 | 122 | 388 | 665 | 26 |
| 5 Nov | 2,709 | 136 | 926 | 1,233 | 31 |
| 6 Nov | 2,923 | 125 | 1,029 | 910 | 17 |
| 7 Nov | 3,478 | 140 | 1,518 | 1,626 | 19 |
| 8 Nov | 4,353 | 136 | 2,101 | 1,642 | 20 |
| 9 Nov | 3,602 | 135 | 1,685 | 1,664 | 19 |
| 10 Nov (to 10:29) | 1,667 | 110 | 746 | 731 | 6 |

# What changed

* **The silent relay came back.** The strongest neighbour, silent from
  3 November, passed on packets again from 6 November, and the packets read a
  day rose from about 2,900 to 4,350 by Sunday. Its signal is the best yet,
  about 0 dB (it was -5 dB the week before). The other relay that went silent
  on 31 October was back before this week began, under a new name, now about
  +4 dB. Radios read a day stayed near 135, not the 155 of late October.
* **"5 dB weaker" was a return to normal.** The summary flagged two busy
  relays as about 5 dB weaker. Over four weeks they read -12 to -14.5 dB,
  -8 to -9 dB last week, -13.5 dB now: last week was the odd one. The summary
  compares with the snapshot before only ([B83](/known-problems.md)).
* **One distant relay grows weaker.** Its straight copies: about -9.5 dB on
  27 October, -11 to -12 dB by 31 October, about -15 dB from 7 November, four
  weekly medians in a row lower (-9, -10.5, -11.5, -15). The others held or
  got stronger and the noise floor did not move: at that radio or on its path,
  not the hub. To watch.
* **Heard straight:** 14 radios, 4 for the first time, two of them close and
  strong (above 0 dB) but rarely heard.
* **Farthest radio heard this week:** about 30 km, through 5 relays.

# What people said

* Public texts fell to 138 in six days (about 20 a day), from 30 to 120 a
  day in late October. Two thirds came from four senders: a quote bot, a
  base station's welcomes, node counts and counting tests, the city's chat
  bot, and one person. The rest: greetings, tests on the test channel, a
  newcomer who found the right settings and was welcomed.
* Channels: the open one 93, the test channel 28, the city's group 17.
* **Stories:** none. No word but greetings and *test* was used by more than
  two people.
* Private messages and receipts: 347 pairs of radios, 21 both ways; the
  busiest pair is a base station with a bot, as before.

# The hub

* No reboot from 29 October 21:18 to at least 10 November 10:35 (11 days
  13 h). 35 °C, no undervoltage or throttling, card 59 % full, the app and
  meshtasticd running.
* The guard restarted meshtasticd 14 times, each at 104 to 105 MB. The gap
  shrank from 11 to 14 hours early in the week to 8 to 10 hours from
  7 November, when traffic rose: the leak ([B79](/known-problems.md)) grows
  with the interrupts, so with what the radio hears.

# Conclusions

* The hub keeps working; the guard holds until meshtasticd 2.8.
* What the hub hears still rides on one or two relays: when the strongest
  came back, a third more packets were read a day.
* A drop flagged against last week needs the weeks before it; one relay's
  slow fall over four weeks is the only trend.
