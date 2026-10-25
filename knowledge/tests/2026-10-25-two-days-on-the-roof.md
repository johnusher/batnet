---
type: Test Report
title: "2026-10-25: two days on the roof"
description: The roof hub (Pi Zero 2 W, antenna on the roof) heard 184 radios on MediumFast in about 37 hours, 81 with a place spread over 35 by 23 km and up to 30 km away; 68 of them lie between west and north-north-west, in a lobe about 27 by 17 km; five radios straight, the farthest 12 km, no stronger than the first hub heard them with the same antenna before it moved.
tags: [field, lora, meshtastic, roof, coverage, berlin, relays]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-25T06:45:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md) (Pi Zero 2 W, Elecrow Meshstick,
the 1 m antenna on top of the house), EU_868 MediumFast, hop limit 3, from
about 18:00 on 23 October to 07:25 on 25 October 2026 (about 37 hours). Off
MediumFast from 02:00 to 08:01 on 24 October: an hour on LongFast, then the
frozen radio stick ([the night before](/tests/2026-10-24-presets-and-reach-from-the-roof.md),
B74). Logging was off, so the figures come from the panel's status (each
radio's last hearing, its relays on the shortest copy (B76), its place and
its straight readings), the traffic view's last hour and
`tools/hubcompare`, never a second meshtasticd client. Names, IDs and places
stay on the hub.

# Results

| What | Two days on the roof |
|---|---|
| Radios heard since the antenna went up | 184 (the first hub's night: 154) |
| With a place | 81, over about 35 km east to west and 23 km north to south |
| Farthest with a place | 30 km to the west-north-west, through 3 relays (a rough area, ±2.9 km): the farthest either hub has heard, against about 25 km before; then 29 km through 7 relays (an exact place), 27, 25 and 24 km, all west-north-west |
| Packets read | 2,752 in the 13 hours since batnetd last restarted, about 210 an hour (the first hub's night: 205) |
| A sample hour (06:25-07:25 on 25 October) | 144 read, 215 too garbled to read, 132 passed on by the hub, 19 times the channel was busy |
| The hub's own airtime | 0.7 % of the hour (`airUtilTx`); the channel busy 2 to 4 % (`chanUtil`) |

Relays on the shortest copy the hub heard of each radio:

| Relays | straight | 1 | 2 | 3 | 4 | 5 | 6 | 7 |
|---|---|---|---|---|---|---|---|---|
| Radios | 5 | 12 | 39 | 53 | 30 | 23 | 14 | 8 |

Where the 81 radios with a place lie, by direction from the house: north-west
50, west 13, north 12, north-east 3, south 2, south-west 1, east and
south-east none. 68 lie between west and north-north-west (bearings 270° to
340°, rounded to 10°).

An ellipse of two standard deviations round them (holding about 86 %): about
27 × 17 km, its long axis at 115°, its middle 9.4 km to the north-west
(313°). The [evening before's](/tests/2026-10-24-presets-and-reach-from-the-roof.md)
from 52 radios: 21.5 × 17 km, its middle 7.3 km to the north-west.

The map's zone: five radios heard straight, of which two count (the others
share no place or one too rough to say which way): one 12 km to the
north-west at −13 dB, so a petal reaching about 13 km (14 km from the rounded
figures in the diagram), about 38 km².

# The roof against before it went up

`tools/hubcompare/compare.py pz7 pz7:/var/lib/batnet/first-hub-antenna-links-2026-10-23.json`
(the first hub's straight readings with the same antenna before it moved, kept
on the roof hub): five radios heard straight by both, the roof a **median
0 dB** stronger (from −2.25 to +1.5 dB, the 12 km one 1.25 dB weaker); three
heard straight only from the roof (one of them the first hub itself, 0.4 km
away); six only before, most of them heard a few times. On 2026-10-23 at
13:50 the same comparison gave four in common and the roof 0.2 dB stronger.

# What it shows

* **The hub hears the city as a lobe to the north-west**, not a circle: that
  is where Berlin's busiest relays and most of its radios are, and the south
  is the chimney's side. The far end of the lobe (25 to 30 km) comes through
  three to six relays.
* **The roof gave a wider audience than one night by the first hub (184
  radios against 154), not a stronger signal**: the radios heard straight
  arrive as strongly as before the antenna went up. The two counts are not
  quite alike either: 37 hours against 8.7, and daytime radios with them.
* **Still almost nothing straight**: five radios, the farthest 12 km. A
  higher mast clear of the chimney is the next thing to try
  ([bench notes](/hardware/bench-pz7.md)).

The diagram `roof-lobe` in `docs/img/radio/make.py` draws the 81 radios
rounded to 10° and 1 km, the ellipse and the zone; the README's *What our hub
heard* shows it.
