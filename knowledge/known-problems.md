---
type: Issue List
title: Known problems
description: Open bugs, known limits, deferred review findings and things still to measure, each with where it was found and its status.
tags: [bugs, known-problems, review, backlog]
status: stable
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T13:45:00Z }
---

Every bug found, every known limit and every review finding not fixed at once
goes here, with where it came from. Move a line to **Resolved** (with the
commit or decision) when it is fixed; delete resolved lines after a month.
This page is the list.

# Open bugs

| # | Problem | Found | Area |
|---|---|---|---|
| B5 | Seqs at or above 2^63 are stored as negative numbers in SQLite; a range up to MaxUint64 returns nothing | plan 1 review | `ledger/sqlstore` |
| B6 | `ParseVector` reserves memory for the claimed count before checking the bytes are there | plan 1 review | `ledger/vector.go` |
| B7 | Repair goroutines aren't waited for when `Run` returns; closing the store can race in-flight writes (real-time path only) | plan 1 review | `meshsync` |

| B9 | A gateway's LoRa duty meter is kept across a restart in the simulator; batnetd must store it, or a reboot could exceed the 10% limit | plan 3 | `internal/lora` |

| B11 | Another village on the same channel key: its beacons are answered and its signed entries injected; one short beacon draws up to 8 entries back (a cheap way to drain a gateway's airtime). Relies on Meshtastic's channel key; rate-limit answers per requester | plan 3 review m2 | `internal/lora` |
| B15 | The simulator's connectivity counts LoRa paths of any length: a house four radio hops away counts as connected, though only beacon recovery (gateway to gateway) reaches it; the flooding itself now stops after 3 hops | plan 3 review m6; flooding fixed 2026-10-15 | `internal/sim` |
| B16 | `sameWiFiGroup` rebuilds WiFi groups on every frame; `snapGroups` is O(radios²) (the `heard` and `seen` tables are bounded now) | plan 3 review m7, m8 | `internal/sim`, `internal/lora` |
| B18 | Short IDs (4-byte prefixes) can be ground to collide; a tie makes compact vectors order by map and runs stop repeating | plan 3 review m9 | `internal/lora/frame.go` |

| B19 | Sessions and PIN lockouts live in memory: a reboot signs everyone out, and a guesser can move to another house, or take another address on the hotspot, to reset the lockout (it is per phone address and name since B22) | plan 4a | `internal/auth`, `internal/web` |
| B20 | Two people with the same name and the same PIN at one house can't be told apart at sign-in | plan 4a | `internal/web/api.go` |
| B32 | Village over other seeds (simulator): seed 2 has 6 posts late against 15 min after the 10 min power cut of a hamlet's only gateway; the backlog, its catch-up beacons every 2 min and the others' recoveries share one busy channel. Farms seed 2: 1 late | review 2026-10-15 | `internal/lora` |
| B34 | Real mode: heartbeats on bat0 are flooded to the whole mesh (`udplink.BroadcastLocal` is a plain broadcast), not sent one hop as [0021](/decisions/0021-one-hop-heartbeats.md) asks; fine for a few Pis, not for 40 houses. Real mode itself is untested until a USB WiFi stick and a second Pi are on the bench | pi7 bring-up 2026-10-21 | `internal/udplink` |
| B35 | meshtasticd listens on the whole LAN: its TCP API (4403) and web client (9443) let anyone on the network change the radio, with no password. Bench only; a house node should firewall both to localhost. On the hub's own WiFi open to neighbours both are blocked ([0028](/decisions/0028-hub-own-wifi.md)); on the home WiFi and the closed own WiFi they are not | pi7 bring-up 2026-10-21 | `deploy/pi` |
| B36 | Two batnet nodes could pick the same mesh address (172.27.a.b from the first two bytes of the node key: a 1 in 65 000 chance per pair) | pi7 bring-up 2026-10-21 | `cmd/batnetd/hub.go` |
| B37 | A second hub joins the village only by copying `village.key` by hand (until the QR join of plan 5) | pi7 bring-up 2026-10-21 | `cmd/batnetd/hub.go` |
| B38 | meshtasticd serves one TCP API client at a time: a second client (the Meshtastic command-line tool, a phone app over WiFi, another batnetd) pushes the hub off the radio; the hub reconnects within 5 s and pushes it off in turn. Use the command-line tool briefly, or stop the hub first | pi7 screen 2026-10-21 | `internal/meshtastic` |
| B39 | With no internet the Pi's clock is wrong after a power cut: it restarts from the time saved before (`batnet-clock.timer`, every minute), behind by the time without power. A hub needs a time source of its own: GPS (spec: GPS time), a phone's time, or a clock HAT | bike ride 2026-10-21 | `deploy/pi`, plan 5 |
| B41 | In a power cut the home router is off. The hub's own WiFi ([0028](/decisions/0028-hub-own-wifi.md)) now starts from the panel, the button or by itself, and was tested on the bench without a phone: a phone or laptop joining it (DHCP, the QR code, the captive portal) and the fallback with the router really off are still to try | help pages 2026-10-21; built 2026-10-21 | `internal/hub/wifi.go`, `deploy/pi` |
| B42 | The screen's sleep and reduced mode (`wlr-randr` inside cage) are untested with a screen: if cage quits when its only output is switched off, the screen would restart instead of sleeping (the panel goes black anyway, as a fallback) | plan 5a 2026-10-21 | `deploy/pi/batnet-kiosk` |
| B43 | The hub's button is polled every 30 ms (a wake-up 33 times a second); the GPIO uAPI's edge events would let it sleep | plan 5a 2026-10-21 | `internal/hub/light.go` |
| B33 | A beacon nudge (decision 0024) needs another gateway's beacon or a partial message: a post whose every frame was lost at a receiver, with no beacon heard since, still waits for the receiver's own quarter-hour beacon | review 2026-10-15 | `internal/lora/bridge.go` |


# Known limits (by design, for now)

| # | Limit | Why / when it changes |
|---|---|---|
| L10 | The app has no voice messages, photos or location sharing on a map yet | milestone 2 ([0019](/decisions/0019-location.md), [0020](/decisions/0020-photos.md)) |
| L12 | A house remembers the outbox's message ids (clientIds) for a day, in memory: a retry after the house Pi restarts, or at another house, can post twice | plan 4b review (ids kept off the radio) |
| L11 | The simulator's phone: its bedside clock shows the real time, not the village's; the page never closes the sessions and live streams it opens, so they pile up over a long visit (at most 8 per person now, B26); each phone's outbox lives only until the phone is reloaded | plan 4b, minor |
| L1 | A locked phone doesn't ring; batnet must be open (bedside mode) | browsers offline have no service worker or push ([0018](/decisions/0018-notifications-and-house-speaker.md)) |
| L3 | The simulator's WiFi model is not measured: ranges, the loss curve (2% + load²) and frame overhead are guesses | bench and field tests |
| L13 | The simulator's LoRa rebroadcast delay follows Meshtastic's contention window in shape (a random number of ~70 ms slots, 4 for a weak signal to 128 for a strong one, with distance standing in for SNR), not in measured numbers; its 50 ms carrier sense and 3 km LongFast reach are guesses too | bench: two Meshsticks |
| L4 | In the simulator a WiFi channel is tracked per group; when groups split or merge the backlog resets, and loss ignores hop count | plan 2 review, minor |
| L5 | The model engine (JavaScript) shows screens the real one hasn't got yet: voice, the radio scan, a neighbour's Meshtastic app | milestone 2, plan 3b |
| L8 | Everyone-channel posts don't yet go out as plain Meshtastic text for users holding the key (done-when check 3); public-channel text interop isn't built | plan 3b (meshtasticd client) |
| L9 | A signed entry costs about 100 bytes before its text on LoRa (origin key and signature): two packets for a 100-character post | a compact LoRa form, later |
| L6 | An author signing two different entries with the same seq would not be detected (vectors hold seqs, not content) | plan 1 review; needs a protocol decision |
| L14 | The hub's screen shows the hub PIN and the house admin PIN to anyone standing at it: physical access is trusted on the bench | [0025](/decisions/0025-hub-panel-and-mesh-modes.md); plan 5 decides for real houses |
| L15 | The hub panel shows the public channel raw; the app's "Neighbourhood (public)" group and plain-text posts on the private channel come with the LoRa bridge in plan 3b (L8) | plan 3b |
| L16 | The hub's messages and its traffic view live in memory: the last 200 messages and the last hour of traffic, gone when batnetd restarts (the log keeps them on disk while logging is on); what was read is kept per browser | hub panel 2026-10-21; changes when the hub's messages move into the house log (plan 3b) |
| L17 | The panel's map uses OpenStreetMap's tiles: they need the internet, and OSM allows only light use. Grid-down, the map shows the radios on a plain grid without streets | hub panel 2026-10-21; changes with offline tiles for the village's area on the hub |
| L18 | The panel names only six common boards (T-Beam, T-Echo, RAK 4631, Heltec V3, T1000-E, meshtasticd); others show as a model number | hub panel 2026-10-21 |
| L19 | A Pi 4 has one built-in WiFi radio: the hub is on the home WiFi or runs its own, never both; while it runs its own there is no internet | [0028](/decisions/0028-hub-own-wifi.md); a second USB stick that does AP mode would lift it |
| L20 | The Pi 4's audio jack has no plug detection: the hub can't tell whether a speaker is plugged in (the panel says so and offers a test sound) | plan 5a 2026-10-21 |
| L21 | The translator needs about 400 MB of memory while loaded and 464 MB on the card; German and English only by default; machine translation errs on slang and abbreviations | [translator test](/tests/2026-10-21-offline-translator.md) |
| L22 | WiFi passwords given to Join pass to `nmcli` on its command line, visible to other users of the Pi for a moment | plan 5a 2026-10-21; the Pi has no other users |
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
* Two Meshsticks with meshtasticd: range, airtime, priority queue (the first hub runs: [bring-up](/tests/2026-10-21-pi7-hub-bring-up.md)).
* The keyboard on the hub's screen (the 800 × 480 display works).
* Real mesh mode with a USB WiFi stick and a second Pi (IBSS, batman-adv, sync over bat0).
* The hub's power: each power mode, the screen awake, asleep and unplugged, with a USB power meter ([0029](/decisions/0029-hub-power-modes.md)).
* The power-cut kit with hands at the hub: a phone on the hub's own WiFi, the fallback with the router off, the screen's sleep, a screen and keyboard plugged in later, the LED, the button, a speaker ([test](/tests/2026-10-21-hub-power-cut-kit.md)).

# Resolved

| Problem | Fixed by |
|---|---|
| The kiosk ran with no screen plugged in: cage and Chromium drew to nothing and kept about a CPU core busy, and the Pi warm (58 °C) | the screen runs only while one is plugged in: `ExecCondition` at boot, batnetd starts and stops it as screens come and go ([0029](/decisions/0029-hub-power-modes.md)); 42 °C idle after; reported by the owner, 2026-10-21 |
| B40 The hub's log missed packets that fail their checksum (meshtasticd drops them before its API) | batnetd follows meshtasticd's journal (`journalctl -u meshtasticd -f`, the hub in the systemd-journal group) and logs each as an `unreadable` line with its signal and garbled header; the panel counts them; bike ride follow-up, 2026-10-21 |
| Sign-in hardening B21-B25 (exact argon2 parameters, lockout per phone address and name with a bounded table, a dummy PIN check when no name matches, JSON-only sign-in, sign-up and sign-out, a stricter CSP); B26 at most 8 live streams per account; B13 High capped at 8; B10 the LoRa gate opens only on a verified entry or well-formed beacon; B12 the oldest partial message is evicted; B16 (part) bounded `heard` and `seen` tables; B14 the duty check reads the air's own tally; B15 (part) a 3-hop limit on rebroadcasts; B17 Meshtastic users validated; B30 phones at a house without power get 503; B31 a house with no one else posts as its admin; B3 scripted posts that can't be made are reported; B4 one bad scenario file no longer stops the rest; B2 a failed restart in the real engine is shown; B27 the outbox goes round again for a message written mid-send; B28 bedside mode shows an alert's text; B29 failed buttons flash their error; alerts come back after a reload and can be dismissed | review 2026-10-15 |
| B8 LoRa-only farms: 0 missing and 0 late on the CI seed, 0 missing and 1 late over six other seeds, with decision [0024](/decisions/0024-ask-soon-for-what-was-lost.md) (a gateway asks within 2 min for what it saw it was missing), the 3-hop limit and Meshtastic's contention window in the model, and a delivered check that counts only time connected to a holder; the new festival scenario (6 pocket nodes, LoRa only) passes on seven seeds | 2026-10-15 |
| On a shared phone a waiting message could go out under the next person's name; one post refused at the High limit stalled the whole outbox; a gateway that heard another start a post dropped its copy for good (now [0023](/decisions/0023-hold-on-a-first-fragment.md)); picking who posts shifted every scenario's timing; message ids rode the radio | plan 4b review fixes, 2026-10-15 |
| The WebAssembly bridge stopped on a bad call from the page (B1); the app lacked an outbox, bedside mode, speaker and notification settings, German and help (L10); the real engine had no phone | plan 4b, 2026-10-15 |
| Concurrent map access could crash the server; parallel wrong PINs switched the lockout off; unlimited sign-ups; verdicts and group changes depended on arrival order; alerts forgeable by another node; live streams outlived sign-out; the app missed dropped streams and wiped typing | plan 4a review fixes, 2026-10-15 |
| Recovery stalled behind non-post entries; gateways echoed other groups' posts; public posts triggered catch-up beacons; recovered High posts dropped to 4; the simulated air merged identical frames and had instant carrier sense; power cuts didn't reboot gateways; two gateways answered every beacon together | plan 3 review fixes, 2026-10-15 (with a CSMA backoff after every frame heard) |
| Posts couldn't cross between WiFi groups | the LoRa bridge, plan 3, 2026-10-15 |
| Flooded heartbeats saturated a 40-house mesh; bang failed | [0021](/decisions/0021-one-hop-heartbeats.md), 2026-10-15 |
| Repair was free in the simulator; checks could pass falsely; bad scenarios could crash the engine | plan 2 review fixes, 2026-10-15 |
| Clock ordering after restart, author seq reuse, unbounded range requests, spoofed heartbeats | plan 1 review fixes, 2026-10-15 |
