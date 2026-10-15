---
type: Vision
title: batnet vision
description: Grid-down community comms - a Raspberry Pi per house, an open hotspot, a WhatsApp-like web app, linked by WiFi mesh and LoRa.
tags: [vision, scope, milestones]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Purpose

Communications that keep working when the power grid and the internet are down,
for a household, a street or a village. Built from Raspberry Pis people already
own (Zero, Zero 2, 3, 4, 5), and simple enough for others to set up.

# The house model

* Each **house** has a Pi running an **open WiFi hotspot** with a friendly name
  (e.g. "batnet · Miller House") and no password.
* Joining it shows a landing page: **help**, **log in**, **create account**.
  The house **admin approves** new accounts.
* Inside, it works **like WhatsApp in the browser**: a directory of people in this
  house and other houses, **groups** (e.g. medical, food), a channel for
  **everyone**, text, voice messages and **live calls**.
* **No app download**, any phone or laptop with a browser, simple to use.
* A **radio scan mode** shows who else is on the air: Meshtastic users, WiFi
  meshes, walkie-talkie and ham bands, any 868 MHz LoRa, anything that looks
  like human comms (activity only, never content).

# No-phone mode

A node must also work with no phone at all (user, 2026-10-15): plug a screen
and keyboard (or the official touchscreen) into the Pi and use batnet on it
directly, like a shared household computer or a notice board in a village
hall. See [decision 0016](/decisions/0016-no-phone-mode.md).

# Portable use

The same kit travels (user, 2026-10-15): festivals such as Burning Man and
Glastonbury where the mobile network collapses under the crowd, camping and
hiking groups, community events. A camp or group carries a small node (a Pi Zero
2 W, a LoRa HAT and a power bank in a 3D-printed case) instead of a house Pi.
See [decision 0012](/decisions/0012-pocket-node.md).

# How houses connect

See [decision 0001](/decisions/0001-two-layer-network.md). Houses near each other
link over a batman-adv WiFi mesh (full IP: calls, voice, text). Houses further
away link over Meshtastic LoRa (text only; speech crosses as text). The first
deployment is a village, houses 150 m to 1 km apart.

# Milestones

| | Scope |
|---|---|
| **M1** | House Pi, captive portal, accounts + admin approval, directory, groups + everyone channel, text chat, log sync over batman-adv, text over LoRa via meshtasticd, installer, "radios nearby" page (Meshtastic node list + WiFi scan); house speaker, button and LED for notifications; location per message (phone or node GPS) |
| **M2** | Voice messages: speech-to-text on the house Pi, sent as text over LoRa, spoken by the phone; photos; location tracking |
| **M3** | Live audio calls between WiFi-linked houses (WebRTC + Pion) |
| **M4** | Radio scanner: SDR sweep, human-comms detection, band-plan labels, village "on air" view, Meshtastic preset sweep |
| Later | Video calls, sensor and camera nodes, solar/battery monitoring, location zones, offline street maps |

Milestone 1 is specified in the [milestone 1 spec](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md) (approved 2026-10-15).

# Milestone 1 is done when

1. Phone A on house 1 sends text; phone B at a LoRa-only house receives it.
2. Phone C at a WiFi-linked house receives it too.
3. A plain Meshtastic app user on the private channel sees everyone-channel posts and can reply.
4. A node that was off catches up on what it missed when it comes back.
5. A new node is one script plus one config file.

# Not in milestone 1

Live calls, voice messages, video, sensors, the SDR scanner, solar/battery
management, native phone apps.
