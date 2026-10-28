---
name: weekly-mesh-stats
description: Use when the owner asks for the hub's weekly statistics, "the stats", trends in what a hub hears, whether its antenna or radio is getting worse, or how the mesh around it changed - and after moving an antenna or a hub. Runs tools/meshstats (a private snapshot of the hub's status, zone readings, sightings and meshtasticd's journal; a plain-words summary with trends; charts of traffic by hour, who brings the packets, and a map of links), reads it, checks the oddities, and records what is safe in the knowledge bundle. Also when the owner asks who talks to whom on the mesh, or what people are talking about (a news story going round): tools/meshstats/talk.py, a private map of conversations by day and the words the public channels used, run with every weekly snapshot.
---

# Weekly mesh statistics

One snapshot a week (or after a change to the antenna or the hub), each
compared with the ones before. The point is trends: a neighbour heard weaker
every week, a noise floor creeping up, a direction going quiet.

## Run it

```sh
tools/meshstats/meshstats.py pull pz7     # the hub's ssh name
tools/meshstats/meshstats.py report pz7
tools/meshstats/talk.py pz7               # who talks to whom, and the words of the week
```

`pull` reads the hub's panel API and saved files and meshtasticd's journal
since the last snapshot (at most eight days back; the hub keeps up to 400 MB of
journal, about a week and a half on the roof, so run it at least weekly or days
are lost), never meshtasticd itself: a second client takes the radio's only slot
(B38). Everything lands in `private/meshstats/<hub>/<date>/` (gitignored):
`summary.md`, `figures.json`, `report.html` (one page: the summary, a map with layers and names on hover, the flows, the traffic) and `report.png` (a picture of it, for a mail). They
name real radios and places.

## Read it, then check before you tell

1. **Open `report.html`** (or `report.png`) and `summary.md`. The summary's *Worth a look*
   is a list of suspicions, not findings.
2. **Antenna and radio health**, from the summary and the table *Over the
   weeks*:
   - *Heard straight*: the same neighbours, about as strong? A drop of 3 dB
     or more on several neighbours at once points at the hub (antenna, cable,
     connector, the radio); on one neighbour, at that neighbour.
   - *Noise floor* (RSSI minus SNR of weak copies): a rise of 3 dB or more is
     interference near the hub (a new power supply, a charger, LED lights).
   - *Readable share*: copies read against copies heard. It falls when the
     hub hears more at the edge of decoding, which can also mean more radios
     far away; read it with the noise floor.
3. **Oddities**: a radio close by that comes only through many relays (like
   *Basecamp*, 2026-10-25). Before calling it a problem, grep the snapshot's
   journal for it (`zcat meshtasticd.log.gz | grep -i <its hex id>`): the
   copies' `flags` byte holds the hops (top three bits: allowed, low three:
   left), `relay=` the last relay's byte, and a copy too weak to read may
   have come shorter than the one read. One packet is not a pattern.
4. **Rough places**: many radios sit on Meshtastic's grid (13 bits: squares
   5.8 by 3.6 km in Berlin); directions and distances to them are ranges
   ([research](/knowledge/research/meshtastic-position-precision.md)).
5. **Traced routes**: other people's traceroutes the hub overhears name
   direct links with their signal; they are the best evidence of who hears
   whom. Count them, do not over-read one.
6. **What kind of traffic and how crowded** (*What kind*, *Channel busy*):
   the share of packets for one radio (from the bare headers of garbled
   copies: private messages, their receipts, route checks), the share heard
   whole on channels the hub has no key for, how many pairs of radios wrote
   to each other (counted, never named; keep it so in anything recorded),
   and the busy waits. About 0.35 busy waits per packet passed on was usual
   on the roof ([test](/knowledge/tests/2026-10-27-channel-busy-on-the-roof.md));
   much more, with channel use rising, means crowded air.
7. **Who talks to whom** (`talk.py`, every run): it adds the week to an
   archive in `private/meshstats/<hub>/talk/` and writes `talk.html` there (a
   map of conversations, the days, the words people used day by day, the
   messages behind them; `talk.png` a picture). Private messages are known by
   their headers only (who, to whom, when), never their words. Look at:
   - **The busiest pairs and days**, against the weeks before: a new pair
     writing a lot, a day much busier than the rest.
   - **The word grid**: a word many different people used on one or two days
     is a story going round; one sender repeating a word is not (the grid
     counts senders, not messages). Open the messages behind it before
     calling it news.
   - **Receipts**: every private message sends one back, so *both ways* is
     not yet a conversation.
   A saved journal (`--import <files>`) fills a gap. Like the rest, it names
   real radios and what people wrote: the bundle gets counts and topics in
   general words only, never a name or a quote.

## Tell the owner

Plain words, short: what changed since last time, what is worth a look and
why, what you checked; the week's busiest talk and the words that rose, if
any. Send the summary and pictures by email only when
asked (the owner's address and sender are in their own setup, not here).

## Record it

- A test report in `knowledge/tests/` when something changed or was found
  (an antenna moved, a trend over three or more weeks): figures only, **no
  names, IDs or places** (rule 2 of `AGENTS.md`); directions and distances
  rounded (10°, 1 km).
- A new problem in `knowledge/known-problems.md`; the log in
  `knowledge/log.md`.
- The snapshot itself stays in `private/`; never commit it.
