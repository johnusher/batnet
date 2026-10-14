---
type: Decision
title: "0008: One radio on LongFast by default; a second radio as an upgrade"
description: Both channels share one HAT on the LongFast preset (~100-200 batnet messages/hour per village); a second radio on a faster preset is optional.
tags: [lora, meshtastic, capacity]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
---

# Status

Proposed in design section 2; awaiting approval.

# Context

A Meshtastic radio uses one modem preset for all its channels, so the private
channel runs at whatever speed public interop needs. See
[LoRa airtime](/research/lora-airtime-eu868.md).

# Proposal

Default one HAT on LongFast. Optional second radio (USB Meshtastic board) on a
faster preset for the private channel only, ~5x capacity at shorter range.
