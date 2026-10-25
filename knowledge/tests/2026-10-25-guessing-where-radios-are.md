---
type: Test Report
title: "2026-10-25: guessing where radios are, from how they reach the roof hub"
description: Leave-one-out test of the panel's owner-only guesses on the roof hub's radios whose place is known closely - the light patch held the hidden radio about four times in five, the relays narrow it only a little (about 400 to 300 km²), a shared rough area a lot (a 10-bit area of 1,320 km² to about 110 km²); one hub and a few hours mostly say "in the lobe to the west and north".
tags: [guesses, map, relays, meshtastic, roof, privacy]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-25T13:40:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), EU_868 MediumFast. Every packet
heard (readable or not) from 13:06 to 15:04 on 25 October was read off the
panel's traffic view once a minute from a laptop (never a second meshtasticd
client): its sender, the last relay (one byte) and the relays it passed.
That is what the hub now keeps itself (`sightings.json`). The radios' places
come from the panel's status. Names, IDs and places stay off the repo.

The guess (`internal/hub/panel/guess.js`): over a grid of 500 m squares,
each radio whose place is known within 3 km is smeared out (1.5 km, over
its whole square if its place is rough: see
[how Meshtastic rounds a place](/research/meshtastic-position-precision.md)),
weighted by how alike it reaches the hub: the share of last relays in
common, and the fewest relays (weight falling over about 2). A tenth is
spread evenly. A shared rough area cuts the guess to its square.

The test: each of the 19 radios placed to within 400 m was hidden in turn
and guessed from the others. *Light patch*: the smallest area holding four
fifths of the guess; *dark*: half. The baseline ignores the relays (every
known radio weighted the same): where radios that reach this hub usually are.

# Results

| Guess | Light patch held it | Light patch, median | Dark held it |
|---|---|---|---|
| Baseline, no relays | 18 of 19 | 405 km² | 12 of 19 |
| **From the relays** | **16 of 19** | **299 km²** | 11 of 19 |
| Baseline, inside a 10-bit area (1,320 km²) | 17 of 19 | 130 km² | 13 of 19 |
| **From the relays, inside a 10-bit area** | **17 of 19** | **108 km²** | 10 of 19 |
| From the relays, inside a 12-bit area (82 km²) | 16 of 19 | 54 km² | 13 of 19 |

- The patches are honest: the light one held the radio about four times in
  five, as it says.
- The relays narrow the guess by about a quarter. From one hub, the last
  relay says little about which way: three neighbours bring almost
  everything, and each hears a wide area behind it. Most radios with no place
  come out as "probably 4 to 22 km away, between west and north": the lobe
  the hub hears ([two days on the roof](/tests/2026-10-25-two-days-on-the-roof.md)).
- A shared rough area helps most: a 10-bit area shrinks about twelvefold, to
  where radios that reach the hub are within it.
- A first version put each rough known radio at the middle of its square;
  radios sharing that point with the hidden one flattered it (all 46 radios
  within 3 km: 83%; the 19 placed closely: 74%). Smearing over the square
  fixed it.

# The signal, as an option

For radios heard straight, how strongly they reach the hub says roughly how
far they are (the map zone's fading, 30 dB for each tenfold distance), learned
from the straight radios whose place is known. At 17:40 there were three
(1.2, 2.1 and 11.6 km, the farthest the strongest): height matters as much as
distance on the roof, and hidden in turn the guess with the signal held 2 of
3 against 3 of 3 without. So it is a tick box, off at first, that says how
many radios it learned from and how wide its distances are.

# What would make it better

More days of sightings (a radio heard through several relays narrows faster),
a second hub (two views to cross), and traceroutes or neighbour lists heard
on the air, which name direct links; the roof hub heard none in these two
hours. Added to [ideas](/ideas.md) as the radio's way, for the owner only.
