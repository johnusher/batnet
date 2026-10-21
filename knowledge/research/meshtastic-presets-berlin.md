---
type: Research Note
title: Meshtastic presets in Berlin - a city split between LongFast and MediumFast
description: Berlin's Meshtastic users are split between LongFast and MediumFast, which in EU_868 share one frequency but cannot hear each other; what the community reports, what it means for a hub, and what the bench hub measured.
tags: [meshtastic, lora, presets, berlin, interop, field]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T15:00:00Z }
sources:
  - id: reddit-divided
    resource: https://www.reddit.com/r/meshtastic/comments/1mmsay9/berlin_a_divided_city/
    title: r/meshtastic - Berlin, a divided city (August 2025, with node maps for both presets)
---

# What the community reports

From one Reddit thread of August 2025 [reddit-divided], a year before this
note, so the balance may have moved since:

* Part of Berlin moved from LongFast (LF) to MediumFast (MF) because 868 MHz
  is crowded: MF's packets are about three times shorter in the air, so fewer
  collide. The choice was made in the Berlin Meshtastic Telegram group after
  coordinated tests; one user reports conversations across half the city on
  MF that never worked on LF.
* Freifunk and c-base switched to MF; they run repeaters on the Teufelsberg
  and on church towers, so "most of the momentum is on MF".
* Counts from single homes: 80 LF and 79 MF nodes over 16 hours (one user);
  80 MF and 30 LF in the east of the city (another). One user asked whether
  the city had switched back to LF, so not everyone agrees which one is
  current.
* The node maps posted with it: MF nodes mostly in the west, north and
  centre; LF nodes in the centre, south-ish and east, and around Potsdam.
* A visitor on LF heard almost no one. Others blame LTE interference too:
  repeaters with band-pass filters do much better than bare boards.

# What it means for a hub

* **In EU_868 all 250 kHz presets share one frequency, 869.525 MHz**
  (measured on the bench hub: switching to MediumFast left meshtasticd on
  869.525 MHz). They differ in spreading factor (LongFast SF11, MediumFast
  SF9), and a LoRa radio only hears its own spreading factor, so an LF radio
  and an MF radio side by side hear nothing of each other, not even as noise
  they can read.
* A hub's public channel only reaches the people on the same preset. Which
  one is local is a community choice, not a technical default, and it can
  differ across one city. Draft decision
  [0027](/decisions/0027-preset-follows-the-local-mesh.md) makes the preset a
  village setting.
* On a faster preset each hello and each message costs less shared airtime,
  but reaches a little less far (about 2.5 dB less link budget from SF11 to
  SF9).

# What the bench hub saw

* On LongFast, at home and on a bike ride on 2026-10-21, it decoded nothing;
  at the far end of the ride it heard one probable answer to a hello, too
  weak to read ([test](/tests/2026-10-21-bike-ride-log.md)).
* From the evening of 2026-10-21 it listens on MediumFast at home overnight,
  hello once an hour, to compare.
