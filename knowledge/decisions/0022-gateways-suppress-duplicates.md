---
type: Decision
title: "0022: Gateways share the radio by suppression"
description: Every gateway in a WiFi group may send a post over LoRa after a short delay that differs per post and gateway; hearing it sent cancels the others. Supersedes 0017's assignment by hash.
tags: [lora, gateway, suppression, duty-cycle]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T08:00:37Z }
---

# Context

[Decision 0017](0017-gateways-share-the-radio.md) assigned each post to one of
its group's gateways by hashing its key over the gateways known from
heartbeats. With one-hop heartbeats ([0021](0021-one-hop-heartbeats.md)) a
gateway no longer hears every other gateway of its group.

# Decision

* A gateway puts a post on air only if, in the last 30 min, it heard a batnet
  radio outside its own WiFi group (batman-adv's originator table says who is in it).
* It waits `hash(post, gateway) mod 3 s` (plus 3 s if it has used over half its
  hourly airtime), then queues the post. Hearing the post on air from another
  gateway cancels its own send.
* Entries received over LoRa are never re-sent, except to answer a beacon.

Built in plan 3 and exercised in the simulator's village scenario (four
gateways, no duplicate sends). Status of 0017: superseded in its "assigned by
hash" rule; the principle (gateways share the sending) stands.
