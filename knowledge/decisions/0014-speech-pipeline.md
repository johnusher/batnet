---
type: Decision
title: "0014: Speech pipeline - Moonshine, raw audio to the recogniser, the phone's voice (draft)"
description: Voice messages are trimmed on the phone, transcribed from the raw audio by Moonshine on the house Pi, denoised only for listening, and read aloud by the phone's own voice.
tags: [speech, stt, tts, denoise, vad, voice-messages, m2]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:00:04Z }
---

# Context

Milestone 2 sends voice messages. Over WiFi the audio travels; over LoRa only
its transcript fits. Research: [speech pipeline](/research/speech-pipeline.md).

# Decision (proposed)

1. **Phone (browser):** record, trim silence with Silero VAD (onnxruntime-web),
   upload. No app.
2. **House Pi, speech-to-text:** Moonshine streaming models, MIT licence, from
   the **raw** audio: Small on a Pi 5, Tiny on a Pi 4 (English and German).
   Whisper is 10× slower on short phrases and not used by default.
3. **No denoising before speech-to-text.** Published tests found enhancement
   raised error rates for Whisper and Parakeet. GTCRN (48K parameters, MIT)
   cleans only the copy people listen to over WiFi.
4. **The sender checks the transcript** before it goes out (it is what LoRa
   houses will hear).
5. **Reading aloud:** the receiving phone's own offline voice
   (`speechSynthesis`, on a tap); Piper on the Pi (CC0 or public-domain
   voices only, run as a separate process because of GPL) as the fallback that
   returns audio.
6. **One speech helper process** per house Pi, built on Moonshine's library and
   sherpa-onnx (Apache-2.0), talking Wyoming over a local socket to batnetd, which
   stays pure Go ([decision 0005](/decisions/0005-pure-go-core.md)).

# Open

Benchmarks on our boards (the research note lists seven, starting with raw vs
denoised clips from noisy rooms); whether Moonshine Tiny fits a Pi Zero 2 W
([decision 0012](/decisions/0012-pocket-node.md)); offline voices on iPhones and
Android phones.
