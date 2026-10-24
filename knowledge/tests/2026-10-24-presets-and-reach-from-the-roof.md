---
type: Test Report
title: "2026-10-24: a night on the other presets, and how far the roof hub reaches"
description: The roof hub sent one private message to each of 29 radios 8.5 to 24.6 km away (none confirmed, two passed on by a relay, no answers), then listened on LongFast for 55 minutes (5 packets read from 2 radios, all through relays at the noise floor) before its radio stick froze switching to LongModerate and stayed frozen for five hours.
tags: [field, lora, meshtastic, presets, reach, berlin, roof]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-24T06:30:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md) (Pi Zero 2 W, Elecrow Meshstick,
antenna on the roof), EU_868, home preset MediumFast, hop limit 3. Two tools
ran on it overnight, both through the panel's API, never a second meshtasticd
client:

* `tools/reachtest` from 21:09 to 01:50 on 2026-10-23: one private message in
  German every 10 minutes to the farthest radio not yet written to, first those
  with a known place at least 8 km away, then those with no place that came
  through at least two relays, never one beyond the hub's hop limit; one
  message per radio, following the [etiquette](/research/mesh-etiquette.md).
  Stopped at 01:55 with 29 tried, so it could not overlap the survey.
* `tools/presetsurvey` from 02:00: 55 minutes on each of LongFast,
  LongModerate, LongSlow, MediumSlow, ShortSlow and ShortFast, one hello on
  each, then back to MediumFast.

Radio names, IDs and places are in a private report, not here.

# The reach test

| Outcome | Radios | Distance |
|---|---|---|
| Confirmed by the far radio (delivered) | 0 | |
| Answered by a person | 0 | |
| Passed on: a relay was heard sending it on | 2 | 8.8 and 19.2 km |
| No answer after three tries (error 5) | 16 | 8.5 to 16.6 km |
| No key for that radio (error 39) | 10 | 9.8 to 24.6 km (two with no place) |
| Still sending | 1 | 22 km |

* **Error 5 is mostly [B73](/known-problems.md)**: in 9 of the first 10
  cases checked, the nearest strong neighbour (about 1 km away) passed the
  message on within 5 to 17 s, but the hub heard that echo garbled (failed
  checksum, SNR −10 to −12 dB, RSSI −122 dBm), so its radio never counted it
  as passed on. The messages went into the mesh; whether they arrived is
  unknown.
* **Error 39**: the hub asked each radio for its name card, as [B72](/known-problems.md)'s
  fix does, and would have sent again within the hour. None came in time. One
  card arrived 2 h 40 min later through five relays, most likely that radio's
  routine broadcast; the tool's report counts that as *reached*, which
  overstates it.
* **Still sending**: the message at 21:30 went out ten seconds before a
  three-minute LongFast trial restarted the radio, so it probably lost its
  repeats. It was not sent again.
* It ran at night: a person who received a message was likely asleep.

So hearing radios 20 km away works (through relays), and talking to them is
unproven: not one receipt came back from a radio more than 8 km away. A
receipt has to travel back through the same two or three relays, and the
hub's messages may cross at most three, which leaves no slack for radios
first heard through two or three.

# The survey

| Preset | Listened | Packets read | Unreadable | Radios read | Straight |
|---|---|---|---|---|---|
| LongFast | 02:00-02:56 | 5 | 15 | 2 | 0 |
| LongModerate, LongSlow, MediumSlow, ShortSlow, ShortFast | not heard: the radio froze | | | | |

* **LongFast at night is nearly empty from this roof**: 5.5 packets read an
  hour, against 205 an hour on MediumFast through the night of
  [2026-10-22](/tests/2026-10-22-one-night-on-mediumfast.md) (154 radios) and
  24 an hour on LongFast in the
  [20-minute test of 2026-10-21](/tests/2026-10-21-presets-and-antenna.md)
  (5 radios, in the evening).
* All five readable packets were encrypted on a channel the hub has no key
  for, from two radios the hub has never heard on MediumFast, one through one
  relay and one through two. None shared a place.
* 17 of the 20 packets were last passed on by the same relay (the last byte of
  its ID is all a packet carries).
* Everything sat at the bottom: SNR −19 to −21.5 dB, RSSI −128 dBm (the
  radio's noise floor). The unreadable ones were weak, not collisions, unlike
  MediumFast's; 13 of the 15 carry what looks like one radio's ID with bits
  flipped.

## The radio froze

At 02:56 the survey set LongModerate (the one preset of these on a 125 kHz
channel). meshtasticd logged the radio set-up to *Final Tx power* and then
nothing: no API, no packets. A restart of meshtasticd froze at the same line,
and so did one after its saved preset was set back to MediumFast with the
Meshtastic library's protobufs. A USB reset of the Meshstick (`usbreset
1a86:5512`) at 08:01 brought it back on MediumFast at once. So the stick
locked up, not the setting ([B74](/known-problems.md)). The survey tried
each next preset and the way home, was refused each time (*radio not
connected*), and ended; its guard, written for a reboot, could not help either
([B75](/known-problems.md)). The hub was off the air from 02:56 to 08:01.

The map zone's readings, set aside at 02:00, were put back by hand.

# Coverage, from the places the hub knows

An ellipse fitted to the radios' places (two standard deviations of their
positions in kilometres, which holds about 86 % of them), on MediumFast:

| Radios | How many | Ellipse | Its middle |
|---|---|---|---|
| Every radio with a known place | 52 | 21.5 × 17 km, axis 126° | 7.3 km to the north-west |
| Through two relays at most | 19 | 19 × 9.9 km, axis 137° | 5.8 km to the north-west |
| Heard straight | 3 | 5 × 1.2 km | 1.2 km to the north-west |

The roof sits towards the south-eastern end of what it hears: it hears the
city through relays to its north-west, and its straight view is the
north-west (the south is the chimney's side, see [pz7](/hardware/bench-pz7.md)).
LongFast gets no ellipse: no place came with it.

# What it shows

* MediumFast is where the people are; LongFast round here at night is two
  radios at the edge of hearing.
* A far radio being heard is not the same as reaching it. A receipt needs the
  way back, and B73 hides even the first relay's echo. Fix B73 before reading
  more into error 5, then repeat by day.
* A preset survey needs its own way out of a frozen radio stick: a USB reset
  and the home preset when the radio is not back within minutes.

# Afterwards

B73 was fixed the same morning: the hub now counts a garbled echo carrying
its own ID and a sent message's packet ID as *passed on*. Checked against the
radio's log of the night, 10 of the 16 error-5 messages had such an echo, so
they would have read *passed on*; 6 had none heard.
