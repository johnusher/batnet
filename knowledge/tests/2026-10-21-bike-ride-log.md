---
type: Test Report
title: "2026-10-21: the hub on a bike ride (first field log)"
description: The bench hub on a USB power bank on a bike ride, logging every packet it hears and saying hello every 5 minutes, beside a GPS track from a phone; one probable answer to a hello at the far end, heard but too weak to read.
tags: [field, lora, meshtastic, log, gps, power-bank, clock]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T15:25:00Z }
---

# Setup

* The [bench hub](/hardware/bench-pi7.md) (Pi 4, one Meshstick on its own antenna,
  EU868 LongFast), in a bag on a bike, on a USB power bank.
* The hub's log on (panel: Start logging): every packet with its signal (SNR,
  RSSI), relays and any shared position; a sample of the radio and the Pi every
  30 s; a hello (NodeInfo with want-response, about 1 s of airtime) every 5
  minutes. One JSON-lines file per boot in `/var/lib/batnet/logs/`.
* A GPS track recorded on the rider's phone (Open GPX Tracker on iOS), started
  at power-on. Positions are personal data: the track and any position heard
  stay out of the repo; results here use rounded distances.

# Before the ride

* After the power was moved to the power bank the hub was logging 24 s after
  power-on, on the radio at 29 s, first hello at 54 s. No under-voltage on the
  power bank (`vcgencmd get_throttled` = 0x0); 45 °C.
* **The clock:** the Pi has no clock battery. It booted with the time saved at
  the last minute before the power cut (`batnet-clock.timer`), 4 min 36 s behind
  the real time (the time without power), until it reached the home WiFi and
  NTP set it 40 s after boot. Every log line carries `up` (seconds since boot),
  so the early lines are corrected from the real boot time; a hand-added
  `clock` line in the log says so. Away from WiFi a power cut would leave the
  clock behind by the time without power ([B39](/known-problems.md)).

# Results

A ride of about 5 km and an hour (15:16-16:15 CEST), out to 1.7 km from
home in a straight line, a 12-minute stop at the far end, and back. The hub
was in a bag at about hip height.

| What | Result |
|---|---|
| Hellos sent | 12, every 5-5.5 minutes (the 30 s sample tick rounds the 5 minutes up); every one accepted by the radio |
| Packets the log recorded from other radios | none |
| Packets meshtasticd heard but could not read | 2, both during the stop at the far end (1.7 km), both SNR −20.75 dB, RSSI −94 dBm, failed CRC (RadioLib error −7) |
| The first of those | addressed to this hub, 16 s after a hello asking for an answer, relayed twice: almost certainly a reply to the hello |
| The second | a broadcast 48 s later, relayed four times, through the same relay as the first |
| The hub | no restart, no under-voltage (`get_throttled` = 0x0), 45-60 °C in the bag, about 1.56 GB of memory free throughout |
| GPS track | two gaps without fixes (21 min near the start, and the 12-minute stop: Open GPX Tracker records no points while standing still); the hub's times are correct to about a second (NTP before leaving) |

# What it shows

* There are Meshtastic radios within reach of a hub 1.7 km from home, at
  least one of them relaying: a hello from the hub reached someone, and their
  answer came back through relays, but too weak to decode. LongFast decodes
  down to about −17.5 dB SNR; these were 3 dB below. A few dB more (closer, a
  higher antenna, or a better position at the stop) would probably have been
  enough.
* Nothing at all was heard near home or on the way: the neighbourhood is
  quiet on LongFast, or the antenna in a bag at hip height hears little. Much
  of Berlin is on MediumFast, which a LongFast radio cannot hear at all
  ([presets in Berlin](/research/meshtastic-presets-berlin.md)).
* **The log misses packets that fail CRC.** meshtasticd drops them before its
  API, so batnetd never sees them; they were found only in meshtasticd's
  journal ([B40](/known-problems.md)). For field logs those near misses are
  the most interesting lines.
* The hub runs an hour on a power bank without trouble, and the autostart
  after the power move worked (logger, scanner and hellos all running without
  anyone touching it).

# Next time

* Log meshtasticd's "Ignore received packet" lines too (B40): done the same day, they are `unreadable` lines in the log now.
* The antenna up and outside the bag; stop for a few minutes at a few
  distances, since standing still gives the radio time to hear more.
* The second Meshstick at home, to measure the hub's own range directly.
