---
type: Decision
title: "0020: Photos: full over WiFi, a blurred preview over LoRa (M2)"
description: Photos are cleaned of metadata, resampled only above 500 KB, stored by content hash and fetched between houses over WiFi; LoRa carries the caption and a ~30-byte BlurHash.
tags: [photos, media, lora, privacy, m2]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:58:54Z }
verified: { by: human:johnusher, at: 2026-10-15T06:58:54Z }
---

# Decision (milestone 2, with voice: both need file storage and transfer between houses)

* The phone always **strips metadata** (EXIF, including location) and
  **resamples only if the photo is over 500 KB**, to fit under it (long edge
  1600 px, then lower quality).
* Stored on the house Pi by content hash; houses fetch photos from each other
  over the WiFi mesh when needed. Each house keeps up to a set amount (default
  2 GB) and drops the oldest full photos first, keeping previews.
* **Over LoRa:** the caption plus a BlurHash (~30 bytes) shown as a blurred
  preview: "Photo, visible once your house is linked by WiFi". A full 500 KB
  photo would need ~40 min of airtime against a 6 min/h limit.
* Later option: an admin may allow a tiny (~3 KB) photo over the radio for High
  messages (~18 s of airtime).
