---
type: Issue List
title: To do
description: What comes next for milestone 1, in order - plans, bench tests, decisions waiting, and the known problems to fix first.
tags: [todo, roadmap, milestone-1]
status: stable
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T13:45:00Z }
---

Tick items off here (move them to Done with the date). Bugs and limits live in
[known problems](known-problems.md); this page is the order of work.

# Milestone 1 plans ([spec](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md))

- [x] Plan 1: core log, sync, mock village (2026-10-15)
- [x] Plan 2: village runtime, scenarios, the real code in the simulator (2026-10-15)
- [x] Plan 3: LoRa bridge and priority, in the simulator (2026-10-15)
- [x] Plan 4a: the house server - state from the log, PIN accounts and admin approval, captive portal, API and live events, a first real app; `batnetd -mock 3 -web` (2026-10-15)
- [x] Plan 4b: the full app - outbox, bedside mode, speaker and notification settings, German, help, admin alerts; the real app inside the simulator's phone (2026-10-15)
- [ ] **Plan 3b: meshtasticd client on real radios - includes everyone-channel posts as plain Meshtastic text (done-when check 3) and public-channel interop (next).** Started 2026-10-21: `internal/meshtastic` reads the radio, the node list and texts and sends text and hellos; the private channel, envelopes and the bridge to the log remain
- [ ] Plan 5: Pi set-up, radios, kiosk, speaker - first boot, QR join, radio detection, batman-adv and hostapd, one-hop heartbeats on the mesh interface, cage + Chromium kiosk, speaker/button/LED, GPS time. Started 2026-10-21 on the [bench hub](/hardware/bench-pi7.md): `deploy/pi/` (set-up script, systemd units, watchdogs), the kiosk, batman-adv bring-up (`batnet-mesh`), sync over bat0 (`internal/udplink`), the hub panel ([0025](/decisions/0025-hub-panel-and-mesh-modes.md))
- [ ] Plan 6: image, installer, CI hardening - the batnet.zip, Imager scripts, signed releases, gitleaks in CI

# Bench tests (need hardware)

- [ ] Two Pis + two Meshsticks: meshtasticd, range, airtime, priority queue, the bridge between two WiFi groups (two hubs are up: the second, [pz7](/hardware/bench-pz7.md), since 2026-10-23, has its Meshstick on MediumFast, BerlinMesh and the first hub's `village.key`; their logs have no path between them yet: the USB WiFi stick for batman-adv, or plan 3b's bridge)
- [ ] On 2026-10-25, the roof against the first hub's day with the same antenna: `tools/hubcompare/compare.py pz7 pi7:/var/lib/batnet/antenna-links-2026-10-23.json`, and the second hub's zone ([bench notes](/hardware/bench-pz7.md))
- [ ] The keyboard on the hub's screen (the 800 × 480 display works, 2026-10-21); real mesh mode with the USB WiFi stick
- [ ] How many phones the hub's own WiFi (the Pi's built-in radio as an access point) holds, on a Pi 4 and a Zero 2 W; the app took 200 ([load test](/tests/2026-10-23-hub-load-test.md))
- [ ] Microphone and WebRTC after the certificate warning on a real iPhone and Android (decision 0004 depends on it)
- [ ] Captive-portal pop-up on each phone type; desktop notifications past the warning; Android background tab; iOS Wake Lock
- [ ] Power: the USB WiFi stick; a pocket node on a power bank
- [ ] Moonshine on Pi 4, Pi 5, Zero 2 W; raw vs denoised audio

