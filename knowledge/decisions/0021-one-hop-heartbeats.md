---
type: Decision
title: "0021: Heartbeats go one hop"
description: Each node's 5 s heartbeat reaches its direct neighbours only; differences are repaired hop by hop. Flooded heartbeats saturated a 40-house mesh.
tags: [sync, heartbeat, wifi, batman-adv, scaling]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T07:44:11Z }
verified: { by: human:johnusher, at: 2026-10-15T07:44:11Z }
---

# Context

Spec section 5 broadcast heartbeats on `bat0`, which batman-adv floods to every
node: the cost grows with the square of the mesh. Running the real code in the
[village simulator](/research/village-sim-findings.md): heartbeats alone kept
the channel 41% busy at 40 houses and 100% at 80, and the *bang* scenario (40
houses all posting within 30 s) failed: median delivery 41 s, 45 posts lost.

# Decision

* A heartbeat goes **one hop**: to direct neighbours only (on a Pi: on the mesh
  interface below `bat0`, or unicast to batman-adv's neighbours). A neighbour
  whose hash differs repairs from the sender; changes spread hop by hop.
* New entries are still broadcast on `bat0` (flooded) for live delivery.
* `meshsync.Link` gains `BroadcastLocal`.

# Result

Simulated, same scenarios: heartbeats alone 1–2% of the channel at 10–80 houses;
bang passes (median 10 ms, worst 48 s, nothing lost).
