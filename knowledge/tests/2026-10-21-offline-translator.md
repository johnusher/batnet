---
type: Test Report
title: "2026-10-21: offline translator on pi7"
description: Argos Translate's German-English models on CTranslate2 (int8, 2 threads) translate a short message in 0.5 to 1.1 s on a Pi 4, well; 464 MB on the card, about 400 MB of memory with both directions loaded.
tags: [bench, translation, ctranslate2, argos, pi4, memory]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
stale_after: 2027-04-21
sources:
  - id: argos-index
    resource: https://github.com/argosopentech/argospm-index
    title: Argos Translate package index
    author: argosopentech
---

# Setup

The [bench hub](/hardware/bench-pi7.md) (Pi 4, 2 GB), with the kiosk still
using about one core at the time. `deploy/pi/translate-setup.sh`: a venv with
ctranslate2 4.8.3, sentencepiece 0.2.2, numpy 2.5.4 and pyyaml 6.0.3 (wheels
only); Argos packages de→en and en→de (package 1.3, SentencePiece) from the
index[^argos-index]; the helper `batnet-translate` (JSON lines, models loaded on
first use, `int8`).

# Results

| | |
|---|---|
| On the card | venv 151 MB, each model 157 MB: 464 MB (a uk→en model adds 110 MB) |
| First install | 3 min 37 s; again: 0.5 s |
| Loading a model | 3.8 s from the card, 0.4 s cached |
| Memory | 26 MB idle, 406 MB with both models, 551 MB with three |
| CPU while translating | about 230 % (2 threads) |

Seconds per message, de→en, by threads (beam 2): 1 thread 0.73 to 1.13; **2
threads 0.56 to 0.81**; 3 threads 1.0 to 2.7; 4 threads 2.7 to 4.0 (they
fought the kiosk). Beam size barely mattered; float32 was twice as slow as
int8.

| German | English (0.5 to 1.1 s) |
|---|---|
| Wasser ist bei uns wieder da. | Water is back with us. |
| Hat jemand eine Ersatzbatterie für 12 V? | Does anyone have a spare battery for 12 V? |
| Die Straße an der Brücke ist durch einen Baum blockiert. | The road on the bridge is blocked by a tree. |
| Wir brauchen Hilfe, meine Mutter hat keine Medikamente mehr. Kann jemand Insulin bringen? | We need help, my mother has no more medication. Can someone bring insulin? |
| Treffen um 18 Uhr am Gemeindehaus, bringt Decken mit. | Meet at the church house at 6 p.m., bring blankets. ("church house" is wrong) |

English to German was as good ("meiner Mutter sind die Medikamente
ausgegangen"), but lost "spare" once. Emoji came through. Small faults: ":)"
lost its bracket, "torch" became "Fackel", "Ca. 12 V." split in two. The
Ukrainian model that has a SentencePiece tokenizer (1.4) was poor ("Water is
again").

# What it settles

* German↔English works offline on a Pi 4 at about a second a message:
  [decision 0030](/decisions/0030-offline-translator.md) holds.
* Two threads; `int8`; stop the helper when idle to give back 400 MB.

# Not recorded

* The models' licences: the Argos packages carry none; the index and Argos
  Translate are MIT or CC0. OPUS-MT based packages are CC-BY 4.0.
* Timing with the kiosk off (it was using a core).
