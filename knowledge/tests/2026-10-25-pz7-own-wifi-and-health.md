---
type: Test Report
title: "2026-10-25: the Zero 2 W hub's own WiFi, and a health check"
description: The roof hub (a Pi Zero 2 W) became its own WiFi for 20 minutes; an iPhone joined, got an address and stayed on for 9 minutes; the hub was back on the home WiFi 3 s after being asked, with no errors of note. The health check found the system healthy, but the logs were kept only in memory (about 8 MB, 7 hours, 87% meshtasticd); they are now kept on the card, at most 64 MB.
tags: [bench, zero-2-w, wifi, access-point, logs, journald, health]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-25T09:45:00Z }
verified: { by: process:2026-10-25-pz7-own-wifi, at: 2026-10-25T07:51:00Z }
---

# Set-up

The roof hub [pz7](/hardware/bench-pz7.md): a Raspberry Pi Zero 2 W (512 MB)
with a Meshstick, in the loft, on the home WiFi only. Its built-in WiFi chip
offers AP mode, one AP alongside one client on a single channel
(`iw list`). As on the [first hub](/tests/2026-10-21-hub-power-cut-kit.md), the hub is reached only over
WiFi, so a script run by systemd on the hub did the test: it started the hub's
own WiFi through the panel's API on localhost (with a password), logged every
10 s for 20 minutes (the WiFi's state, the connected devices, the addresses
handed out, memory, load), then asked for the home WiFi back; had that
failed, it would have forced the home connection and then rebooted. The
owner joined with an iPhone in the house while it ran.

# Results

| Step | Result |
|---|---|
| Start the hub's own WiFi | AP mode on channel 6 within the request (07:30:35 to 07:30:38); NetworkManager's own DHCP and DNS (dnsmasq) up |
| An iPhone joins | at about 07:41: address 10.42.0.72, named "iPhone"; it left and came back once in its first minute, then stayed until the WiFi went off at 07:50:38 |
| Traffic | the hub sent the phone about 7.5 MB and got 1.4 MB in 9 minutes |
| Back to the home WiFi | asked at 07:50:38, connected at 07:50:41; the hub was reachable again at once |
| Memory while on its own WiFi | 214 to 224 MB used of 416, 191 to 202 MB available, swap steady at 172 MB; load under 0.35; batnetd 19 to 20 MB |
| The radio | meshtasticd ran on throughout |

The only warnings: dnsmasq found no DNS server to forward to (none is meant to
be: the hub's own WiFi has no internet), and on going back the WiFi driver
logged `brcmf_vif_set_mgmt_ie: vndr ie set error : -52` twice and
wpa_supplicant could not watch the signal (`bgscan simple`), both harmless
and known for this chip. The Zero's WiFi driver gives no signal strength for
the devices on its own WiFi, so the range was not measured.

# The health check

| What | Found |
|---|---|
| Card | 6.8 GB, 55% used, 3.0 GB free; about 0.65 GB written in 47 hours, most of it deploys and package updates; batnetd and meshtasticd each under 5 MB |
| Services | none failed, none restarted by systemd; meshtasticd last started at the USB reset of 24 October ([B74](/known-problems.md)), batnetd at the last deploy |
| Power and heat | `throttled=0x0` (no undervoltage ever), 36.5 °C |
| Memory | 416 MB, about 205 MB available; swap on zram holds 187 MB in 20 MB (compressed); nothing written to the swap file on the card |
| Clock | synchronised; the clock file saved every minute |
| Kernel | no card, file system or memory errors since boot; one orphan clean-up at the boot of 23 October after a power-off |
| The logs | in memory only (`Storage=volatile` from Raspberry Pi OS), 7.8 MB, reaching back about 7 hours; 87% from meshtasticd at `info` (in 6 hours 7,936 lines, 1,102 of them *Ignore received packet due to error*: packets too weak to read), 13% the clock saver's three lines a minute. Lost at every reboot or power cut |
| Package lists | the 32-bit (armhf) lists of the desktop image, 55 MB, with no 32-bit package installed |

# What changed

* `setup.sh` keeps the journal on the card, at most 64 MB and never the card's
  last 1 GB (`deploy/pi/systemd/journald.conf`): about a week at this rate,
  kept across reboots, and 8 MB of memory given back.
* The clock saver no longer logs (`LogLevelMax=notice`); it still saves every
  minute.
* `setup.sh` drops the armhf architecture when no 32-bit package is installed.
* meshtasticd stays at `info`: its lines are what B73 and B76 were found with.

Deployed on pz7 the same morning: 21 MB of journal on the card, no clock lines,
the radio and the home WiFi back at once.

# Still open

* How far the Zero's own WiFi reaches, and how many phones it holds
  ([to do](/todo.md)).
* The captive portal and the PIN on the open WiFi, on a Zero.
