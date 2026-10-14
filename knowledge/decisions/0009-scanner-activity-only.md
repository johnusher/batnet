---
type: Decision
title: "0009: The radio scanner reports activity, never content"
description: Scan mode records frequency, strength, time and a band-plan label, and does not decode or record speech.
tags: [scanner, sdr, legal, privacy]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Context

Listening to the content of transmissions not meant for you is restricted in
several countries, Germany included. See [SDR scanning](/research/sdr-scanning.md).

# Decision

The scanner (M4) and the radios-nearby page (M1) report **activity**: frequency,
signal strength, time, burst pattern, band-plan label. No demodulated audio is
stored or played. Meshtastic node lists are public metadata the radio already holds.
