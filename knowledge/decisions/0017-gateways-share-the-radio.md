---
type: Decision
title: "0017: Gateways in a WiFi group share the radio sending"
description: Each entry is assigned to one of its WiFi group's online gateways by its key; a gateway near its duty limit hands over. Supersedes the one-sender rule in 0007.
tags: [lora, gateway, duty-cycle, sync]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:27:44Z }
verified: { by: human:johnusher, at: 2026-10-15T06:27:44Z }
---

# Context

[Decision 0007](0007-replicated-log.md) had one LoRa sender per WiFi group
(the lowest gateway node ID). The [village simulator](/research/village-sim-findings.md)
showed that sender hitting the 10% airtime limit in a busy village while other
gateways in the same group sat idle.

# Decision

* An entry that must go over LoRa is assigned to one of the group's online
  gateways by hashing its key over the gateways seen in heartbeats.
* Heartbeats carry each gateway's airtime used in the last hour; a gateway near
  its limit hands its queue to the next gateway.
* `lora.sent` entries stop two gateways sending the same entry.
* The 4-bit priority ([0015](0015-message-priority.md)) travels in the envelope
  so every gateway applies it.

Part of the approved [milestone 1 spec](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md), section 6.
