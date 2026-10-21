---
type: Test Report
title: "2026-10-21: the hub's power-cut kit on pi7"
description: The bench hub became its own WiFi and went back to the home WiFi from the panel, with and without a password (the firewall loaded only when open); power modes, LED and button pins, the jack, the terminal and the translator worked; the kiosk with no screen had been keeping a CPU core busy.
tags: [bench, hub, wifi, access-point, power, gpio, sound, terminal, kiosk, cpu]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
---

# Setup

The [bench hub](/hardware/bench-pi7.md) (Pi 4, 2 GB, Raspberry Pi OS trixie,
NetworkManager 1.52, cage 0.3.1, Chromium 155) on the home WiFi, no screen,
keyboard, LED, button or speaker plugged in. batnetd with plan 5a
([decisions 0028 to 0031](/decisions/0028-hub-own-wifi.md)) deployed with
`deploy/pi/deploy.sh`. The hub is reached only over WiFi, so the WiFi test ran
as a timed script on the hub itself, with a safety timer that would have put
it back on the home WiFi after 6 minutes.

# Results

**The kiosk with no screen.** Before the kit, with no HDMI screen plugged in,
`cage` and Chromium used about 45 % of a core each (49 CPU-minutes in 47
minutes); the owner noticed the Pi was hot. Stopping the screen took the
temperature from 58 °C to 53 °C in 20 s, and to 42 °C a few minutes later
with a load average of 0.03. The screen now runs only while one is plugged in
([0029](/decisions/0029-hub-power-modes.md)).

**The hub's own WiFi, through the panel's API** (each step one POST from the hub):

| Step | Result |
|---|---|
| Start the hub's own WiFi (password) | in AP mode on channel 6 within the request; NetworkManager `connected` on `batnet-own-wifi`; no firewall table |
| Back to home WiFi | on the home WiFi again within about 20 s |
| Let neighbours in, then start it | open (no password); the firewall table `inet batnet_own` loaded by the dispatcher (only DHCP, DNS, mDNS, port 80; no forwarding) |
| Back to home WiFi, neighbours off | home WiFi again; the table removed |

The connection is saved with autoconnect off, AP isolation on and IPv4
`shared` (NetworkManager's own DHCP and DNS). polkit let batnetd's user do all
of this.

**Other parts:**

* GPIO: line 17 an output and line 27 an input with pull-up, both held by
  `batnet` (`gpioinfo`); the uAPI struct sizes match the kernel's.
* Sound: the jack found (`bcm2835 Headphones`); the test chime and espeak-ng's
  sentence played with no error (nobody listened: no speaker).
* Power modes: Low set the CPU to `powersave` at 600 MHz, WiFi power saving
  on and the Pi's lights off; High put back `ondemand` at 1.8 GHz, power
  saving off and the green light on `mmc0`.
* Terminal: the button switched the console to tty2 with a login prompt
  (`getty@tty2`); `chvt 1` came back.
* Translator: de→en 1.4 s and en→de 1.0 s per message through the panel's API,
  the first including loading the model ([translator test](/tests/2026-10-21-offline-translator.md)).
* batnetd itself: about 1 % of a core (0.6 CPU-seconds a minute) with the kit
  running, after looking at the WiFi every 30 s on the home WiFi instead of 10 s.

# What it settles

* NetworkManager's `shared` mode is enough for the hub's own WiFi on a Pi 4: no
  hostapd or dnsmasq of our own.
* The kiosk must not run without a screen.

# Not tested (needs hands at the hub)

* A phone or laptop joining the hub's own WiFi: DHCP, the QR code, the captive
  portal (every name answers with the hub), the PIN on the open WiFi.
* The automatic fallback with the router really off, and its return.
* The screen: sleep and wake (`wlr-randr` inside cage), reduced mode, a screen
  plugged in later, keyboard and mouse plugged in later.
* The LED and the button with real parts; the sound with a speaker.
* Power: how much each mode and the screen's sleep save (a USB power meter).
