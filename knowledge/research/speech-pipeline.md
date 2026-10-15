---
type: Research Note
title: Speech pipeline per board - VAD, denoiser, speech-to-text, text-to-speech
description: Which voice-activity detector, denoiser, speech-to-text and text-to-speech model to run on a Pi 5, Pi 4, Zero 2 W and in the phone browser, with the measured numbers, who measured them, and licences.
tags: [stt, tts, vad, denoise, moonshine, whisper, piper, sherpa-onnx, gtcrn, wyoming]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:57:00Z }
stale_after: 2027-04-15T00:00:00Z
sources:
  - id: ms-models
    resource: https://github.com/moonshine-ai/moonshine/blob/main/docs/models/available-models.md
    title: Moonshine - available models (params, WER, licence)
    author: vendor:moonshine
  - id: ms-vs-whisper
    resource: https://github.com/moonshine-ai/moonshine/blob/main/docs/moonshine-vs-whisper.md
    title: Moonshine vs Whisper (latency table incl. Raspberry Pi 5)
    author: vendor:moonshine
  - id: ms-bench
    resource: https://github.com/moonshine-ai/moonshine/blob/main/docs/using/benchmarks.md
    title: Moonshine benchmarks - how the latency numbers were measured
    author: vendor:moonshine
  - id: ms-licence
    resource: https://github.com/moonshine-ai/moonshine/blob/main/LICENSE
    title: Moonshine licence (MIT, plus Community Licence list)
  - id: ms-tts
    resource: https://github.com/moonshine-ai/moonshine/blob/main/docs/using/text-to-speech.md
    title: Moonshine text to speech
  - id: dutta25
    resource: https://arxiv.org/abs/2507.14451
    title: Adapting Whisper for lightweight ASR of children on edge devices (Raspberry Pi 5 measurements)
  - id: whispercpp
    resource: https://github.com/ggml-org/whisper.cpp
    title: whisper.cpp README (memory table)
  - id: vosk
    resource: https://alphacephei.com/vosk/models
    title: Vosk model list (size, WER, licence)
    author: vendor:alphacephei
  - id: parakeet
    resource: https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3
    title: NVIDIA Parakeet TDT 0.6B v3 model card
  - id: canary
    resource: https://huggingface.co/nvidia/canary-180m-flash
    title: NVIDIA Canary 180M Flash model card
  - id: kroko
    resource: https://huggingface.co/Banafo/Kroko-ASR
    title: Kroko ASR - community models licence
  - id: sherpa
    resource: https://github.com/k2-fsa/sherpa-onnx
    title: sherpa-onnx README (features, platforms, WASM demos)
  - id: sherpa-rel
    resource: https://github.com/k2-fsa/sherpa-onnx/releases/tag/v1.13.8
    title: sherpa-onnx v1.13.8 release (prebuilt linux-aarch64 and arm-gnueabihf binaries)
  - id: sherpa-go
    resource: https://github.com/k2-fsa/sherpa-onnx-go-linux
    title: sherpa-onnx Go bindings (cgo)
  - id: sherpa-tts-rtf
    resource: https://github.com/k2-fsa/sherpa/blob/master/docs/source/onnx/tts/pretrained_models/rtf.rst
    title: sherpa-onnx - TTS real-time factors on a Raspberry Pi 4 Model B
  - id: sherpa-se
    resource: https://github.com/k2-fsa/sherpa/blob/master/docs/source/onnx/speech-enhancement/models.rst
    title: sherpa-onnx speech enhancement - GTCRN
  - id: sherpa-dpdfnet
    resource: https://github.com/k2-fsa/sherpa/blob/master/docs/source/onnx/speech-enhancement/dpdfnet.rst
    title: sherpa-onnx speech enhancement - DPDFNet
  - id: sherpa-tenvad
    resource: https://github.com/k2-fsa/sherpa/blob/master/docs/source/onnx/vad/ten-vad.rst
    title: sherpa-onnx - TEN VAD
  - id: sherpa-supertonic
    resource: https://github.com/k2-fsa/sherpa/blob/master/docs/source/onnx/tts/supertonic.rst
    title: sherpa-onnx - Supertonic TTS
  - id: wyoming
    resource: https://github.com/OHF-Voice/wyoming
    title: Wyoming protocol
  - id: wyoming-fw
    resource: https://github.com/OHF-Voice/wyoming-faster-whisper
    title: wyoming-faster-whisper (backends incl. sherpa, Parakeet, Kroko)
  - id: wyoming-docker
    resource: https://hub.docker.com/r/rhasspy/wyoming-piper/tags
    title: Wyoming Docker images (amd64 and arm64 only)
  - id: piper
    resource: https://github.com/OHF-Voice/piper1-gpl
    title: Piper (piper1-gpl, GPL-3.0, embeds espeak-ng)
  - id: piper-voices
    resource: https://huggingface.co/rhasspy/piper-voices
    title: Piper voices and their MODEL_CARD licences
  - id: lessac
    resource: https://www.cstr.ed.ac.uk/projects/blizzard/2013/lessac_blizzard2013/license.html
    title: Blizzard 2013 Lessac research licence
  - id: espeak
    resource: https://github.com/espeak-ng/espeak-ng
    title: eSpeak NG (GPL-3.0, formant synthesis, 100+ languages)
  - id: kokoro
    resource: https://huggingface.co/hexgrad/Kokoro-82M
    title: Kokoro-82M model card
  - id: supertonic
    resource: https://huggingface.co/Supertone/supertonic-3
    title: Supertonic 3 model card
  - id: gtcrn
    resource: https://github.com/Xiaobin-Rong/gtcrn
    title: GTCRN (ICASSP 2024) repository
  - id: ulunas
    resource: https://github.com/Xiaobin-Rong/ul-unas
    title: UL-UNAS (IEEE TASLP 2026) repository
  - id: lisennet
    resource: https://arxiv.org/abs/2409.13285
    title: LiSenNet - lightweight sub-band and dual-path speech enhancement (includes FSPEN figures)
  - id: dfn2
    resource: https://arxiv.org/abs/2205.05474
    title: DeepFilterNet2 - real-time speech enhancement on embedded devices
  - id: dfn
    resource: https://github.com/Rikorose/DeepFilterNet
    title: DeepFilterNet repository
  - id: rnnoise
    resource: https://github.com/xiph/rnnoise
    title: RNNoise
  - id: dtln
    resource: https://github.com/breizhn/DTLN
    title: DTLN repository (Raspberry Pi 3 B+ timings)
  - id: se-medical
    resource: https://arxiv.org/abs/2512.17562
    title: When de-noising hurts - speech enhancement and modern ASR
  - id: se-sam
    resource: https://arxiv.org/abs/2603.04710
    title: When audio separation hurts zero-shot ASR (SAM-Audio with Whisper)
  - id: se-artifacts
    resource: https://arxiv.org/abs/2201.06685
    title: How bad are artifacts? Speech enhancement errors and ASR
  - id: silero
    resource: https://github.com/snakers4/silero-vad
    title: Silero VAD
  - id: tenvad
    resource: https://github.com/TEN-framework/ten-vad
    title: TEN VAD (licence with extra conditions)
  - id: mdn-local
    resource: https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesisVoice/localService
    title: MDN - SpeechSynthesisVoice.localService
  - id: mdn-ns
    resource: https://developer.mozilla.org/en-US/docs/Web/API/MediaTrackConstraints/noiseSuppression
    title: MDN - MediaTrackConstraints.noiseSuppression
  - id: android-tts
    resource: https://support.google.com/accessibility/android/answer/6006983
    title: Android text-to-speech output settings
