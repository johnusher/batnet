---
type: Research Note
title: The Gv4 Bot - another automatic node on Berlin's Meshtastic mesh, and what batnet takes from it
description: What mesh.lasystem.de's FAQ and pages say about the Gv4 Bot (a Heltec V4 and a Raspberry Pi 5 near Ostbahnhof with battery and solar, a 🤖 on everything automatic, quiet hours, 26 commands for help places, weather and signal, the Warnmeldung channel, statistics from LoRa only) and Berlin's public channels as it lists them; what that means for a batnet hub.
tags: [meshtastic, berlin, community, bot, etiquette, channels, warnings]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-26T16:00:00Z }
stale_after: 2027-04-26
sources:
  - id: faq
    resource: https://mesh.lasystem.de/faq/
    title: lasystem.de Mesh-Statistik - FAQ (114 entries, read 2026-10-26)
  - id: gv4
    resource: https://mesh.lasystem.de/gv4-bot/
    title: lasystem.de Mesh-Statistik - Gv4 Bot
  - id: commands
    resource: https://mesh.lasystem.de/bot-befehle/
    title: lasystem.de Mesh-Statistik - Bot-Befehle (26 active on 2026-10-26)
  - id: channels
    resource: https://mesh.lasystem.de/kanaele/
    title: lasystem.de Mesh-Statistik - Kanäle
  - id: stats
    resource: https://mesh.lasystem.de/
    title: lasystem.de Mesh-Statistik - overview, activity, growth, places, hardware
---

# What it is

The Gv4 Bot is a Meshtastic node run by one person as a hobby, with a public
statistics site about Berlin's mesh [^stats]. Its operator gives its place as
Ostbahnhof [^faq]. The radio is a Heltec V4 running Meshtastic; a Raspberry Pi 5
next to it runs the bot, stores the data and feeds the site [^gv4] - the same
split as a batnet hub (meshtasticd and batnetd on one Pi). Its site says:

* **Power**: a Tecnoware ERA PLUS DC UPS for the first hours, then a larger
  reserve and solar; "about 10 to 14 days" under good conditions [^faq][^gv4].
  The operator's claim, not measured here; compare the
  [power budget](/research/power-budget.md), where a Pi's idle draw dominates.
* **Kept cool** by a fan all the time, with a separate internet line [^gv4].
* **No MQTT**: everything it records comes from the air, so its statistics
  are Berlin's mesh as heard from one place, not all of it [^faq]. The same
  stance as batnet's [weekly stats](/skills/catalog.md).
* **Privacy**: it counts public messages but keeps no contents; it uses only
  the positions radios share themselves, and recognises place names only when
  someone writes them [^faq].

# How it behaves on the air

* **Everything automatic starts with 🤖.** A message without it was typed by
  the operator [^faq].
* **Quiet hours**: no automatic messages from 22:00 to 07:00 [^faq].
* **Every few hours a random line** from categories (mesh, small talk, tips,
  safety, bot notes, sarcasm), never the same text or category twice in a
  row, at fixed spacing [^gv4][^faq].
* **Answers greetings and Berlin place names** from a list of districts,
  localities, stations and nicknames ("Xberg"), with a pause per sender, and
  ignores words that only look like places ("Fahrstuhl", "Bett") [^faq].
* **Rule-based, not an AI**, and its answers are "for fun", not reliable or
  official [^faq].
* **Many answers go privately** to the asker rather than on the channel [^gv4].

# Its commands

26 on 2026-10-26, sent as a plain message starting with `!` [^commands]:

* **Help places nearest your radio's last shared position**: `!aed`
  (defibrillators), `!er` (emergency departments), `!hospital`, `!pharmacy`,
  `!police`, `!fire` (fire stations), `!pump` (Berlin's public emergency water
  pumps), `!water` (fountains), `!wc`.
* **The mesh**: `!signal` (the RSSI, SNR and hops of your own command message
  as the bot heard it), `!ping`, `!status`, `!active` (nodes seen now, in 24
  hours, in 30 days), `!distance` and `!bearing` (to the bot).
* **Weather and sky at the bot**: `!wetter`, `!rain`, `!wind`, `!air`, `!sun`,
  `!moon`.
* **Warnings**: `!mowas` and `!katwarn` give the channel's join link or the
  last warning, privately.
* `!witz` (a joke), `!help`, and `!bot`, restricted to the operator.

The commands are the bot's, not part of Meshtastic [^faq].

# Berlin's channels, as it lists them

Three public rooms, each with a join link for the Meshtastic app [^channels]:

| Channel | For |
|---|---|
| MediumFast (the primary channel) | general talk about Berlin's mesh |
| TEST | radio experiments and test messages, kept off the main channel |
| Warnmeldung | official warnings from MoWaS and KATWARN, prepared and posted automatically |

Berlin Chaos Mesh lists TEST, SOS and BerlinMesh instead
([presets in Berlin](/research/meshtastic-presets-berlin.md),
[etiquette](/research/mesh-etiquette.md)): the two sites agree on MediumFast and
TEST, and each has rooms the other does not. The *Warnmeldung* channel the
roof hub saw asked to stop posting warnings for all of Germany (log,
2026-10-25) is almost certainly this bot's: it is the only one listed that
posts MoWaS and KATWARN. Not confirmed; the hub still does not join it.

# Etiquette it writes down

Short messages; occasional tests are fine, many repeated ones are not; no
passwords, addresses, phone numbers or other private details; no device
names that sound official or like an emergency service; never transmit
without an antenna [^faq]. Antennas: 868 MHz and 50 ohm with matching SMA
plugs; a flat window cable (an example: 30 cm, SMA male to female) lets an
outdoor antenna feed a radio indoors without a balcony; an 868 MHz bandpass
filter can help at a site with strong interference, at the cost of a little
loss, and only then [^faq].

It also names two Berlin blackouts as the reason LoRa matters: Treptow-Köpenick
in September 2025 and the south-west around Zehlendorf in January 2026, when
mobile networks were affected too [^faq] (the record:
[the Berlin blackouts](/research/berlin-blackout-2025.md)).

# What it means for batnet

* **A precedent for automatic messages.** A 🤖 at the start and quiet hours at
  night are what one established bot in Berlin already does; batnet's reach
  test and away note ([0033](/decisions/0033-away-note.md)) could do the same.
  In [ideas](/ideas.md); the owner decides.
* **A second listener to measure the roof hub against.** `!signal` sent
  privately to the bot returns how strongly it heard the hub, from the other
  side of the city: one message, by hand, under the `talking-on-the-mesh`
  rules. Its site's statistics are a second view of the same mesh to compare
  with the hub's weekly stats. The hub's distance to it stays private (it
  would place the house).
* **Help places for the offline map.** The same kinds of places (defibrillators,
  emergency water pumps, pharmacies, hospitals) are open data that a hub can
  keep for a blackout, on the [offline map](/decisions/0032-offline-maps.md)
  with no radio traffic at all. In [ideas](/ideas.md).
* **Warnings**: the bot posts warnings on its own channel and answers
  `!mowas` privately; batnet's idea of keeping its own district's warnings
  while online and sending nothing on the air ([ideas](/ideas.md)) is the
  quieter version. Both need the internet to learn of a warning.
* **Cooperation**: the operator welcomes people who can keep a base station
  at a high site [^faq]. The roof hub might be one; that is for the owner to
  decide, through the site's contact form.
