---
type: Decision
title: "0028: The hub's own WiFi in a power cut, with neighbours let in only by the admin"
description: The hub's built-in WiFi joins the home WiFi on every boot; from its screen, a button or by itself when home WiFi is missing for a few minutes, it becomes its own password-protected WiFi; "Let neighbours in" (off by default) opens it to everyone, behind a firewall that leaves only the app.
tags: [hub, wifi, hotspot, access-point, power-cut, networkmanager, security]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T23:30:00Z }
---

# Context

In a power cut the home router is off, and the bench hub's built-in WiFi only
joins the home WiFi ([B41](/known-problems.md)): phones and laptops lose the
panel. The owner wanted, on 2026-10-21: to make the hub an access point from its
own screen and keyboard, so a laptop or phone can reach the page; neighbours let
in only by an admin switch, off by default; a normal reboot back on the home
WiFi; an automatic fallback too; and always a way back to the home WiFi. Also:
join other WiFi networks from the panel.

The milestone 1 spec has an always-open hotspot on the built-in radio of every
house Pi ([spec §8, §12](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md)).
A Pi 4 has one built-in radio, and its driver allows a home WiFi connection and
an access point only on one channel together; that combination is
known to be fragile, so the hub does one or the other.

# Decision

* **NetworkManager does it.** It already runs the home WiFi on the hub; its
  `shared` mode gives an access point with DHCP and DNS (its own dnsmasq), no new
  packages. The hub's WiFi is a saved connection `batnet-own-wifi`, never
  connected automatically, so **every boot starts on the home WiFi**.
* **Three ways in, one way back:**
  * a button, *Start the hub's own WiFi* (System view; the hub's screen needs no
    PIN);
  * **automatic** (on by default): no home WiFi for 3 minutes (after boot, or
    after it was lost) starts the hub's own WiFi. Started this way, the hub tries
    the home WiFi again every 10 minutes while nobody is connected to it, and goes
    back when it is there;
  * a long press (3 s) on the hub's button, for a hub with no screen.
  * **Back to home WiFi** is always offered while the hub's WiFi is on, and always
    works. After it, the automatic fallback waits until the home WiFi has been
    seen again: the owner's choice wins.
* **Password by default.** The hub's WiFi is WPA2 with a password made on first
  start (`wifi-password`, shown with a QR code on the hub's screen only, like the
  PINs). The name is `batnet-` and four letters of the node ID, editable.
  Clients cannot see each other (AP isolation).
* **"Let neighbours in" (admin, off by default)** removes the password. Then a
  firewall on the WiFi lets in only DHCP, DNS, mDNS and port 80 (the house app;
  every new account still needs an admin's approval), and drops SSH,
  meshtasticd's API (4403) and its web client (9443), which would let anyone
  change the radio. On the open WiFi the panel, reads included, needs the hub
  PIN. DNS answers every name with the hub, so phones show the app as a captive
  portal.
* **Joining other WiFi:** the System view lists the networks in range (when the
  hub is not running its own), joins one with its password, joins a hidden one,
  forgets one, and makes one the default (the highest autoconnect priority).
* batnetd runs as its own user; NetworkManager lets it do this through a polkit
  rule ([0031](/decisions/0031-root-actions-through-polkit.md)).

# Consequences

* Resolves [B41](/known-problems.md) on the bench hub, once tested in a real cut.
* The hub cannot be on the home WiFi and its own at once: while its own runs, it
  has no internet (no map tiles, no clock from the internet).
* A USB WiFi stick is batman-adv's ([0025](/decisions/0025-hub-panel-and-mesh-modes.md)); a
  second stick could carry the hotspot permanently, as the spec's house Pi does.
* Supersedes nothing yet; plan 5's open hotspot will build on this connection.
