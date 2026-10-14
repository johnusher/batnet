---
type: Decision
title: "0006: A private batnet channel plus the public LongFast channel"
description: batnet traffic uses a private Meshtastic channel keyed from the village key; the public LongFast channel appears as a Neighbourhood group.
tags: [meshtastic, privacy, channels]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Decision

* **Private channel:** all batnet traffic; its key is derived from the village key.
  Plain Meshtastic users given the key see everyone-channel posts as text and can reply.
* **Public LongFast:** shown in the app as "Neighbourhood (public)". Everything
  heard there appears; posting there is deliberate, behind an "anyone can read this" warning.
