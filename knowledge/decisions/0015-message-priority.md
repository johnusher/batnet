---
type: Decision
title: "0015: Message priority - High, Normal, Low on a 0-9 scale (draft)"
description: People pick High, Normal or Low (admins also Alert); the network turns that into 0-9 for the radio queue, notifications and speech work. High is rate-limited.
tags: [priority, lora, queue, alerts, ui]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:12:37Z }
sources:
  - id: mesh-proto
    resource: https://github.com/meshtastic/protobufs/blob/master/meshtastic/mesh.proto
    title: Meshtastic MeshPacket.Priority
---

# Context

The radio is the bottleneck: one busy gateway hits the 10% airtime limit and
messages queue for hours ([simulator findings](/research/village-sim-findings.md)).
Over WiFi everything arrives in well under a second, so priority matters only
where there is a queue: the radio, the speech helper, and people's attention.
Meshtastic has a packet priority (BACKGROUND 10, DEFAULT 64, HIGH 100, ALERT 110,
ACK 120) but it orders one radio's own queue and is **never sent over the
air**.[^mesh-proto] batnet must carry priority in its own envelope so every
batnet gateway on the way applies it.

# Decision (proposed)

**What people see:** a ❗ button beside the message box. Normal by default;
**High** ("goes first over the radio and alerts people"), **Low** ("can wait,
sent when the radio is quiet"). House admins also get **Alert everyone**, for
emergencies: it shows at the top of every phone. The choice resets to Normal
after each message.

**Inside, 0-9:**

| Level | What |
|---|---|
| 9 | Alert (admins only) |
| 8 | High, any kind of message (never below 8) |
| 6 | Sync beacons (they keep recovery working) |
| 5–6 | Normal: 5, direct messages +1 |
| 4 | Normal public posts (−1), resends (−1) |
| 2–3 | Low |
| 1 | Directory cards, housekeeping |

* Waiting raises a level by one step per 20 minutes, up to 7: Low cannot
  starve, Normal never overtakes High.
* Each radio sends its highest level first, oldest first within a level.
* **High is limited to 3 per person per hour** (the admin can change it), so
  it keeps its meaning. Alerts show the admin's name.
* 4 bits in the batnet envelope carry the level over LoRa; on the local radio
  it maps to Meshtastic's queue: Alert → ALERT, High → HIGH, Normal → DEFAULT,
  Low and background → BACKGROUND.
* The same order applies to the speech helper's transcription queue and to
  catch-up after an outage (newest High first).
* Phones: High plays a sound while the app is open; Low never notifies.
* Public Meshtastic posts carry no priority (plain text for other apps).

# Evidence

Village simulator, 40-house stress scenario, 6 h, 5% of messages High and
12.5% Low: slowest 5% of radio deliveries **21 min for High against 131 min for
Normal**. In quiet villages all levels arrive in about 2 s: priority only
matters when the radio is congested. It cannot create airtime: a radio at its
duty limit still holds High messages until the limit allows.

# Open

Whether people understand three levels without help; whether the High limit
is right; a sound for High that works on iPhones without a tap.
