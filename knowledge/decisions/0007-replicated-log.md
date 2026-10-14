---
type: Decision
title: "0007: A replicated append-only log, broadcast live and synced to fill gaps"
description: Accounts, groups and messages are immutable entries in a per-node log, broadcast on bat0 and reconciled with version vectors; LoRa carries a compact subset.
tags: [sync, log, store-and-forward]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
---

# Status

Proposed in design section 2; awaiting approval.

# Proposal

* Entries keyed `(origin node, seq)`, ordered by a hybrid logical clock, signed
  by the origin node (ed25519). State (approvals, membership) is derived by folding.
* WiFi mesh: UDP broadcast on `bat0` for live delivery; 5 s heartbeats; on a
  hash mismatch, version-vector exchange and range pulls over HTTP.
* All inter-node traffic sealed with the village key (XChaCha20-Poly1305).
* Every node keeps the full log, relays included.
* LoRa: only posts whose audience includes a LoRa-only house, directory cards and
  15-min compact vector beacons. One sender per WiFi partition (lowest gateway
  node ID). `lora.sent` entries map Meshtastic packet IDs to entry IDs so copies
  merge when partitions rejoin. See [LoRa airtime](/research/lora-airtime-eu868.md).
* Pi clocks are set from the first phone that logs in when obviously wrong.
