---
type: Issue List
title: To do
description: What comes next for milestone 1, in order - plans, bench tests, decisions waiting, and the known problems to fix first.
tags: [todo, roadmap, milestone-1]
status: stable
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T13:45:00Z }
---

Tick items off here (move them to Done with the date). Bugs and limits live in
[known problems](known-problems.md); this page is the order of work. No Linear
for batnet yet.

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

- [ ] Two Pis + two Meshsticks: meshtasticd, range, airtime, priority queue, the bridge between two WiFi groups (one hub is up: set up the second Pi with `deploy/pi/deploy.sh`, copy `village.key`)
- [ ] The hub's HDMI screen and keyboard; real mesh mode with the USB WiFi stick
- [ ] Microphone and WebRTC after the certificate warning on a real iPhone and Android (decision 0004 depends on it)
- [ ] Captive-portal pop-up on each phone type; desktop notifications past the warning; Android background tab; iOS Wake Lock
- [ ] Power: the USB WiFi stick; a pocket node on a power bank
- [ ] Moonshine on Pi 4, Pi 5, Zero 2 W; raw vs denoised audio

# Known problems to fix first

- [ ] B19 sessions and lockouts in SQLite, so a reboot keeps people signed in
- [ ] B16 `sameWiFiGroup` and `snapGroups` without rebuilding the groups per frame
- [ ] B15 connectivity in the simulator limited to 3 LoRa hops, as the flooding is
- [ ] B33 a gateway that lost every frame of a post, with no beacon heard since, still waits a quarter-hour (an entry's seq that leaves a gap could nudge too; costs beacons)

# Features to consider (from the 2026-10-15 review)

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
- [ ] Final project name (the repo is "batnet" for now)

# Done

- 2026-10-21: the first real hub on the bench ([pi7](/hardware/bench-pi7.md)): meshtasticd on a Meshstick, `batnetd -hub` with the control panel and mesh modes, the HDMI kiosk, autostart and watchdogs ([test](/tests/2026-10-21-pi7-hub-bring-up.md))

- 2026-10-15 (review): B21-B25 sign-in hardening; B13, B10, B14, B12, B15 (flooding), B17, B26-B31, B2-B4 fixed; decision [0024](/decisions/0024-ask-soon-for-what-was-lost.md) makes farms pass and a festival of pocket nodes pass
- 2026-10-15: one-hop heartbeats (0021); MIT licence (0011); GitHub Pages with the real engine
