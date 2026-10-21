---
type: Decision
title: "0026: No extra handshake to recognise batnet radios"
description: A batnet node is known by its traffic on the village's private channel (signed entries on PRIVATE_APP), not by a new handshake; on the public channel batnet sends nothing extra, and a name starting with "batnet" is only a hint.
tags: [lora, meshtastic, identity, privacy, airtime]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T14:45:00Z }
---

# Context

With a radio on the air, the owner asked whether batnet needs its own
Meshtastic handshake so that a hub can tell which radios nearby run batnet.

# Decision

* **Within a village, no handshake is needed.** Every batnet gateway already
  sends on the village's private channel, whose key comes from the village key,
  using the `PRIVATE_APP` port (spec §6): beacons every 15 minutes, directory
  cards every 30. A radio whose packets decrypt on that channel and carry
  signed batnet entries is a batnet node of this village, and the signature says
  which house. Nothing an outsider can fake.
* **On the public channel, batnet sends nothing extra.** A periodic "I run
  batnet" packet would cost shared airtime on LongFast, which Meshtastic users
  rightly guard, and would tell anyone listening that a batnet village is
  there.
* **A hint for people:** batnet nodes name themselves "batnet …" (this hub is
  "batnet hub 1"). The panel tags a radio "batnet" when it sent `PRIVATE_APP`
  traffic or has such a name; the name is never trusted for anything.
* **Who is out there:** the panel lists every radio meshtasticd has heard, and
  "Say hello" broadcasts this node's name asking for replies (Meshtastic's own
  NodeInfo exchange), at most once every 5 minutes.

# Revisit when

Two batnet villages meet and need to find each other (a shared "batnet
directory" channel would be the place, not LongFast), or when the private
channel is built in plan 3b and the tag can rest on it alone.
