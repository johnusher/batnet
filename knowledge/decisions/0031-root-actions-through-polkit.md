---
type: Decision
title: "0031: The hub's root actions go through polkit; the terminal is the Pi's own console"
description: batnetd stays an unprivileged user; a polkit rule lets it control NetworkManager, start a few fixed batnet units (power modes, the console, the screen), reboot, switch off and set the clock; the panel's Terminal button switches the hub's screen to a normal Linux login, never a shell in the browser.
tags: [hub, security, polkit, terminal, systemd]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
---

# Context

The power-cut features ([0028](/decisions/0028-hub-own-wifi.md),
[0029](/decisions/0029-hub-power-modes.md)) need root: WiFi settings, CPU speed,
the Pi's lights, the clock, reboot. batnetd runs as the `batnet` user with only
the network capabilities it needs ([0025](/decisions/0025-hub-panel-and-mesh-modes.md)).
The owner also asked for a button that opens a terminal on the Linux system,
used "generally at the hub's own keyboard".

# Decision

* **polkit, not sudo.** `/etc/polkit-1/rules.d/50-batnet.rules` lets the user
  `batnet` do exactly: switch, scan, save and share WiFi through NetworkManager
  (`network-control`, `settings.modify.system`, `wifi.scan`, `wifi.share.*`);
  start, restart or stop `batnet-power@{high,medium,low}`, `batnet-console`,
  `batnet-screen` and `batnet-clock`; reboot and power off; set the time.
  Nothing else. batnetd calls `nmcli`, `systemctl`
  and `timedatectl`, which ask over D-Bus; the hub's unit keeps its restrictions.
* **The terminal is the Pi's console.** *Open a terminal* (shown and allowed only
  on the hub's own screen) starts `batnet-console.service`, which switches the
  screen to tty2: a normal login prompt that needs the Pi's user and password;
  Ctrl+Alt+F1 comes back. A shell inside the web page would be one PIN away from
  anyone on the WiFi, and would die with batnetd, which is when a terminal is
  most needed.
* From another device, the PIN-protected buttons may reboot, switch off and set
  the clock (from the device's own clock, after a power cut without internet),
  never open a shell.

# Consequences

* A bug in batnetd can, at worst, change WiFi, reboot, switch off or set the
  clock: no files, no shell. NetworkManager's permissions are coarse, though:
  `settings.modify.system` and `network-control` also let batnetd read every
  saved WiFi password, add or change any connection (wired, DNS) and take the
  wired port down (review, 2026-10-22). A root helper with a fixed list of
  verbs, like `batnet-power`, would narrow it when it matters.
* SSH stays the way in from a laptop on the home WiFi or the hub's own WiFi with
  its password; it is blocked when neighbours are let in.
