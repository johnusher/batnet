---
type: Test Report
title: "2026-10-27: why the roof hub's radio so often finds the channel busy"
description: 35 hours of the roof hub's radio log - about 70 "channel busy" waits an hour, all but 1 % of them while it waited to pass on someone else's packet, right after hearing it, when other relays were passing on the same one; about one relay in four waits at least once. Channel use 5.5 % (48 neighbours, median 3.7 %), its own airtime about 1 % (a tenth of the allowance); more at night, with more traffic. Nothing batnet sends causes it; nothing to fix.
tags: [field, lora, meshtastic, roof, airtime, relaying, berlin]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-27T19:40:00Z }
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md), MediumFast, role *client*, hop
limit 5 ([0034](/decisions/0034-hop-limit-five.md)), firmware 2.7.26. The
owner saw many *channel busy* entries in the panel's traffic. Source:
meshtasticd's journal on the hub's card from 26 October 10:25 to 27 October
20:15 (35 hours, two restarts of meshtasticd), and the panel's status. A
*channel busy* is the line `Can not send yet, busyRx`: the radio had a packet
ready, was hearing someone at that moment, and waited a random moment before
trying again (`internal/meshtastic/journal.go`).

# What the log shows

| | per hour |
|---|---|
| *Channel busy* lines | 73 (1,851 separate waits in 35 h, some retried 2 to 6 times) |
| Packets the hub decided to pass on | 203 |
| Packets it sent (its own counter) | 174 to 185 |
| Its own packets (name, place, a hello to a new radio) | about 3 |
| Packets read (every copy) | 620 to 700 |
| Copies heard but unreadable | 290 to 330 |

* **The waits are relaying.** 2,433 of the 2,467 busy lines (99 %) came
  within 10 s of the hub deciding to pass on a packet; 32 followed its own
  sends, about one a day. About one relay in four (1,851 waits for 6,866
  relays) found the channel busy at least once.
* **Why relays meet a busy channel.** A relay goes out a moment after the
  packet is heard, which is exactly when the other radios that heard it pass
  it on too. Meshtastic gives each relay a random wait in a window set by
  the signal (weaker copies wait less, to carry it further), and listens
  before sending; the busy line is that wait working. Over the whole hour
  the channel is quiet, but in the second after a packet it is crowded.
* **The channel is not crowded.** The hub's radio measured 5.5 % channel use
  on average (the highest minute 17.5 %). 48 neighbours that report theirs:
  median 3.7 %, the middle half 1.8 to 6.9 %, the highest 24 %. A roof site
  hearing about 78 radios at a time sits in the upper half, as expected.
* **Its own airtime is small.** About 1 % of each hour (0.96 % on average),
  a tenth of the 10 % the band allows. Neighbours: median 0.69 %, upper
  quarter above 0.93 %: the hub is among the busier relays, as a well-placed
  one is.
* **More at night.** From 22:00 to 06:00 it heard and passed on about a
  fifth more (220 to 250 relays an hour against 180 to 210 by day), and
  waited more (75 to 106 an hour against 55 to 80). The busy count follows
  the relaying hour by hour.
* **What it passes on.** 43 % of its relays (2,942) were packets it cannot
  read: mostly one private channel, others' direct messages, and radios
  that named their channel *LongFast* while on MediumFast. That is
  Meshtastic's default (rebroadcast mode *all*): a relay carries what it
  cannot read. One router two relays away sends a traceroute about every
  6 minutes, day and night, and its routes often run through the hub: about
  7 of the hub's relays an hour.
* **Unreadable copies.** About a third of the copies heard failed their
  checksum. Half were near MediumFast's limit (below -12 dB SNR); a fifth
  were at -7 dB or better, where a lone packet reads cleanly: two packets
  overlapping, most likely. That loss is the real cost of a busy mesh, more
  than the waits. The log also shows 438 *missed RX_DONE* and 142 interrupts
  out of receive mode in 35 h: the USB radio stick on the Zero 2 W missing
  a little, not measured further.

# Conclusions

* **Nothing is wrong.** The waits are the radio taking turns while it
  relays for others; batnet's own messages cause about one a day. The hop
  limit of 5 changes only the hub's own few packets.
* **Ways to have fewer, not taken**: rebroadcast mode *local only* or
  *known only* would stop it carrying packets it cannot read (about 40 % of
  its sending, and of the waits), and role *client mute* would stop all
  relaying. Both would take a roof site out of the mesh it lives on;
  Berlin's mesh works because radios like it relay.
* **What to watch**: channel use near 25 % (Meshtastic holds back
  non-essential sends around there), or busy waits far above about one in
  four relays. [Weekly stats](/skills/catalog.md) could chart busy waits
  against relays.
