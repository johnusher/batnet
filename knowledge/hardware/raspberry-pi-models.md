---
type: Hardware
title: Raspberry Pi models and their batnet roles
description: What each Pi (Zero W, Zero 2 W, 3, 4, 5) can do in batnet - house, relay, gateway, speech, video.
tags: [raspberry-pi, roles, hardware]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
---

# Roles by board

| Board | CPU / RAM | Go target | Best roles |
|---|---|---|---|
| Zero W | 1× ARM11 1 GHz / 512 MB | `GOARCH=arm GOARM=6` | relay, LoRa gateway, sensor |
| Zero 2 W | 4× Cortex-A53 1 GHz / 512 MB | `arm64` | relay on a pole, gateway, light speech |
| Pi 3 | 4× Cortex-A53 1.2–1.4 GHz / 1 GB | `arm64` | relay, gateway, small house |
| Pi 4 | 4× Cortex-A72 / 2–8 GB | `arm64` | **house**, speech, video (hardware H.264) |
| Pi 5 | 4× Cortex-A76 / 4–16 GB | `arm64` | **house**, best speech-to-text; no hardware H.264 encode |

Tried as hubs: a Pi 4 with 2 GB ([pi7](/hardware/bench-pi7.md)) and a Zero 2 W
([pz7](/hardware/bench-pz7.md)). Both took 200 phones on the app in every
power mode ([load test](/tests/2026-10-23-hub-load-test.md)); the Zero has no
kiosk and no translator (under 1 GB of memory). The Zero W cannot run the
set-up, which builds for 64-bit Pis only.

# Notes

* A house Pi needs two WiFi radios: [WiFi radios](/hardware/wifi-radios.md).
* Speech capabilities: [on-device speech](/research/on-device-speech.md).
