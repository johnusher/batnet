---
type: Research Note
title: Meshtastic presets in Berlin - a city split between LongFast and MediumFast
description: Berlin's Meshtastic users are split between LongFast and MediumFast, which in EU_868 share one frequency but cannot hear each other; what the community reports, what it means for a hub, and what the bench hub measured.
tags: [meshtastic, lora, presets, berlin, interop, field]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T15:00:00Z }
sources:
  - id: reddit-divided
    resource: https://www.reddit.com/r/meshtastic/comments/1mmsay9/berlin_a_divided_city/
    title: r/meshtastic - Berlin, a divided city (August 2025, with node maps for both presets)
  - id: reddit-freifunk
    resource: https://www.reddit.com/r/meshtastic/comments/1naqcci/berlin_freifunk_is_now_mediumfast/
    title: r/meshtastic - Berlin Freifunk is now MediumFast (2025)
  - id: mt-blog
    resource: https://meshtastic.org/blog/why-your-mesh-should-switch-from-longfast/
    title: Meshtastic blog - Is LongFast holding your mesh back? (22 April 2025)
  - id: chaosmesh
    resource: https://potatomesh.net/pages/about
    title: Berlin Chaos Mesh (PotatoMesh) - the settings Berlin's mesh community publishes
  - id: ff-list
    resource: https://lists.berlin.freifunk.net/pipermail/berlin/2024-November/043130.html
    title: Berlin Freifunk list - Meshtastic and Freifunk (monthly meetup at c-base)
  - id: mt-telegram
    resource: https://meshtastic.discourse.group/t/meshtastic-telegram-groups/7003
    title: Meshtastic forum - Meshtastic Telegram groups (undated)
  - id: raspberry-tips
    resource: https://raspberry.tips/?p=19026
    title: raspberry.tips - German regional presets, after Mesh Dresden's overview
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

What else the people in that thread report, which matters for batnet:

* **Antennas decide it.** A user with a ground-plane antenna on a mast on a
  top-floor terrace holds conversations across half the city on MF. Another,
  after fitting a better antenna, counts more than 170 nodes but hears only
  four directly (two of them well-placed routers), and traceroutes succeed in
  5–10% of tries within three hops.
* **Splitting a mesh has a cost.** In another city part of the community
  moved to ShortFast: radios at the edge then heard no one, the remaining
  LongFast users were cut off, and a few enthusiasts were left talking among
  themselves. MediumFast was suggested as the compromise.
* **There is no automatic way to follow a local preset.** Someone asked for
  one (a map of congested areas); the answer was that it is a community
  decision, made in Berlin's Telegram group. A proposal for telling a radio
  that two communities share an area exists, but nothing more.
* **The wave is just starting:** "1,000 new users a week" in one channel;
  crowded areas will grow.

Since then the move has gone further:

* Freifunk Berlin's repeaters switched one after the other: "After
  Teufelsberg and Zwingli, also the repeater at Vaterhaus switched to
  MediumFast. I recommend we use this momentum to move the rest of the
  community to MediumFast, too." [reddit-freifunk]
* Berlin Chaos Mesh, the community's own page, publishes MediumFast at
  869.525 MHz, 250 kHz, as Berlin's Meshtastic preset: public channel
  "MediumFast", a coordination channel "BerlinMesh" with its own key, and
  TEST and SOS channels with the open key. (Its key is public on that page;
  it is not copied into this repo.) It lists MeshCore at 869.618 MHz,
  62.5 kHz, and Reticulum at 869.475 MHz, 125 kHz, beside it. [chaosmesh]
  (The page's table gives SF 8 for MediumFast; Meshtastic's MediumFast is
  SF 9, which is what the bench hub's radio uses.)
* The BerlinMesh channel's hash is 35. Berlin's welcome bot greets a new radio
  on the public channel and then privately; its name card, with the public key
  a private message needs, goes only on BerlinMesh, so a radio without that
  channel cannot open the bot's private message (seen on the bench hub,
  2026-10-22).
