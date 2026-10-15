---
type: Issue List
title: Known problems
description: Open bugs, known limits, deferred review findings and things still to measure, each with where it was found and its status.
tags: [bugs, known-problems, review, backlog]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T07:44:39Z }
---

Every bug found, every known limit and every review finding not fixed at once
goes here, with where it came from. Move a line to **Resolved** (with the
commit or decision) when it is fixed; delete resolved lines after a month.
No Linear for batnet yet: this page is the list.

# Open bugs

| # | Problem | Found | Area |
|---|---|---|---|
| B1 | The WebAssembly bridge panics on a missing or non-number argument from the page, which stops the Go program; every later call fails | plan 2 review | `cmd/batnetwasm` |
| B2 | Restart in the real engine view doesn't catch a failed scenario load (unhandled rejection) | plan 2 review | `real.js` |
| B3 | A scripted post or power change that fails (house without power) is dropped without a report | plan 2 review | `internal/sim/scenario.go` |
| B4 | One unreadable or invalid scenario file stops `batnetd -sim` before the remaining files | plan 2 review | `cmd/batnetd` |
| B5 | Seqs at or above 2^63 are stored as negative numbers in SQLite; a range up to MaxUint64 returns nothing | plan 1 review | `ledger/sqlstore` |
| B6 | `ParseVector` reserves memory for the claimed count before checking the bytes are there | plan 1 review | `ledger/vector.go` |
| B7 | Repair goroutines aren't waited for when `Run` returns; closing the store can race in-flight writes (real-time path only) | plan 1 review | `meshsync` |

| B8 | LoRa-only farms (`scenarios/farms.json`, marked known failure): farms that can't hear each other collide; 1 of 50 posts missing, 1 late (27 min against 20) | plan 3 scenarios | `internal/lora`, `internal/sim/lora.go` |
| B9 | A gateway's LoRa duty meter is kept across a restart in the simulator; batnetd must store it, or a reboot could exceed the 10% limit | plan 3 | `internal/lora` |

# Known limits (by design, for now)

| # | Limit | Why / when it changes |
|---|---|---|
| L1 | A locked phone doesn't ring; batnet must be open (bedside mode) | browsers offline have no service worker or push ([0018](/decisions/0018-notifications-and-house-speaker.md)) |
| L3 | The simulator's WiFi model is not measured: ranges, the loss curve (2% + load²) and frame overhead are guesses | bench and field tests |
| L4 | In the simulator a WiFi channel is tracked per group; when groups split or merge the backlog resets, and loss ignores hop count | plan 2 review, minor |
| L5 | The model engine (JavaScript) and the real engine (Go) differ until accounts and the app join the real one | plans 4-5 |
| L8 | Everyone-channel posts don't yet go out as plain Meshtastic text for users holding the key (done-when check 3); public-channel text interop isn't built | plan 3b (meshtasticd client) |
| L9 | A signed entry costs about 100 bytes before its text on LoRa (origin key and signature): two packets for a 100-character post | a compact LoRa form, later |
| L6 | An author signing two different entries with the same seq would not be detected (vectors hold seqs, not content) | plan 1 review; needs a protocol decision |
| L7 | Sealed packets can be replayed (no sequence or associated data in `seal`); harmless today (dedup), to settle with the HTTPS transport | plan 1 review |

# Deferred minors (from reviews)

* SQLite gap filling runs one query per entry (plan 1).
* `OnEntry` can run concurrently in real time; undocumented. The first heartbeat's and `Publish`'s errors are ignored in `main.go` (plan 1).
* Heartbeats carry an address where the spec says node ID (plan 1).
* gitleaks runs in the pre-commit hook only, not in CI (plan 1).
* No meshsync test delivers seq 5 before 4; the forged-entry test uses a fixed sleep (plan 1).
* `hops()` and `channelFor` cost O(n²) per broadcast; the page runs `Judge` twice per frame (plan 2).
* No test of a split and merge under a saturated channel, or of a sender that stays dark (plan 2).

# To measure (bench)

* Microphone and WebRTC after the certificate warning on a real iPhone and Android ([0004](/decisions/0004-self-signed-https.md)).
* USB WiFi stick current; a pocket node on a power bank ([power budget](/research/power-budget.md)).
* Moonshine speed on Pi 4, Pi 5, Zero 2 W; raw vs denoised audio ([0014](/decisions/0014-speech-pipeline.md)).
* Captive-portal pop-up on each phone; desktop notifications past the warning; Android background tab; iOS Wake Lock.
* Two Meshsticks with meshtasticd: range, airtime, priority queue.

# Resolved

| Problem | Fixed by |
|---|---|
| Posts couldn't cross between WiFi groups | the LoRa bridge, plan 3, 2026-10-15 |
| Flooded heartbeats saturated a 40-house mesh; bang failed | [0021](/decisions/0021-one-hop-heartbeats.md), 2026-10-15 |
| Repair was free in the simulator; checks could pass falsely; bad scenarios could crash the engine | plan 2 review fixes, 2026-10-15 |
| Clock ordering after restart, author seq reuse, unbounded range requests, spoofed heartbeats | plan 1 review fixes, 2026-10-15 |
