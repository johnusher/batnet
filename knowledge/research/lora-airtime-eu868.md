---
type: Research Note
title: LoRa airtime and capacity in EU868
description: How many batnet messages per hour a village can push over Meshtastic LongFast, and why text fits but voice does not.
tags: [lora, meshtastic, airtime, eu868, capacity]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: ttn-sf
    resource: https://thethingsnetwork.org/docs/lorawan/spreading-factors
    title: The Things Network - spreading factors
  - id: mesh-lora
    resource: https://meshtastic.org/docs/configuration/radio/lora/
    title: Meshtastic LoRa configuration
---

# Numbers

* SF7 / 125 kHz ≈ 5.5 kbit/s raw; each step up in spreading factor roughly halves the rate and adds range.[^ttn-sf]
* Meshtastic's EU default sits in the 869.4–869.65 MHz sub-band, which allows 10% duty cycle (other EU868 sub-bands: 1%).[^mesh-lora] Confirmed 2026-10-15 against Meshtastic's region table and the CEPT slice table: see [EU868 radio basics](/research/eu868-radio-basics.md).
* **LongFast** (SF11, 250 kHz): a ~150-byte packet ≈ **1.4 s airtime**: ~148 payload symbols of 8.2 ms plus Meshtastic's 16-symbol preamble (~165 ms). Earlier ~1.2 s left the longer preamble out; see [EU868 radio basics](/research/eu868-radio-basics.md).
* 10% duty = 360 s/hour per device; channel congestion is shared and Meshtastic backs off as utilisation rises, and every rebroadcast hop costs airtime again.
* **Working budget: ~100–200 batnet messages per hour per village.**

# What fits

| Payload | Rate | Over LoRa? |
|---|---|---|
| Speech as text (150 words/min) | ~120 bit/s, less compressed | yes |
| Codec2 700C voice | 700 bit/s continuous | short bursts at best |
| Opus voice | 6–24 kbit/s | no |

# Implications

* LoRa never carries the full log: [decision 0007](/decisions/0007-replicated-log.md).
* One modem preset per radio: [decision 0008](/decisions/0008-longfast-single-radio.md).
