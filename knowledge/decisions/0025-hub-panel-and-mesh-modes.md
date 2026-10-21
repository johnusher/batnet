---
type: Decision
title: "0025: A hub control panel, with the WiFi mesh off, mock or real"
description: batnetd -hub serves the house app and a control panel (radio, radios nearby, public channel, mesh, the Pi) on the LAN and full screen on HDMI; the mesh runs off, mock (pretend houses) or real (batman-adv), switched from the panel.
tags: [hub, panel, kiosk, batman-adv, mock, systemd, watchdog]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T14:45:00Z }
---

# Context

The first real Pi ([pi7](/hardware/bench-pi7.md)) arrived with the two
Meshsticks. The owner wanted, from the Pi's own screen and keyboard and from a
browser on the LAN: a good-looking view of the radio, a way to see who else is
out there, and the choice to run without batman, with a mock batman, or for
real. And a Pi that starts everything by itself when plugged in, with
watchdogs. Plan 5 (Pi set-up) has the kiosk in it; this decision brings part of
it forward for the bench.

# Decision

* **`batnetd -hub`** is one process: the house app on `/`, the **control
  panel** on `/hub/`, a client of meshtasticd's TCP API, and the WiFi mesh.
* **The panel** shows the radio's settings, every radio heard (a range plot by
  relay count and a table: signal, relays, last heard), the public channel with
  a send box, a "Say hello" that asks nearby radios for their names, the mesh,
  and the Pi's health and the house app's address and PINs.
* **The screen** is the panel in Chromium inside the cage kiosk compositor on
  tty1, run as a user with no shell or password (not a desktop auto-login).
  Requests from the Pi itself need no PIN; from the LAN the buttons need the
  **hub PIN**, shown only on the screen (5 wrong tries in 10 min lock the LAN out).
* **Mesh modes**, saved in `hub.json`, switched from the panel (batnetd re-runs
  itself in place):
  * *off*: the hub alone, the log on the SD card;
  * *mock*: four pretend houses in memory, each a real node with its own log on
    an in-process mesh, posting now and then; nothing is saved;
  * *real*: batman-adv on the USB WiFi stick (IBSS on channel 8, `bat0` in
    172.27.0.0/16), the log synced over UDP broadcasts (4200) and HTTP repairs
    (4201); waits for the stick if it is not plugged in.
* **Start and watchdogs**: systemd starts meshtasticd, then batnetd, then the
  screen; all restart on a crash; batnetd pets systemd's watchdog only while its
  panel answers; the hardware watchdog reboots a hung Pi.

# Consequences

* The panel is a bench and admin tool; plan 5 still owns first boot, QR join,
  the hotspot and the speaker.
* The public channel is shown raw on the panel; the app's "Neighbourhood
  (public)" group (spec §6) is plan 3b.
* Open points are in [known problems](/known-problems.md) (B34-B37).