- [x] Read the bike-ride log against the GPS track ([test](/tests/2026-10-21-bike-ride-log.md)): one probable answer to a hello at 1.7 km, too weak to read
- [x] The hub's log picks up packets meshtasticd heard but could not read (B40)
- [x] MediumFast against LongFast at the bench hub, 20 minutes each with the antenna near the roof: 26 radios against 5; the hub stays on MediumFast ([research](/research/meshtastic-presets-berlin.md)); confirm draft decision [0027](/decisions/0027-preset-follows-the-local-mesh.md)
- [x] Hub panel: messages to one radio and replies with delivery ticks, a traffic view of everything on the air (readable, encrypted, unreadable, sent, passed on)
- [x] Hub panel: a simple Home, a map of shared positions, click a radio or a packet for what it says, Settings for the radio (preset, relays, power, listen-only), the name on the air and the hub; the bench hub is "lorabatnet 🐰🦇"
- [ ] Offline map tiles for the village's area on the hub (L17)
- [x] The hub's own WiFi when the home WiFi is gone (B41): from the panel, the button or by itself after 3 minutes, password and QR, neighbours let in only by an admin, back home always ([0028](/decisions/0028-hub-own-wifi.md), [test](/tests/2026-10-21-hub-power-cut-kit.md))
- [x] Plan 5a, the power-cut kit: joining other WiFi, sound on the jack, an LED and a button on GPIO, power modes with a sleeping and reduced screen, devices plugged in at any time, a terminal button, the offline translator, restart, switch off and the clock from a phone ([plan](https://github.com/johnusher/batnet/blob/main/docs/plans/2026-10-21-m1-plan-5a-hub-power-cut-kit.md))
- [ ] Try the power-cut kit with hands at the hub: a phone on the hub's own WiFi (QR, captive portal, the PIN when open), the router off for the fallback, the screen's sleep and reduced mode (B42), a screen and keyboard plugged in later, an LED on pin 11 and a button on pin 13, a speaker on the jack
- [ ] Meanwhile try a laptop on a cable straight into the hub
- [ ] Measure the bench hub's power with and without the screen, and in each power mode (the help says 3 to 4 W, the screen 1 to 3 W unmeasured)
- [ ] Next for the kit (suggested): a battery HAT that reports its charge, so the hub switches to low power by itself and shuts down cleanly before it is flat (the loft hub's UPS: [power budget](/research/power-budget.md)); the button on GPIO edge events (B43); a German voice for speech; a second WiFi stick for the hub's own WiFi alongside the home WiFi
- [x] Help in the panel, offline and printable: start here, no power, no network, troubleshooting, commands, radio words, good manners
- [ ] The bench hub's antenna outside and high on a short low-loss cable (RG-58 or thicker), or a 5 dBi collinear: inside, it hears nothing ([research](/research/meshtastic-presets-berlin.md))
- [ ] Maybe: Berlin's coordination channel "BerlinMesh" as a secondary channel on the hub, to read the community (its key is on the Chaos Mesh page; kept on the hub only, never in the repo)
- [ ] The second Meshstick on pi7 as a roving listener: a second meshtasticd (own `--fsdir`, `--port`, `--hwid`), each instance pinned to its stick by USB serial, receive only, stepping through the EU presets a few minutes each (they share 869.525 MHz and differ in spreading factor), logged beside the first

# Known problems to fix first

- [ ] B19 sessions and lockouts in SQLite, so a reboot keeps people signed in
- [ ] B16 `sameWiFiGroup` and `snapGroups` without rebuilding the groups per frame
- [ ] B15 connectivity in the simulator limited to 3 LoRa hops, as the flooding is
- [ ] B33 a gateway that lost every frame of a post, with no beacon heard since, still waits a quarter-hour (an entry's seq that leaves a gap could nudge too; costs beacons)

# Features to consider (from the 2026-10-15 review)

More, not yet picked or ordered, in [ideas](ideas.md) (2026-10-22 review: app, panel, settings, village, radio, power, offline AI hooks).

- [ ] Unread counts and the last-read mark kept on the phone (IndexedDB), so they survive a reload; `msg.read` entries per house later (spec §4)
- [ ] "Delivered to N of M houses" under a bubble, from the heartbeats' vectors (spec §8: "✓ 3/7 houses")
- [ ] The People tab's reach chips: "Call + text", "Text via LoRa", "Out of reach", from WiFi groups and the LoRa gate (spec §8)
- [ ] Admin health warnings: "Your WiFi group has no radio", "Radio near its airtime limit" (the bridge's duty meter and the gate know), "Hasn't heard from <house> for 2 hours"
- [ ] A Low-priority batch over LoRa: several Low posts in one envelope when a radio is near its limit (decision 0015; the simulator's festival case has airtime to spare for it)
- [ ] A scenario check for the admin's High limit and for ageing (a Low post waiting 40 min sends as Normal+2)
- [ ] Change PIN and admin PIN reset (spec §8; the log has `account.update`)

# Decisions waiting

- [ ] 0014 speech pipeline (draft): confirm after the speech bench test
- [ ] 0025 hub panel and mesh modes, 0026 no extra handshake for batnet radios (drafts, 2026-10-21): confirm
- [ ] 0028 the hub's own WiFi, 0029 power modes, 0030 offline translator, 0031 root actions through polkit and the console terminal (drafts, 2026-10-21): confirm
- [ ] Final project name (the repo is "batnet" for now)

# Done

- 2026-10-22: review of the whole repository (security, bugs, features): 30-odd fixes ([known problems](known-problems.md), Resolved), B50-B64 and L25 recorded, fuzz tests for every byte parser, [ideas](ideas.md) written
- 2026-10-21: the hub's log (every packet, samples, hellos every 5 min; on across power cuts) and the first field log, on a bike ride
- 2026-10-21: the first real hub on the bench ([pi7](/hardware/bench-pi7.md)): meshtasticd on a Meshstick, `batnetd -hub` with the control panel and mesh modes, the HDMI kiosk, autostart and watchdogs ([test](/tests/2026-10-21-pi7-hub-bring-up.md))

- 2026-10-15 (review): B21-B25 sign-in hardening; B13, B10, B14, B12, B15 (flooding), B17, B26-B31, B2-B4 fixed; decision [0024](/decisions/0024-ask-soon-for-what-was-lost.md) makes farms pass and a festival of pocket nodes pass
- 2026-10-15: one-hop heartbeats (0021); MIT licence (0011); GitHub Pages with the real engine
