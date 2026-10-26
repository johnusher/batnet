---
type: Decision
title: "0034: A hub sends with a hop limit of 5 where the local mesh uses more than 3"
description: The roof hub's own packets may be passed on 5 times, not Meshtastic's default 3 - in Berlin, where most radios send with 7, a limit of 3 reached about 60 % of the radios the hub hears and 5 about 88 %; 7 is left to the city, and anything automatic stays rare.
tags: [lora, meshtastic, hops, etiquette, berlin]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-26T21:40:00Z }
verified: { by: human:johnusher, at: 2026-10-26T21:30:00Z }
sources:
  - id: mt-lora
    resource: https://meshtastic.org/docs/configuration/radio/lora/
    title: Meshtastic docs - LoRa configuration (hop limit, default 3, at most 7)
---

# Context

The hop limit is how many times other radios may pass on a packet this hub
sends: its texts, its name and its place. It does not change what the hub
hears; each sender sets its own. Meshtastic's default is 3 and its docs say
3 is enough [^mt-lora]. Both bench hubs ran at 3
([etiquette](/research/mesh-etiquette.md)).

Berlin does not: on the [first night](/tests/2026-10-22-one-night-on-mediumfast.md)
77 % of packets were sent with a limit of 7 and 15 % with 3. Over
[two days on the roof](/tests/2026-10-25-two-days-on-the-roof.md) the hub
heard 184 radios, by relays on the shortest copy: 5 straight, 12, 39 and 53
through one to three relays, 30 through four, 23 through five, 22 through
six or seven. If paths work about the same both ways, a limit of 3 reached
about 109 of them (59 %), 5 about 162 (88 %), 7 all. A private `!signal` to a
bot four relays away ([the Gv4 Bot](/research/berlin-gv4-bot.md)) got no
further than *passed on* at 3.

# Decision

* **The roof hub sends with a hop limit of 5** (set 2026-10-26 on
  [pz7](/hardware/bench-pz7.md), owner's choice).
* **Not 7.** The last two hops add 22 radios for the widest, most expensive
  rings of repeats; the city already floods at 7, the hub need not.
* **Elsewhere the default stays 3**, unless the local mesh, like Berlin's,
  mostly uses more: like the preset ([0027](/decisions/0027-preset-follows-the-local-mesh.md)),
  the hop limit follows the neighbours, a step below them.
* **Automatic messages stay rare**: at 5 each costs more of everyone's
  airtime, so the reach test's and the away note's limits matter more
  ([0033](/decisions/0033-away-note.md), [etiquette](/research/mesh-etiquette.md)).

# Consequences

* The hub's messages, name and place reach most of the radios it hears.
* Each broadcast it sends is passed on by more radios. The hub sends little
  (its place once an hour, its name now and then, a few texts), and private
  messages flood only until firmware learns a route.
* First result, the same evening: the `!signal` to the Gv4 Bot, sent again at
  5, reached the bot's radio four relays away, which answered with error 35
  (it does not know the hub's key yet, so it could not open the message); at
  3 the same message got no further than *passed on*. Once the bot has heard
  the hub's name and key, which now travel 5 hops too, a second try should be
  read.
