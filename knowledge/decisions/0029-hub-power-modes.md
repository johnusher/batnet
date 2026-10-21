---
type: Decision
title: "0029: Three power modes for the hub: high, medium, low"
description: One setting trades speed and screen time for battery life - the CPU's top speed, WiFi power saving, the screen sleeping after a few idle minutes (waking on a key or a message), the Pi's own lights - and the radio always keeps listening.
tags: [hub, power, battery, screen, cpu, power-cut]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
---

# Context

The hub runs on a power bank or a battery in a power cut. It needs about 3 to 4 W
([power budget](/research/power-budget.md)); a Pi 4 idles at about 2.7 W and a
screen adds 1 to 3 W (not measured). The owner asked for a low power mode,
"perhaps configurable, or at least high, mid, low". Most of a Pi 4's idle power
does not depend on its settings; what can be cut is the screen, the browser
drawing it, the CPU's peaks, the WiFi's listening and a few LEDs.

# Decision

| | High (default) | Medium | Low |
|---|---|---|---|
| CPU top speed | 1.8 GHz | 1.0 GHz | 600 MHz (powersave) |
| Screen | always on | sleeps after 10 min idle | sleeps after 2 min idle |
| Screen wakes on | | a key, the mouse, a message for the hub | a key or the mouse (a message: sound and LED only) |
| Reduced screen (the lowest resolution and refresh rate the screen offers, no animations) | off | off | on |
| The panel's refresh | 2 s | 2 s, 30 s while the screen sleeps | 5 s, 30 s while the screen sleeps |
| WiFi power saving | off | on | on |
| The Pi's red and green lights | on | on | off |
| Translator kept loaded | 10 min | 10 min | 1 min |

* **Reduced screen** can also be switched on by itself in any mode (asked for on
  2026-10-21: "a much lower res screen, or at least an option for that on low
  power mode, lower refresh rate"). The kiosk picks, with `wlr-randr`, the
  screen's mode with the fewest pixels at or above 640 × 480 and, among those,
  the lowest refresh rate.
* **The screen runs only while one is plugged in.** Found the same night: with
  no screen, the kiosk compositor and Chromium kept drawing to nothing, about one
  CPU core between them (49 CPU-minutes in 47 minutes) and a warm Pi. batnetd
  starts the kiosk when a screen is connected and stops it when it is
  unplugged; at boot it starts only if one is there.
* **The radio, the log, the house app, sound and the LED are never cut**: they
  are what the hub is for in a cut.
* The WiFi mesh (the USB stick, about 1 W) stays as set; the System view says
  what it costs in low power.
* A sleeping screen is switched off (`wlr-randr` inside the kiosk, which turns
  the HDMI signal off, so most screens go to standby) and, as a fallback, the
  panel goes black.
* The root part (CPU, WiFi power saving, the Pi's lights) is a small script run
  as `batnet-power@<mode>.service`, which batnetd may start through polkit
  ([0031](/decisions/0031-root-actions-through-polkit.md)); the mode is saved in
  `hub.json` and applied at every start.
* **The power supply is watched**: the Pi's under-voltage alarm shows on Home
  ("power too weak") as it happens.

# Consequences

* The savings are estimates until measured with a USB power meter
  ([known problems](/known-problems.md)): probably 0.3 to 1 W from the screen's
  sleep (plus the screen's own power), little from the CPU at idle.
* Later: a battery HAT that reports its charge could switch to low power by
  itself below a threshold.
