---
type: Research Note
title: Voice bitrates - text, Codec2, Opus
description: Why speech-to-text-to-speech is the way to carry speech over LoRa, and why WiFi keeps real audio.
tags: [voice, codec2, opus, stt, tts]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: codec2
    resource: https://en.wikipedia.org/wiki/Codec_2
    title: Codec 2
  - id: lxst
    resource: https://pypi.org/project/lxst
    title: LXST - Reticulum real-time voice
---

# Rates

* Text of speech: ~15 characters/s ≈ 120 bit/s.
* Codec2: 700–3200 bit/s; 700C uses 40 ms frames (~7 bytes each).[^codec2] Note the FreeDV 700 modes are flagged as not actively maintained.
* Opus speech: ~6–24 kbit/s; LXST profiles span ~4.5–96 kbit/s and switch to Codec2 for LoRa.[^lxst]

# Design consequence

On the WiFi mesh, send real audio (Opus) with a transcript. Across LoRa, send the
transcript and speak it at the far end. LXST is CC BY-NC-ND licensed, so batnet
reuses the idea, not the code.