---

# Who measured what

Almost no number here was taken on our boards. Each figure is labelled:
**V** = the model's own authors or vendor, **R** = the runtime project
(sherpa-onnx) measuring other people's models, **I** = independent.
**We found no published speech numbers for a Pi Zero 2 W**; the Zero 2 rows are
our estimates and say so.

The latency budget is loose. A voice message is store-and-forward: over LoRa a
10-second message becomes about 150 characters, one packet, and the radio may
send one packet per ~10 s anyway ([LoRa airtime](/research/lora-airtime-eu868.md)).
Transcribing a 10 s clip in 10–20 s is acceptable, so slower-than-real-time
models are allowed on small boards.

# Recommended pipeline per board

| Board | VAD (trim silence) | Denoise | Speech-to-text | Text-to-speech |
|---|---|---|---|---|
| Phone browser | Silero via onnxruntime-web, before upload | none before STT; optional GTCRN (sherpa-onnx WASM) for the audio copy people listen to | not needed (Pi does it) | `speechSynthesis` with a `localService` voice; else play Pi-made audio |
| Pi 5 | Silero (sherpa-onnx) or Moonshine's built-in segmenter | none before STT; GTCRN for the WiFi audio copy | Moonshine **Small Streaming** (en 7.84 %, de 7.5 %), Tiny if busy | Piper medium (CC0/PD voices) |
| Pi 4 | Silero | as Pi 5 | Moonshine **Tiny Streaming** (en 12.0 %, de 12.0 %) | Piper medium, 2–4 threads |
| Zero 2 W (512 MB) | Silero | none (DTLN or GTCRN fit if wanted) | Moonshine Tiny Streaming or Vosk small, run after recording; benchmark both | phone's own voice; espeak-ng as fallback |

