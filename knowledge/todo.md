---
type: Issue List
title: To do
description: What comes next for milestone 1, in order - plans, bench tests, decisions waiting, and the known problems to fix first.
tags: [todo, roadmap, milestone-1]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T08:15:14Z }
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
- [ ] **Plan 3b: meshtasticd client on real radios - waits for the two Meshsticks; includes everyone-channel posts as plain Meshtastic text (done-when check 3) and public-channel interop (next, when the radios arrive)**
- [ ] Plan 5: Pi set-up, radios, kiosk, speaker - first boot, QR join, radio detection, batman-adv and hostapd, one-hop heartbeats on the mesh interface, cage + Chromium kiosk, speaker/button/LED, GPS time
- [ ] Plan 6: image, installer, CI hardening - the batnet.zip, Imager scripts, signed releases, gitleaks in CI

# Bench tests (need hardware)

- [ ] Two Pis + two Meshsticks: meshtasticd, range, airtime, priority queue, the bridge between two WiFi groups
- [ ] Microphone and WebRTC after the certificate warning on a real iPhone and Android (decision 0004 depends on it)
- [ ] Captive-portal pop-up on each phone type; desktop notifications past the warning; Android background tab; iOS Wake Lock
- [ ] Power: the USB WiFi stick; a pocket node on a power bank
- [ ] Moonshine on Pi 4, Pi 5, Zero 2 W; raw vs denoised audio

# Known problems to fix first

- [ ] B21-B25 sign-in hardening: exact argon2 parameters, lockout by account, constant-time sign-in, JSON-only, a stricter CSP
- [ ] B13 cap High at 8 (High DMs reach 9, reserved for Alerts)
- [ ] B10 open the LoRa gate only after a verified frame
- [ ] B14 a duty check that can fail (tally airtime from the air)
- [ ] B15 a 3-hop limit in the simulator's flooding and connectivity
- [ ] B12 evict the oldest partial message instead of refusing new ones

# Decisions waiting

- [ ] 0014 speech pipeline (draft): confirm after the speech bench test
- [ ] Final project name (the repo is "batnet" for now)

# Done

- 2026-10-15: one-hop heartbeats (0021); MIT licence (0011); GitHub Pages with the real engine
