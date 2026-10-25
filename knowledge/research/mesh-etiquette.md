---
type: Research Note
title: Etiquette on Berlin's community mesh - what the communities publish, and how batnet behaves there
description: What Freifunk, the Pico Peering Agreement, the CCC's hacker ethics, Berlin Chaos Mesh and Meshtastic say about sharing a community radio network, and the rules batnet follows when its hubs talk to people on it.
tags: [meshtastic, freifunk, community, etiquette, berlin, airtime]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-23T19:45:00Z }
sources:
  - id: freifunk
    resource: https://freifunk.net/en/what-is-it-about/
    title: freifunk.net - What is it about?
  - id: ppa
    resource: http://www.picopeer.net/PPA-en.shtml
    title: The Pico Peering Agreement
  - id: hackerethik
    resource: https://www.ccc.de/de/hackerethik
    title: Chaos Computer Club - Hackerethik
  - id: chaosmesh
    resource: https://potatomesh.net/pages/about
    title: Berlin Chaos Mesh (PotatoMesh) - settings and channels
  - id: mt-lora
    resource: https://meshtastic.org/docs/configuration/radio/lora/
    title: Meshtastic docs - LoRa configuration
---

# What the communities say

**Freifunk** calls itself "a non-commercial initiative for free wireless
networks": people put up their own routers, carry each other's traffic, and
local groups run their own software and meet regularly [freifunk]. Its peering
template, the **Pico Peering Agreement**, asks each node owner for free transit
(no charge, the data neither modified nor interfered with), open communication
(publish what peering needs, under a free licence, and be contactable), and
promises no warranty: a service "can be scaled back or withdrawn at any time"
[ppa].

Berlin's mesh grew around c-base and the **Chaos Computer Club**, whose hacker
ethics include "Mülle nicht in den Daten anderer Leute", "Öffentliche Daten
nützen, private Daten schützen" and "Mißtraue Autoritäten – fördere
Dezentralisierung" [hackerethik].

**Berlin Chaos Mesh** publishes no code of conduct, but names its channels'
purposes: an open TEST channel "for pings and testing", an SOS channel for
"emergency use only", and BerlinMesh as the coordination channel; MeshCore
has #test and #ping [chaosmesh]. Its forum and Matrix room are where to ask
before anything new (see [presets in Berlin](/research/meshtastic-presets-berlin.md)).

**Meshtastic** keeps the hop limit at 3 by default ("Really, 3 is fine"),
recommends faster presets against congestion, and enforces the EU868 duty
cycle of 10% over a rolling hour [mt-lora].

None of them forbids a private test message to a radio, and none writes down
how many are too many: the rules below are batnet's own reading of the spirit.

# How batnet behaves there

* Airtime is shared: one private message per radio and purpose, never
  repeated; tests that go to everyone use the TEST channel, not the public one;
  hellos that ask for answers rarely (each answer is someone else's airtime).
* The SOS channel is for emergencies: a hub only listens there. Nothing automatic goes on it, and the hub refuses a message unless a person confirms it (types SOS), one per 10 minutes (2026-10-25).
* Honest: an automatic message says it is automatic when asked.
* Humble: batnet is a guest in a network others built. It mentions itself and
  its repository when someone asks what it is, thanks people, invites their
  advice, and offers back what it learned, in the spirit of open communication.
* Private data protected: people's names, radio IDs, places and words stay on
  the hub and in private reports; the repository gets counts and distances
  ([secret hygiene](/security/secret-hygiene.md)).
* An objection ends it: an apology, and the radio is never written to again.

The agent skill `talking-on-the-mesh` (in `.claude/skills/`) carries this into
practice: when to answer, what to write, the 200-byte limit, opt-outs.
