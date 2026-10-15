---
type: Decision
title: "0008: One radio on LongFast by default; a second radio as an upgrade"
description: Both channels share one HAT on the LongFast preset (~100-200 batnet messages/hour per village); a second radio on a faster preset is optional.
tags: [lora, meshtastic, capacity]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-15T06:27:44Z }
---

# Context

A Meshtastic radio uses one modem preset for all its channels, so the private
channel runs at whatever speed public interop needs. See
[LoRa airtime](/research/lora-airtime-eu868.md).

# Decision

Default one HAT on LongFast. Optional second radio (USB Meshtastic board) on a
faster preset for the private channel only, ~5x capacity at shorter range.