* A German overview after Mesh Dresden: Berlin on MediumFast, Hessen on
  ShortSlow; a radio left on LongFast does not hear the local network.
  [raspberry-tips]
* Meshtastic itself recommends moving off LongFast in dense meshes (more than
  about 60 nodes close together, or a busy city): MediumFast or MediumSlow for
  dense urban meshes, ShortFast or ShortSlow for very dense ones. Its link
  budgets: LongFast 153 dB at 1.07 kbit/s, MediumSlow 150.5 dB at 1.95,
  MediumFast 148 dB at 3.52, ShortSlow 145.5 dB at 6.25, ShortFast 143 dB at
  10.94, ShortTurbo 140 dB at 21.88. [mt-blog]

# Where Berlin's mesh people talk

The Reddit thread mentions a Berlin Meshtastic Telegram group, where the
preset tests were run, but gives no name or link, and none was found. What
can be reached: Berlin Chaos Mesh's Matrix room #berlinmesh:c-base.org and its
forum, forum.chaosmesh.net [chaosmesh]; Freifunk Berlin's monthly meetup,
the first Wednesday at c-base [ff-list]; the national Telegram group
@meshtasticgermany, from an undated post [mt-telegram]; and the public
MediumFast channel itself.

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
* MediumFast sends about 3.5 kbit/s against LongFast's 1 kbit/s, so each
  hello and message takes about a third of the airtime (a 150-byte message:
  about 0.4 s against 1.4 s). It needs a stronger signal: it decodes down to
  about −12.5 dB SNR against −17.5 dB, about 5 dB less link budget (2.5 dB
  per spreading-factor step), so somewhat less range.

# What the bench hub saw

* On LongFast, at home and on a bike ride on 2026-10-21, it decoded nothing;
  at the far end of the ride it heard one probable answer to a hello, too
  weak to read ([test](/tests/2026-10-21-bike-ride-log.md)).
* The [test report](/tests/2026-10-21-presets-and-antenna.md) has the evening's numbers.
* With the antenna moved up near the roof the same evening, 20 minutes on
  each preset, back to back:

  | | MediumFast | LongFast |
  |---|---|---|
  | Packets read | 30 | 8 |
  | Different radios | 26 | 5 |
  | Heard but unreadable | 33 | 3 |
  | Median SNR | −11.75 dB | −14.75 dB |

  At this house MediumFast is where the mesh is. Only one radio was heard
  directly on either; almost everything came through relays, most through
  one. A hello on MediumFast brought four named answers within 20 s. Many
  unreadable packets came in at a good SNR (−5 to −9 dB): those collided
  rather than being too weak, the crowding the community reports. The hub
  stays on MediumFast.
* The antenna's place, on MediumFast the same evening: near the roof, 20
  minutes gave 47 packets from 32 radios (65 unreadable), median SNR −11 dB;
  brought inside, the next six minutes gave none, one unreadable at −16 dB,
  and an hourly hello got no answer. The roof position was only about 1.5 dB
  above MediumFast's limit (−12.5 dB), and almost everything came through one
  relay (20 of 47 packets): a wall's few dB lose the whole mesh. A hub wants
  the Pi indoors and the antenna outside and high, on a short, thick cable
  (at 868 MHz RG-174 loses about 1 dB a metre, RG-58 about 0.5 dB).
* [One night on MediumFast](/tests/2026-10-22-one-night-on-mediumfast.md)
  (8.7 hours): 1,788 packets read from 154 radios spread across about 29 km
  of the city, five of them heard directly, most packets through two to four
  relays and some through seven; 77 % of senders set a hop limit of 7, not
  Meshtastic's default 3.
* [A night on the other presets](/tests/2026-10-24-presets-and-reach-from-the-roof.md)
  from the roof hub, 2026-10-24: 55 minutes on LongFast at night read 5
  packets from 2 radios (5.5 an hour against MediumFast's 205), all through
  relays at the noise floor (SNR about −20 dB); none shared a place. The radio
  stick froze when switched to LongModerate, so the slower presets are still
  unheard.
