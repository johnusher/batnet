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
| B2 | Restart in the real engine view doesn't catch a failed scenario load (unhandled rejection) | plan 2 review | `real.js` |
| B3 | A scripted post or power change that fails (house without power) is dropped without a report | plan 2 review | `internal/sim/scenario.go` |
| B4 | One unreadable or invalid scenario file stops `batnetd -sim` before the remaining files | plan 2 review | `cmd/batnetd` |
| B5 | Seqs at or above 2^63 are stored as negative numbers in SQLite; a range up to MaxUint64 returns nothing | plan 1 review | `ledger/sqlstore` |
| B6 | `ParseVector` reserves memory for the claimed count before checking the bytes are there | plan 1 review | `ledger/vector.go` |
| B7 | Repair goroutines aren't waited for when `Run` returns; closing the store can race in-flight writes (real-time path only) | plan 1 review | `meshsync` |

| B8 | LoRa-only farms (`scenarios/farms.json`, marked known failure): farms that can't hear each other collide (~5,400 collisions in 90 min); 1 of 50 posts missing, 8 late against 20 min; busiest radio at 88% (after plan 4b's hold on entries another gateway started) | plan 3 scenarios and review | `internal/lora`, `internal/sim/lora.go` |
| B9 | A gateway's LoRa duty meter is kept across a restart in the simulator; batnetd must store it, or a reboot could exceed the 10% limit | plan 3 | `internal/lora` |

| B10 | `remoteAt` (the 30-min gate for sending over LoRa) is set by any frame with a valid header, before its payload is verified: garbage can open the gate | plan 3 review m1 | `internal/lora/bridge.go` |
| B11 | Another village on the same channel key: its beacons are answered and its signed entries injected; one short beacon draws up to 8 entries back (a cheap way to drain a gateway's airtime). Relies on Meshtastic's channel key; rate-limit answers per requester | plan 3 review m2 | `internal/lora` |
| B12 | 64 pending partial messages block new multi-fragment ones for 10 min; evict the oldest instead | plan 3 review m3 | `internal/lora/frame.go` |
| B13 | A High direct message is level 9, which decision 0015 reserves for Alerts; cap High at 8 | plan 3 review m4 | `internal/prio` |
| B14 | The duty check reads the meter that enforces the limit, so it can't fail; tally airtime per radio from the air's transmissions | plan 3 review m5 | `internal/sim/report.go` |
| B15 | The simulator's flooding has no hop limit (Meshtastic's default is 3) and connectivity counts LoRa paths of any length: farm-like results are optimistic | plan 3 review m6 | `internal/sim` |
| B16 | `sameWiFiGroup` rebuilds WiFi groups on every frame; `heard`/`seen` maps grow without bound; `snapGroups` is O(radios²) | plan 3 review m7, m8 | `internal/sim`, `internal/lora` |
| B17 | Scenario `meshtastic` users aren't validated (count, names clashing with houses) | plan 3 review m8 | `internal/sim/scenario.go` |
| B18 | Short IDs (4-byte prefixes) can be ground to collide; a tie makes compact vectors order by map and runs stop repeating | plan 3 review m9 | `internal/lora/frame.go` |

| B19 | Sessions and PIN lockouts live in memory: a reboot signs everyone out, and a guesser can move to another house to reset the lockout | plan 4a | `internal/auth`, `internal/web` |
| B20 | Two people with the same name and the same PIN at one house can't be told apart at sign-in | plan 4a | `internal/web/api.go` |

| B21 | A PIN hash from the log sets its own argon2 cost (up to 1 GiB): a rogue node's account makes each sign-in allocate that much; require the exact parameters | plan 4a review | `internal/auth` |
| B22 | The lockout is keyed by name: anyone can keep a named person (or "Admin") locked out; made-up names grow the table | plan 4a review | `internal/web/api.go` |
| B23 | Sign-in timing shows whether a name exists (argon2 runs only on a match); run one dummy check when nothing matches | plan 4a review | `internal/web/api.go` |
| B24 | Sign-in and sign-up accept any content type (login CSRF from a text/plain form); require application/json | plan 4a review | `internal/web/api.go` |
| B25 | The CSP lacks `frame-ancestors 'none'; base-uri 'none'; form-action 'self'` | plan 4a review | `internal/web/server.go` |
| B26 | No limit on event streams per account | plan 4a review | `internal/web/api.go` |
| B27 | A message written while the outbox is sending waits for the next round (up to 15 s) | plan 4b review m7 | `internal/web/app/app.js` |
| B28 | An Alert that arrives in bedside mode only sounds: the dim screen doesn't show its text | plan 4b review m9 | `internal/web/app/app.js` |
| B29 | Errors from settings, approve, deny, join and leave are swallowed: the button just does nothing | plan 4b review m10 | `internal/web/app/app.js` |
| B30 | In the simulator a phone still works at a house with no power, so its posts later seem to travel from a dead house (spec §13: those phones lose the app) | plan 4b review m11 | `internal/sim/api.go` |
| B31 | A simulated house with no people posts with an empty author, which the state drops | plan 4b review m14 | `internal/sim/village.go` |

