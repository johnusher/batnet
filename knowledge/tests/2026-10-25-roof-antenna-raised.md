---
type: Test Report
title: "2026-10-25: the roof hub's antenna raised above the chimneys"
description: The roof hub's antenna went up to clear a chimney to the north and one to the south (about 14:15 on 25 October). The same daytime hours before and after - a third more packets read, half again as many weak copies heard, the noise floor unchanged; the strongest neighbour 9 dB stronger, a hilltop router 20 km west 6 dB, three neighbours carrying copies that were not before; the radio 3 km south-south-west that only came through relays now heard straight; the north no better.
tags: [field, lora, meshtastic, roof, antenna, coverage, berlin]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-25T16:40:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), MediumFast, hop limit 3, the
same 1 m antenna. The owner raised it between about 14:13 and 14:25 on
25 October 2026 to clear two chimneys of about the same height, one north and
one south of it; the south had been its blind side.

The figures come from meshtasticd's journal on the hub's card (kept since the
morning, [B78](/known-problems.md)): every copy too weak to read gives its
signal and the last relay's byte, and every readable copy the hub passed on
names the neighbour it came from. Before: 10:00 to 14:10 (4.2 hours); after:
14:25 to 17:10 (2.8 hours): the same part of the day. A laptop also read the
panel's traffic view once a minute from 13:06 (never a second meshtasticd
client). Names, IDs and places stay off the repo.

# Results

| | Before | After |
|---|---|---|
| Copies read, per hour | 102 | **135** (+32%) |
| Copies too weak to read, per hour | 180 | **274** (+52%) |
| Read, of all heard | 36% | 33% |
| Noise floor (RSSI − SNR of weak copies) | −110.2 dBm | −110.2 dBm |

The neighbours that hand packets to the hub (weak copies' median SNR; readable
copies passed on per hour):

| Neighbour | Where | SNR before | SNR after | Read/h before | Read/h after |
|---|---|---|---|---|---|
| The strongest, a Heltec V3 | about 2 km west (13-bit square) | −13.0 | **−4.0** | 32 | 31 |
| A rooftop base station | 12 km north-west | −12.25 | −12.0 | 35 | 22 |
| A neighbour | 1.2 km north-north-west | −13.5 | −14.75 | **72** | **6** |
| A hilltop router | about 20 km west (no place shared) | −15.5 | **−9.0** | 1 | **36** |
| A radio with no place | — | −15.75 | −11.5 | 0 | **31** |
| A radio with a 10-bit area | south (its area), likely within a few km | −14.75 (one copy) | **−0.6** | 0 | **35** |

- **West and the far west gained most**: the strongest neighbour by 9 dB,
  the hilltop router by 6.5 dB. Three neighbours that brought almost nothing
  now bring about 30 readable copies an hour each. Before, three neighbours
  brought nearly everything; after, six share it.
- **The south opened**: the Heltec V3 3.2 km south-south-west, which until
  now came only through relays (three at best this morning), was heard
  straight; a radio 5.5 km south-south-west came through two relays instead
  of four, and one 5 km south was heard at all. A radio whose rough area lies
  south and that was hardly heard before now arrives at about 0 dB. It may
  also have just been switched on or moved: its first packet in the hub's
  list came at 14:16, so it is not counted as the antenna's.
- **The north did not**: the neighbour 1.2 km north-north-west is 1.25 dB
  weaker and now rarely the first to hand a packet on (others get there
  first); radios 4 to 7 km north arrive as before.
- **No new noise**: the noise floor did not move, so the new spot has no
  interference of its own. More weak copies (the share read fell from 36% to
  33%) is the antenna hearing farther, not worse.
- A route traced to the rooftop base station 12 km away (one packet, 17:00)
  got no answer in six minutes: the hub hears it at about −12 dB, but no
  trace yet shows it hearing the hub.

# What it means

The raise was worth it: about a third more read, south and west open, and the
hub no longer hangs on three neighbours. The north is unchanged; if it
matters, the north chimney is the next thing to look at. To keep watching:
`tools/meshstats` weekly (skill `weekly-mesh-stats`), and *Start the zone
again* on the panel so the map's zone is the new antenna's alone (the readings
before the raise are kept on the hub).