Other languages: Moonshine streaming covers en, de, es, ar, ja, zh, vi, tl;
for the rest, Whisper base/small (Pi 5) or Parakeet v3 (25 European languages,
unmeasured on a Pi).

# Speech-to-text

| Model | Size | Speed on a Pi | WER | Languages | Licence | Runtime |
|---|---|---|---|---|---|---|
| Moonshine Tiny Streaming | 34 M params, int8 `.ort` | Pi 5: 237 ms from end of phrase to text (V)[^ms-vs-whisper] | en 12.00 % (Open ASR avg), de 12.0 % (FLEURS+MLS) (V)[^ms-models] | en, de, es, ar, ja, zh, vi, tl | MIT[^ms-licence] | C++ core on ONNX Runtime, arm64 Linux libs, Python, WASM |
| Moonshine Small Streaming | 123 M | Pi 5: 527 ms (V); "room to spare on a stock Pi 5" (V) | en 7.84 %, de 7.5 % (V) | en, de, es, ja | MIT | as above |
| Moonshine Medium Streaming | 245 M | Pi 5: 802 ms; vendor advises active cooling (V) | en 6.65 % (V) | en | MIT | as above |
| Whisper tiny | 39 M; whisper.cpp 75 MiB disk, ~273 MB RAM[^whispercpp] | Pi 5: 5.9 s per phrase (V, Moonshine's comparison); fine-tuned tiny.en RTF 0.23–0.39 (I)[^dutta25] | en 12.81 % (Open ASR avg, V) | 99, weak outside the big ones | MIT | whisper.cpp (C++), faster-whisper (Python, CTranslate2) |
| Whisper base / small | 74 M / 244 M; ~388 / ~852 MB RAM | small: 10.4 s per phrase on Pi 5 (V); small throttled a Pi 5 at 80 °C (I) | small en 8.59 % (V) | 99 | MIT | as above |
| Vosk small en / de | 40 MB / 45 MB | "for Android and RPi" (V); no measurements found | en 9.85 % LibriSpeech clean; de 13.75 % Tuda-de, 30.67 % podcast (V)[^vosk] | 20+ | Apache-2.0 (small en/de) | Kaldi C++, `wyoming-vosk` |
| Kroko streaming zipformer | small transducer | none found | none found | en, de, es, fr | CC-BY-SA community models[^kroko] | sherpa-onnx |
| Canary 180M Flash | 182 M | RTF 0.12 on the sherpa-onnx developer's desktop, not a Pi (R) | see card[^canary] | en, de, es, fr | CC-BY-4.0 | sherpa-onnx int8 |
| Parakeet TDT 0.6B v3 | 600 M | none on a Pi | see card[^parakeet] | 25 European incl. de | CC-BY-4.0 | sherpa-onnx int8; `wyoming-faster-whisper` sherpa backend |

Notes:

* Moonshine's Pi 5 column predates a build fix and "reads pessimistically" (V).[^ms-bench]
  The non-streaming Moonshine models for ar, ja, ko, zh, es, uk, vi are under a
  **non-commercial** Community Licence; every streaming model and English
  Tiny/Base are MIT.[^ms-licence] Use streaming models only.
* sherpa-onnx ships Moonshine Tiny/Base (non-streaming, en and Asian languages),
  not Moonshine's German streaming models; German Moonshine means Moonshine's
  own library.
* Whisper's 30 s window and lack of caching make it slow for short phrases on
  a Pi (V, a competitor's claim, consistent with the I figure).
* Distil-Whisper checkpoints are English-only and larger than tiny/base; not
  useful here.

# Text-to-speech

| Engine / voice | Size | Pi 4 RTF, 1 → 4 threads (R)[^sherpa-tts-rtf] | Languages | Licence |
|---|---|---|---|---|
| Piper medium (VITS, 22.05 kHz) | 61–75 MB | 0.79 → 0.36 (lessac, libritts_r) | ~35 incl. de, en | engine GPL-3.0[^piper]; voices per card |
| Matcha (en, ljspeech) | 71 MB | 0.94 → 0.41 | en, zh | per model |
| Kokoro-82M v1.0 | 311 MB | 7.6 → 3.2 (slower than real time) | 8, **no German**[^kokoro] | Apache-2.0 |
| Supertonic 3 | ~99 M params | none published | 31 incl. de[^supertonic]; in sherpa-onnx[^sherpa-supertonic] | OpenRAIL-M weights (use rules), project archived |
| espeak-ng | small (formant synthesis) | trivial | 100+[^espeak] | GPL-3.0; robotic but runs anywhere |
| Phone `speechSynthesis` | 0 | phone CPU | whatever the phone has | the phone's |

* Piper voice licences are per voice.[^piper-voices] Safe picks: German
  `thorsten` (CC0, low/medium/high), English `ljspeech` (public domain),
  `libritts_r` (CC BY 4.0). `lessac` comes from the Blizzard 2013 data under a
  **research licence**,[^lessac] and several voices (thorsten-high, en_GB alba)
  were fine-tuned from lessac; treat those as tainted until checked.
* Piper (and sherpa-onnx's Piper models) phonemize with espeak-ng, which is
  GPL-3.0. Run it as a separate helper process; batnetd stays MIT
  ([decision 0011](/decisions/0011-mit-licence.md)).
* Moonshine also has a TTS module (Kokoro and Piper voices, own G2P, 20 locales
  incl. de-de).[^ms-tts] Quality and licences per voice not checked.
* Browser voices: `SpeechSynthesisVoice.localService` says whether a voice is
  on-device or a remote service;[^mdn-local] filter on it. Android uses the
  installed TTS engine and its downloaded voice data;[^android-tts] iOS voices
  are reported to be on-device (not verified for Safari). Whether every phone has an offline German and English voice,
  and whether Safari speaks without a tap (reports say it needs a user gesture),
  must be tested. Design for "tap to hear", with a Pi-rendered Opus fallback.

# Denoisers

| Model | Params / compute | Rate | Quality (authors) | CPU cost measured | Licence |
|---|---|---|---|---|---|
| GTCRN | 48.2 K, 33 MMAC/s; 523 KB ONNX[^sherpa-se] | 16 kHz | VCTK-DEMAND PESQ 2.87, STOI 0.940; DNS3 DNSMOS OVRL 2.70 (V)[^gtcrn] | streaming RTF 0.07 on an i5-12400 (V); RTF 0.07 offline in sherpa-onnx, 1 thread on a Mac (R) | MIT |
| UL-UNAS | GTCRN's successor | 16 kHz | "outperforms" same-cost models (V)[^ulunas] | streaming ONNX code since 2026-02 | see repo |
| LiSenNet / FSPEN | 37 K, 56 MMAC/s / 79 K, 89 MMAC/s | 16 kHz | PESQ 3.07 / 2.97 (V)[^lisennet] | none | see paper |
| RNNoise | 0.06 M, 0.04 GMAC/s | 48 kHz | PESQ 2.33 (measured by DFN2 authors)[^dfn2] | RTF 0.03 on an i5-8250U | BSD[^rnnoise] |
| DTLN | < 1 M | 16 kHz | DNS challenge entry | **Pi 3 B+ (Cortex-A53, the Zero 2 W's core): 2.2 ms per 8 ms block, quantized TFLite (V)**[^dtln] | MIT |
| DeepFilterNet2/3 | 2.31 M, 0.36 GMAC/s | 48 kHz only | PESQ 3.08 (V) | **Pi 4 RTF 0.42 (V)**[^dfn2] | MIT/Apache; repo idle since 2024-10[^dfn] |
| DPDFNet | 2.3–3.5 M, 0.36–4.4 GMAC | 8/16/48 kHz | see paper | none | in sherpa-onnx[^sherpa-dpdfnet] |

**Denoising before speech-to-text hurts modern models.** Enhancement raised
semantic WER in all 40 model/condition pairs (Whisper, Parakeet and others;
+1.1 to +46.6 points absolute) in one study,[^se-medical] and raised Whisper
WER/CER in every configuration in another.[^se-sam] Both are 2025–26 preprints.
The mechanism: enhancement artifacts, not leftover noise, cause the ASR errors;
mixing some of the noisy signal back in ("observation adding") helps.[^se-artifacts]
So: feed the **raw** clip to STT, and denoise only the copy people listen to.
The browser's own `noiseSuppression` constraint is processing too; request
`false` for the STT copy where supported (MDN: not Baseline).[^mdn-ns]

In the browser: GTCRN runs as sherpa-onnx WASM (a demo Space exists);[^gtcrn]
RNNoise has several WASM ports. A voice message is recorded first, so the
clip can be denoised as a file; no AudioWorklet is needed.

# Voice activity detection

| VAD | Cost | Licence | Browser |
|---|---|---|---|
| Silero VAD | ~2 MB; < 1 ms per 30 ms chunk on one thread (V); 8/16 kHz[^silero] | MIT | yes, onnxruntime-web (`ricky0123/vad`) |
| TEN VAD | 126–324 KB ONNX; RTF ~0.01 on x86 (V); 16 kHz[^sherpa-tenvad] | Apache-2.0 **plus "may not compete with Agora's offerings"**[^tenvad] | yes, WASM |
| WebRTC VAD | negligible (classic GMM) | BSD (WebRTC) | WASM ports |

TEN claims better precision and faster end-of-speech than Silero and WebRTC
(V). Its licence condition is a risk for a communications app: use Silero.

# One helper process

**sherpa-onnx** (Apache-2.0) bundles Silero/TEN VAD, GTCRN and DPDFNet
denoising, STT (Whisper, Moonshine, zipformer incl. Kroko German, Canary,
Parakeet) and TTS (Piper/VITS, Matcha, Kokoro, Kitten, Supertonic) in one
library, with WASM builds.[^sherpa] The v1.13.8 release has prebuilt
`linux-aarch64` and `arm-gnueabihf` binaries, including `sherpa-onnx-vad`,
`-offline-denoiser`, `-offline-tts` and online/offline **websocket servers**.[^sherpa-rel]
Its Go bindings use cgo,[^sherpa-go] which batnetd cannot ([decision 0005](/decisions/0005-pure-go-core.md)):
call the binaries or a websocket server instead. Gap: no German Moonshine
streaming model in it.

**Wyoming** is JSONL headers plus PCM over TCP, Unix socket or stdio, with no
authentication;[^wyoming] a pure-Go client is a few hundred lines. Bind it to
localhost only. `wyoming-faster-whisper` now picks a backend per language
(faster-whisper, sherpa with Parakeet v3 or Kroko streaming de/en/es/fr) and
runs offline with `--local-files-only`;[^wyoming-fw] `wyoming-piper` serves
Piper. Images exist for amd64 and arm64 only[^wyoming-docker] (a Zero 2 W needs
64-bit Raspberry Pi OS), and they are Python services, heavy for 512 MB.

Proposal: one batnet speech helper speaking Wyoming on a local socket, backed by
Moonshine (STT) and sherpa-onnx (VAD, denoise, Piper TTS), so batnetd talks one
protocol and the backends can change.

# Zero 2 W: what fits (our estimates, unmeasured)

* RAM: whisper.cpp tiny needs ~273 MB, too much beside the hotspot and batnetd.
* Moonshine Tiny Streaming is 34 M params, ~35–40 MB at int8 and memory-mapped.
  The A53 at 1 GHz is perhaps 4–6× slower than the Pi 5's A76, so expect
  1–1.5 s per phrase: fine for store-and-forward. Vosk small (45 MB) is the
  fallback, worse on German (13.75 % vs 12.0 %, different test sets).
* DTLN is measured real-time on the same core type; GTCRN needs ~11× fewer
  MACs than DeepFilterNet2, which runs at RTF 0.42 on a Pi 4.
* Let the phone speak; espeak-ng on the Pi if a speaker is attached.

This revises the pocket-node draft ([0012](/decisions/0012-pocket-node.md)),
which assumed speech-to-text is too heavy for a Zero 2 W: it may fit.

# Open questions to benchmark

1. Moonshine Tiny/Small Streaming latency, RTF and peak RSS on Pi 4, Pi 5 and
   Zero 2 W, English and German, from our own recordings (outdoor, wind, crowd).
2. WER raw vs GTCRN-denoised vs browser `noiseSuppression` on, same clips.
3. Piper medium vs low, thorsten and ljspeech, RTF on Zero 2 W and Pi 4.
4. Offline voices: which phones (iOS, Android brands) have a local en and de
   voice, in airplane mode; does Safari speak without a tap.
5. Parakeet v3 and Canary 180M on a Pi 5: speed and RAM.
6. Supertonic 3 on a Pi 5: speed, German quality; OpenRAIL-M fit with MIT.
7. Vosk small vs Moonshine Tiny on the Zero 2 W, German.