# Known limits (by design, for now)

| # | Limit | Why / when it changes |
|---|---|---|
| L10 | The app has no voice messages, photos or location sharing on a map yet | milestone 2 ([0019](/decisions/0019-location.md), [0020](/decisions/0020-photos.md)) |
| L12 | A house remembers the outbox's message ids (clientIds) for a day, in memory: a retry after the house Pi restarts, or at another house, can post twice | plan 4b review (ids kept off the radio) |
| L11 | The simulator's phone: its bedside clock shows the real time, not the village's; the page never closes the sessions and live streams it opens, so they pile up over a long visit; each phone's outbox lives only until the phone is reloaded | plan 4b, minor |
| L1 | A locked phone doesn't ring; batnet must be open (bedside mode) | browsers offline have no service worker or push ([0018](/decisions/0018-notifications-and-house-speaker.md)) |
| L3 | The simulator's WiFi model is not measured: ranges, the loss curve (2% + load²) and frame overhead are guesses | bench and field tests |
| L4 | In the simulator a WiFi channel is tracked per group; when groups split or merge the backlog resets, and loss ignores hop count | plan 2 review, minor |
| L5 | The model engine (JavaScript) shows screens the real one hasn't got yet: voice, the radio scan, a neighbour's Meshtastic app | milestone 2, plan 3b |
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
* `TestSeededDeterministic` compares vector lengths, not contents or results (plan 4b).
* The compact author form (`~/<seq>`: an account created by the writing node) is not described in the knowledge bundle yet (plan 4b).

# To measure (bench)

* Microphone and WebRTC after the certificate warning on a real iPhone and Android ([0004](/decisions/0004-self-signed-https.md)).
* USB WiFi stick current; a pocket node on a power bank ([power budget](/research/power-budget.md)).
* Moonshine speed on Pi 4, Pi 5, Zero 2 W; raw vs denoised audio ([0014](/decisions/0014-speech-pipeline.md)).
* Captive-portal pop-up on each phone; desktop notifications past the warning; Android background tab; iOS Wake Lock.
* Two Meshsticks with meshtasticd: range, airtime, priority queue.

# Resolved

| Problem | Fixed by |
|---|---|
| On a shared phone a waiting message could go out under the next person's name; one post refused at the High limit stalled the whole outbox; a gateway that heard another start a post dropped its copy for good (now [0023](/decisions/0023-hold-on-a-first-fragment.md)); picking who posts shifted every scenario's timing; message ids rode the radio | plan 4b review fixes, 2026-10-15 |
| The WebAssembly bridge stopped on a bad call from the page (B1); the app lacked an outbox, bedside mode, speaker and notification settings, German and help (L10); the real engine had no phone | plan 4b, 2026-10-15 |
| Concurrent map access could crash the server; parallel wrong PINs switched the lockout off; unlimited sign-ups; verdicts and group changes depended on arrival order; alerts forgeable by another node; live streams outlived sign-out; the app missed dropped streams and wiped typing | plan 4a review fixes, 2026-10-15 |
| Recovery stalled behind non-post entries; gateways echoed other groups' posts; public posts triggered catch-up beacons; recovered High posts dropped to 4; the simulated air merged identical frames and had instant carrier sense; power cuts didn't reboot gateways; two gateways answered every beacon together | plan 3 review fixes, 2026-10-15 (with a CSMA backoff after every frame heard) |
| Posts couldn't cross between WiFi groups | the LoRa bridge, plan 3, 2026-10-15 |
| Flooded heartbeats saturated a 40-house mesh; bang failed | [0021](/decisions/0021-one-hop-heartbeats.md), 2026-10-15 |
| Repair was free in the simulator; checks could pass falsely; bad scenarios could crash the engine | plan 2 review fixes, 2026-10-15 |
| Clock ordering after restart, author seq reuse, unbounded range requests, spoofed heartbeats | plan 1 review fixes, 2026-10-15 |
