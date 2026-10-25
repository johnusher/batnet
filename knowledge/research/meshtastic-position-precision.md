---
type: Research Note
title: How Meshtastic rounds a shared place - the grid behind the map's rough areas
description: A radio sharing a rough place sends the middle of a square of a fixed latitude-longitude grid (precision bits), so radios in one square share exact coordinates and whole rows share a latitude; the apps' "approximate" is 13 bits (5.8 by 3.6 km in Berlin), firmware 2.8 makes sharing opt-in and caps public channels at 15 bits; what it means for the map, for distances and for guessing where radios are.
tags: [meshtastic, position, privacy, map, guesses]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-25T13:30:00Z }
stale_after: 2027-04-25
sources:
  - id: fw-prec
    resource: https://github.com/meshtastic/firmware/blob/develop/src/mesh/PositionPrecision.cpp
    title: Meshtastic firmware - PositionPrecision.cpp and .h (the public-channel cap, 15)
  - id: fw-optin
    resource: https://github.com/meshtastic/firmware/pull/10929
    title: Meshtastic firmware - make position and telemetry broadcast opt-in (2.8)
  - id: android-prec
    resource: https://github.com/meshtastic/Meshtastic-Android/blob/main/core/ui/src/commonMain/kotlin/org/meshtastic/core/ui/component/PositionPrecisionPreference.kt
    title: Meshtastic Android - the position precision setting (10 to 19 bits, default 13)
  - id: mt-chan
    resource: https://meshtastic.org/docs/configuration/radio/channels/
    title: Meshtastic docs - Channel configuration, position precision
---

# The grid

Meshtastic sends a place as whole numbers of 10⁻⁷ degrees. A rough place
keeps only the top *bits* of each and sends the middle of what was dropped
[[mt-chan]]: a square of a fixed grid, the same height and width **in
degrees**. In Berlin a degree of longitude is about 0.61 of one of latitude,
so the squares are taller than wide:

| Bits | Square in Berlin | Panel says "within about" |
|---|---|---|
| 10 | 46.7 × 28.5 km | 23 km |
| 12 | 11.7 × 7.1 km | 5.8 km |
| 13 | **5.8 × 3.6 km** | 2.9 km |
| 15 | 1.5 × 0.9 km | 730 m |
| 16 | 730 × 445 m | 365 m |

Every radio in one square sends the **same** coordinates, every radio in one
row the same latitude, every one in a column the same longitude. On the map
this looks like a resonance: dots lined up on exact rows and columns, and
several dots on one point.

# What sets the bits

- The Android app's precision slider runs from 10 to 19 bits and starts at
  **13** when the user picks an approximate place [[android-prec]]: the
  common choice.
- Firmware 2.8 (pre-releases from August 2026) makes sharing a place
  **opt-in**: precision 0 on public channels until the owner turns it on, a
  one-time migration turns it off on older radios [[fw-optin]]. It also never
  sends more than **15 bits** on a channel anyone can decrypt (the default key
  or a well-known one) [[fw-prec]]. 2.7.26, the last stable release and the
  roof hub's, has neither: it sends what it is set to.
- So on a public channel, a place finer than 15 bits comes from firmware
  before 2.8; exactly 15 may be a 2.8 radio capped, or a choice.

# What the roof hub saw (25 October 2026)

Of 82 radios with a place, **40 used 13 bits**, 12 used 15, 8 used 16 (the hub
itself among them), 12 used 17 to 19, one 10 and one 14, and 8 sent an exact
place. The 13-bit radios sat on four rows; one row, the hub's own, held 13
radios, and one square in the city centre held 8 radios on a single point.
One 13-bit radio shares the hub's own square (so it is at most a few km away)
yet reached the hub only through six relays.

# What it means for batnet

- **Draw the square, not a circle.** The panel drew a circle as wide as the
  square is tall; it now draws the square, and one dot (with *+n*) for radios
  sharing a point.
- **Distance and direction to a rough radio** are a range: a 13-bit radio
  next door can be 0 to 5 km away, in a span of directions. Bearings in
  antenna studies must use the square's corners, or skip the radio.
- **Guessing where radios are** (the panel's owner-only guesses) must spread
  a rough known radio over its whole square, not put it at the middle: with
  the middles, the test was flattered by radios sharing a point with the one
  hidden (the light patch held 74% of closely placed radios, not the 83% the
  whole set suggested); with the squares it holds 84%
  ([test](/tests/2026-10-25-guessing-where-radios-are.md)).
- **A rough place is a choice** (decision [0019](/decisions/0019-location.md)
  is batnet's own): the guesses stay with the hub's owner, behind the hub PIN.
