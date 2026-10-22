---
type: Test Report
title: "2026-10-22: one night on MediumFast"
description: The bench hub logged Berlin's MediumFast mesh for 8.7 hours overnight - 1,788 packets read from 154 radios, almost all through relays (most through 2 to 4, up to 7), more heard than could be read, three quarters of senders allowing 7 hops; the hub passed on 1,482 packets itself.
tags: [field, lora, meshtastic, presets, berlin, relays, log]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-22T07:40:00Z }
---

# Setup

The [bench hub](/hardware/bench-pi7.md) at home in Berlin, EU_868 MediumFast,
hop limit 3, role client, the antenna near the roof as in the
[evening's test](/tests/2026-10-21-presets-and-antenna.md). The hub's log ran
from 22:32 on 21 October to 07:14 on 22 October (8.7 hours), with a hello
every hour and the power mode on Medium. Counts below leave out the hub's own
packets. Node IDs and positions stay in the log on the hub, never here.

# Results

| What | Night |
|---|---|
| Packets read from other radios | 1,788 (about 205 an hour, steady through the night) |
| Different radios read | 154 |
| Heard but unreadable (failed checksum) | 2,788 |
| On channels the hub has no key for | 656 of the 1,788 (37 %) |
| Readable ones, by kind | position 714, telemetry 249, name (nodeinfo) 166, text 2, admin 1 |
| Straight from the sender, no relay | 28 packets, from 5 radios |
| Median SNR of those read | −11 dB |
| Packets the hub passed on itself | 1,482 |
| The hub's own share of airtime | 1.1 % of the hour (the radio's `airUtilTx`); the channel busy 11 % (`chanUtil`) |
| The Pi | 35–41 °C, load under 0.6, at least 1.49 GB of memory free |

How many radios passed each readable packet on before the hub heard it:

| Relays | direct | 1 | 2 | 3 | 4 | 5 | 6 | 7 |
|---|---|---|---|---|---|---|---|---|
| Packets | 28 | 259 | 428 | 460 | 264 | 192 | 97 | 49 |

The hop limit senders set (`hopStart`): 7 for 77 % of packets, 3 for 15 %,
5 for 5 %, the rest 1, 2, 4 or 6. Of the packets with a known last relay,
the busiest relay passed on 36 %, the next three 17–20 % each.

The 68 radios that shared a position spread over about 29 km north to south
and 20 km east to west: most of the city.

Of the unreadable packets, 23 % arrived at −10 dB SNR or better, strong
enough to read: they collided with another packet. Half were below −12.5 dB,
too weak.

# What it shows

* **The hub hears most of Berlin, but almost nothing directly.** Five radios
  reach it straight; everything else comes through Berlin's relays, most
  through two to four, some through seven. A hub's view of the city rests on
  a few rooftop and tower radios it does not control.
* **Berlin runs on 7 hops.** Meshtastic's default is 3; three quarters of the
  senders here allow 7, so their packets cross the city. A batnet village in
  Berlin should expect its public posts to travel that far too.
* **More arrives than can be read.** 2,788 failed against 1,788 read: the air
  is crowded (a quarter of the failures were strong collisions) and much is
  at the edge of MediumFast's reach. A better antenna position would turn
  some of the weak half into readable packets.
* **The hub carries its share.** As a client it passed on 1,482 packets in
  the night while using 1.1 % of the hour, well inside the 10 % limit.
* **The public channel is quiet at night.** Two text messages in 8.7 hours;
  the rest is radios saying where they are, how their battery is and who
  they are.

# Not tested yet

* The same on LongFast over a whole night, for a fair count of both.
* Daytime, and a weekend.
* The antenna outside on a cable ([to do](/todo.md)).
