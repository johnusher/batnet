---
type: Research Note
title: Power budget per node and use case
description: Daily energy for pocket, house and relay nodes. The Pi's idle draw dominates; text messages cost almost nothing, speech-to-text a little.
tags: [power, battery, energy, solar, pi-zero, pi-4, pi-5]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:53:23Z }
stale_after: 2027-04-15T00:00:00Z
sources:
  - id: cnx-z2w
    resource: https://www.cnx-software.com/2021/12/09/raspberry-pi-zero-2-w-power-consumption/
    title: Raspberry Pi Zero 2 W power consumption (Otii Arc)
    author: org:cnx-software
  - id: pidramble
    resource: https://www.pidramble.com/wiki/benchmarks/power-consumption
    title: Pi Dramble power consumption benchmarks
  - id: cnx-pi5
    resource: https://www.cnx-software.com/2023/11/05/raspberry-pi-5-review-raspberry-pi-os-bookworm-benchmarks-power-consumption/
    title: Raspberry Pi 5 review - power consumption
    author: org:cnx-software
  - id: kowal
    resource: https://journals.pan.pl/Content/115187/PDF/23_2020.pdf
    title: Kowal and Staniec 2020 - energy of a Raspberry Pi 3 access point
  - id: sx1262
    resource: https://cdn.sparkfun.com/assets/6/b/5/1/4/SX1262_datasheet.pdf
    title: Semtech SX1261/2 datasheet
    author: vendor:semtech
  - id: goughlui
    resource: https://goughlui.com/2023/08/31/quick-review-teardown-veektomx-vt103-10000mah-38-5wh-power-bank/
    title: 10,000 mAh power bank test
  - id: moonshine-bench
    resource: https://www.mintlify.com/moonshine-ai/moonshine/models/benchmarks
    title: Moonshine benchmarks
    author: vendor:moonshine
---

# Inputs (5 V, headless)

| Part | Draw | Source |
|---|---|---|
| Pi Zero 2 W idle, WiFi on | 0.6–0.7 W | measured[^cnx-z2w][^pidramble] |
| Pi 4 idle | 2.7 W | measured[^pidramble] |
| Pi 5 idle, WiFi, cooler fitted | 2.4–3.0 W (3.0 W at the wall) | measured[^cnx-pi5] |
| Pi 5 full load (speech-to-text) | 7–10 W | measured[^cnx-pi5] |
| Built-in WiFi as hotspot | +0.1 W idle with clients, +0.4 W at full rate | measured on a Pi 3[^kowal] |
| USB WiFi stick (mesh) | **0.7 W assumed**, nobody has published MT7610U/MT7612U current | gap |
| SX1262 receive / transmit 22 dBm | 15 mW / 0.39 W | datasheet[^sx1262] |
| "10,000 mAh" power bank | 28–32 Wh usable at 5 V | measured[^goughlui] |

Per message (derived): one WiFi frame ≈ 1 mJ per node that sends or forwards
it; one LoRa text at LongFast ≈ 1.2 s × 0.39 W ≈ 0.5 J; one voice message
transcribed on a Pi 5 ≈ 6 s × 5 W extra ≈ 30 J with Whisper tiny, ≈ 5 J with
Moonshine (vendor latency figures[^moonshine-bench], no power measured).

# Use cases (per node, per day)

| Use case | Node | Wh/day | Messages' share | Runs on a 10,000 mAh bank |
|---|---|---|---|---|
| Festival camp, 300 texts a day, all by radio | Zero 2 W + radio | 18 | 0.04 Wh | ~1.5 days |
| Relay on a pole, WiFi mesh only | Zero 2 W | 16 | ~0 | ~2 days |
| House, WiFi mesh only, 5 houses, 1,000 texts a day | Pi 4 + WiFi stick | 84 | 0.001 Wh | ~8 h |
| Same, 10,000 texts a day | Pi 4 + WiFi stick | 84 | 0.014 Wh | ~8 h |
| House with radio, 1,000 texts (200 by radio) + 100 voice messages | Pi 5 + stick + radio | 92 | 0.9 Wh | ~8 h |

# Conclusions

* **The board's idle draw is the budget.** Message count barely registers: ten
  times more texts changes a house's day by hundredths of a watt-hour.
* A Pi 4/5 house needs mains, a car or leisure battery, or solar. A 12 V
  battery with about 300 Wh usable runs one for 3–4 days.
* A Zero 2 W is the battery node: 4–5× less energy than a Pi 4/5.
* Speech-to-text is the only per-message cost worth counting, and the model
  choice matters (Moonshine about 6× less than Whisper per message, by latency).
* Phones are not counted here; their own batteries are the usual limit.

# To measure

USB WiFi stick current in mesh mode; Zero 2 W as hotspot with phones attached;
Pi 5 during transcription; a full pocket node on a power bank, end to end.
