---
type: Decision
title: "0016: No-phone mode on the Pi's own screen"
description: With a screen and keyboard plugged in, a Pi 4/5 shows the same web app full-screen as a shared terminal or a notice board; a Pi Zero or 3 gets a text-only terminal app later.
tags: [kiosk, hdmi, accessibility, shared-device, notice-board]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:18:29Z }
verified: { by: human:johnusher, at: 2026-10-15T06:27:44Z }
---

# Context

Not everyone has a phone, and some places want one shared screen: a household
without smartphones, an older neighbour, a village hall, a first-aid point. The
user asked for a mode that works with an HDMI screen and a keyboard on the Pi.

It also covers a Pi whose WiFi can't serve phones: a Pi with one WiFi radio
spent on the batman-adv mesh (no dongle), or a weak built-in radio. Such a Pi
is still a full member of the village through its own screen (user, 2026-10-15).

# Decision

* **Pi 4/5 with a screen at boot:** start a full-screen browser (Chromium under
  the cage kiosk compositor, Wayland, no desktop) on the **same web app**, served
  locally. One app to build and test, all features (voice with a USB headset or
  webcam microphone, calls in M3). The kiosk trusts the house certificate, so no
  warning. The official touchscreen works the same way.
* **Two ways to use it**, chosen on the admin screen:
  * **Shared terminal:** people sign in with their name and PIN; idle sign-out
    after 2 minutes; a big "Sign out" button. Several people per screen.
  * **Notice board:** no sign-in, read-only, large type: alerts, the everyone
    channel and the "Neighbourhood (public)" group, rotating. For a hall or a
    window.
* **Pi Zero 2 W / Pi 3** (too little memory for a browser): a text terminal app
  in Go on the HDMI console (tty1), text messages only. Later than M1.
* **One-radio Pis:** when set-up finds a single WiFi radio, it offers "mesh
  plus screen" (radio on the mesh, people use the screen) as well as "hotspot
  only". This makes the small-Pi terminal app more useful, since Zero and Pi 3
  boards usually have one radio.
* **Wired laptop:** a laptop plugged into a Pi 4/5's Ethernet port gets an
  address and the same app, with no WiFi involved.
* Nothing starts when no screen is attached; the kiosk costs no power then.

# Consequences

* The image grows by roughly 300 MB (cage, Chromium, fonts) on arm64.
* The web app needs keyboard-only use and large-type layouts anyway; the kiosk
  is a test of that.
* Milestone: kiosk + shared terminal + notice board in **M1** (same app);
  terminal app for small Pis later.

# Open

Chromium memory on a 2 GB Pi 4 alongside the speech helper; screen blanking
and power on battery; on-screen keyboard for touchscreen-only setups.
