---
type: Decision
title: "0001: Two network layers - batman-adv WiFi mesh and Meshtastic LoRa"
description: Nearby houses link over a batman-adv WiFi mesh carrying full IP; distant ones over Meshtastic LoRa carrying text only.
tags: [network, batman-adv, meshtastic, lora]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Context

batman-adv routes Ethernet frames, so it needs a link that behaves like Ethernet
(WiFi, Ethernet, a point-to-point WiFi bridge). LoRa frames are at most ~255 bytes
at 0.3–5.5 kbit/s with a 10% EU airtime limit, so batman-adv cannot run over it.
See [radio links compared](/research/radio-links-compared.md).

# Decision

* **Local layer:** batman-adv over WiFi (IBSS or 802.11s, see
  [WiFi mesh modes](/research/wifi-mesh-modes.md)) or over Ethernet links such as
  outdoor point-to-point units. Full IP: calls, voice, video, text.
* **Long-range layer:** Meshtastic on LoRa, text and compact data only.
  Speech crosses it as text (speech-to-text at the sender, text-to-speech at the far end).
* Gateways (nodes with both) bridge the two through batnetd's message log.

# Consequences

* Calls work only between houses joined by the WiFi mesh; the app shows which ones.
* Any neighbour with a Meshtastic radio can join the long-range layer.
* LoRa capacity is the scarce resource: see [0008](0008-longfast-single-radio.md).
