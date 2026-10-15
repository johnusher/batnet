---
type: Research Note
title: On-device speech-to-text and text-to-speech on Raspberry Pis
description: Which Pi can run which speech models, from Moonshine and Whisper on the Pi 5 down to nothing on the Zero W.
tags: [stt, tts, moonshine, whisper, vosk, piper]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: moonshine
    resource: https://moonshine-voice.readthedocs.io/en/latest/moonshine-vs-whisper/
    title: Moonshine vs Whisper
    author: vendor:moonshine
  - id: fastio
    resource: https://fast.io/resources/openclaw-raspberry-pi-speech-recognition-offline-voice-agent/
    title: Offline speech recognition on Raspberry Pi
  - id: dcentral
    resource: https://d-central.tech/local-voice-ai-models/
    title: Local voice-AI model database
---

Superseded in detail by the [speech pipeline](/research/speech-pipeline.md) note (2026-10-15), which recommends Moonshine over Whisper.

# By board (mostly vendor figures; nobody has published Zero 2 measurements)

| Board | Speech-to-text | Text-to-speech | Notes |
|---|---|---|---|
| Pi 5 | Moonshine streaming ~0.2–0.8 s latency (vendor)[^moonshine]; Whisper tiny ~2–3 s per 10 s clip via faster-whisper[^fastio] | Piper real-time | no hardware H.264 encoder |
| Pi 4 | Moonshine tiny, Whisper tiny | Piper real-time | hardware H.264: best video node |
| Pi 3 / Zero 2 W (512 MB) | Vosk small, streaming[^dcentral] | Piper low voices, maybe | memory-bound |
| Zero W (ARMv6, 1 core) | none | none | text, sensors, relay |

# Design consequences

* Speech-to-text runs on the house Pi (Pi 4/5), as a helper process: [decision 0005](/decisions/0005-pure-go-core.md).
* Phones speak received text with their built-in offline voices (`speechSynthesis`); Piper serves Pi speakers.
* Benchmark on our own boards before M2 and record the results here.
