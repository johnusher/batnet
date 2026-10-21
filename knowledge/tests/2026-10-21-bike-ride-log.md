---
type: Test Report
title: "2026-10-21: the hub on a bike ride (first field log)"
description: The bench hub on a USB power bank on a bike ride, logging every packet it hears and saying hello every 5 minutes, beside a GPS track from a phone; set-up and timing here, results when the log is read.
tags: [field, lora, meshtastic, log, gps, power-bank, clock]
status: draft
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

To come: radios heard along the route, signal against distance, answers to
the hellos, and the hub's power and temperature over the ride.
