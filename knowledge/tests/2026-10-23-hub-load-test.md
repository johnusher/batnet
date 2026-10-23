---
type: Test Report
title: "2026-10-23: how many phones a hub's app takes, Pi 4 and Pi Zero 2 W"
description: Up to 200 pretend phones on each bench hub, in each power mode - no errors anywhere (a few live messages missed at 100 to 200 phones, B66); up to 100 phones a message reaches every phone in under 0.1 s typically on both boards; at 200 the slowest 5% wait 0.2 to 1.7 s, held up by the WiFi, not the processor (at most 18% busy). The built-in WiFi as an access point, untested here, is the likelier limit.
tags: [bench, load, pi4, zero-2-w, power-modes, web-app, wifi]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-23T07:00:00Z }
stale_after: 2027-04-23
sources:
  - id: openwrt-8
    resource: https://forum.openwrt.org/t/remove-limit-of-8-devices-on-wifi-network-on-raspberry-pi/128734
    title: Remove limit of 8 devices on WiFi network on Raspberry Pi?
    author: OpenWrt forum
  - id: dietpi-limit
    resource: https://dietpi.com/forum/t/wifi-hotspot-number-of-clients-limited/18401
    title: WiFi hotspot number of clients limited
    author: DietPi forum
---

# Setup

Both bench hubs, each running `batnetd -hub` with its radio:
[pi7](/hardware/bench-pi7.md) (Pi 4, 2 GB, with its screen's kiosk) and
[pz7](/hardware/bench-pz7.md) (Pi Zero 2 W, 512 MB, no kiosk). Each in high,
medium and low power mode ([0029](/decisions/0029-hub-power-modes.md)):

| Mode | Pi 4 | Zero 2 W |
|---|---|---|
| high | up to 1.8 GHz, WiFi power saving off | up to 1 GHz, power saving off |
| medium | up to 1 GHz, power saving on | up to 1 GHz (its top), power saving on |
| low | 600 MHz, power saving on | 600 MHz, power saving on |

A laptop on the home WiFi played every phone (`tools/hubload`): each pretend
phone signs up (a name and a PIN), is approved by the admin, holds the app's
live event stream, opens the Everyone conversation every 10 s, and posts to it
every 30 s, which is the 120 posts an hour the hub allows one person. Half
the phones sit on the home screen and fetch the conversation list on every
change, as the real app does. Each run: 25, 50, 100 and 200 phones for 60 s,
starting from the hub's saved database. The hub's sign-up limits (3 an hour
per address, 20 per house) were raised in a batnetd built for the test; the
real one and the saved database went back afterwards. `sample.sh` on the hub
recorded CPU, memory, temperature and throttling every 2 s.

The load is far above a real house: 200 such phones post 6.7 messages a second
and the hub sends out 1,300 live messages a second.

# Results

No errors in any run: every post was stored. In 20 of the 24 runs every phone
got every message live; in four (Pi 4 high and Zero high at 200 phones, Pi 4
low and Zero medium at 100) between 0.1% and 1.9% of the live copies did not
arrive. Some were posts still on their way when the run stopped; some may be
the hub dropping live events for a phone more than 64 behind (B66). batnetd
never restarted and nothing ran out of memory.

**A message reaching every phone** (from the post to its arrival on each phone; typical / slowest 5%, in ms):

| Phones | Pi 4 high | Pi 4 medium | Pi 4 low | Zero high | Zero medium | Zero low |
|---|---|---|---|---|---|---|
| 25 | 19 / 32 | 19 / 106 | 18 / 37 | 22 / 78 | 19 / 33 | 18 / 43 |
| 50 | 23 / 104 | 22 / 45 | 23 / 117 | 25 / 59 | 24 / 52 | 23 / 43 |
| 100 | 30 / 133 | 33 / 379 | 35 / 100 | 40 / 89 | 39 / 181 | 32 / 59 |
| 200 | 83 / 1,204 | 88 / 1,705 | 102 / 1,485 | 284 / 700 | 103 / 458 | 78 / 208 |

**Opening a conversation** (the last 100 messages; slowest 5%, in ms): up to
100 phones at most 0.6 s on either board; at 200 phones 0.4 to 3.3 s. The app's
page itself: typically 10 to 45 ms, 0.2 s on the Zero at 200 phones in high.

**Signing up** (one PIN hash, argon2id with 19 MB, eight at once):

| | high | medium | low |
|---|---|---|---|
| Pi 4 | 0.55 s | 0.87 s | 1.4 s |
| Zero 2 W | 1.7 s | 1.7 s | 2.8 s |

**The hub while it worked:**

| | Pi 4 | Zero 2 W |
|---|---|---|
| Whole Pi's CPU, chatting, 200 phones | 7 to 9% | 12 to 18% |
| Whole Pi's CPU during a burst of sign-ups | about 55% (two cores hashing) | about 60% |
| batnetd's memory, most | 143 MB | 139 MB |
| Memory left, least | 1.36 GB | 149 MB |
| Hottest | 51 °C | 48 °C |
| Throttled | yes, in high: **under-voltage** (B65) | never |

# What it means

* **The app is not what limits a hub.** Both boards took 200 phones in every
  mode with no errors (and with nearly every live message, B66), and up to 100 phones every message arrived in about
  a tenth of a second. Even the Zero's processor was at most 18% busy while
  chatting.
* **At 200 phones the slowdown is the WiFi, not the board.** The faster Pi 4
  was slower there than the Zero, and the Zero in low mode was faster than in
  high: the delay follows the radio link (one laptop's WiFi carried every
  phone's traffic) and varies from run to run, not the CPU's speed.
* **Low power mode costs only sign-ups.** Messages were as quick at 600 MHz;
  only the PIN hash is slower (2.8 s on the Zero). A hub can stay in low.
* **Memory is the Zero's tightest part**, and it held: 149 MB stayed free at
  the worst moment, a burst of sign-ups (each hash takes 19 MB).
* **pi7's power supply sags** when two cores work in high mode: 76 under-voltage
  events since its last boot (B65). The Zero, on its own supply, never.

# Not tested

* **The phones' WiFi.** Here the home router carried the phones. In a power cut
  phones join the hub's own WiFi from the Pi's built-in radio, and forum reports
  put its limit as an access point at about 6 to 8 phones, more with other
  firmware[^openwrt-8][^dietpi-limit]. For more phones than that, the house's
  router (on battery) or a separate access point carries them. Next: count how
  many phones the built-in WiFi takes on each board.
* Many real phones (browsers, sleeping screens, reconnects), and a long run.
* A house with years of messages (B60: the message list is scanned in full).
