---
type: Test Report
title: "2026-10-21: MediumFast against LongFast, and the antenna's place"
description: The bench hub at home in Berlin, 20 minutes on each preset with the antenna near the roof (MediumFast heard 26 radios, LongFast 5), then the antenna brought inside (nothing heard for an hour); what a hub needs to hear the city's mesh.
tags: [field, lora, meshtastic, presets, antenna, berlin]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T18:05:00Z }
---

# Setup

The [bench hub](/hardware/bench-pi7.md) (Pi 4, Elecrow Meshstick, its stock
antenna) at home in Berlin, EU_868, logging every packet: those it reads,
those it hears but cannot read (from meshtasticd's journal), and what it
sends or passes on. Hellos once an hour. Counts are of packets from other
radios; the radio's own telemetry is left out. Positions and node IDs stay
out of this repo.

# Results

## LongFast all day, antenna low

At home on LongFast with the antenna low, and on a one-hour bike ride
([test](/tests/2026-10-21-bike-ride-log.md)), the hub read nothing. Its
only catch was one probable answer to a hello at 1.7 km, too weak to read.

## MediumFast against LongFast, antenna near the roof

Back to back, 20 minutes each, the same evening:

| | MediumFast (17:20–17:40) | LongFast (17:41–18:01) |
|---|---|---|
| Packets read | 30 | 8 |
| Different radios | 26 | 5 |
| Heard directly (no relay) | 1 | 1 |
| Heard but unreadable | 33 | 3 |
| Median SNR of those read | −11.75 dB | −14.75 dB |

On MediumFast a hello brought four named answers within 20 s. Many
unreadable packets came in at a good SNR (−5 to −9 dB): collisions, not weak
signals. The hub stays on MediumFast.

## The antenna brought inside

Still on MediumFast:

| | Near the roof (18:42–19:02) | Inside (19:03–20:00) |
|---|---|---|
| Packets read | 47, from 32 radios | 0 |
| Heard but unreadable | 65 | 3 |
| Median SNR | −11 dB | nothing readable |
| Through the busiest relay | 20 of 47 | none |

An hourly hello at 19:04 got no answer.

# What it shows

* **In Berlin a hub must be on MediumFast** to hear most of the mesh
  ([presets in Berlin](/research/meshtastic-presets-berlin.md), draft
  decision [0027](/decisions/0027-preset-follows-the-local-mesh.md)).
* **The margin is thin.** Near the roof the median signal was about 1.5 dB
  above MediumFast's limit (−12.5 dB). A wall or a roof costs more than that,
  and the hub then hears nothing at all.
* **One relay carries most of it.** Almost everything came through relays,
  and through one more than any other: a hub depends on a link it does not
  control.
* **So a house hub needs its antenna outside and high**, with the Pi
  indoors on a short, low-loss cable, or a higher-gain antenna
  ([shopping list](/hardware/shopping-list-eu.md)).

# Not tested yet

* The antenna outside on a cable, and a 5 dBi antenna.
* Another time of day: the mesh may be busier or quieter.
* The hub's own reach: needs the second Meshstick.
