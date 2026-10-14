---
type: Decision
title: "0003: A house Pi with an open hotspot and a browser-only app"
description: Each house runs an open hotspot and captive portal; people use their own phones in a browser, with accounts approved by the house admin.
tags: [house, web-app, accounts, ux]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Decision

* A **house Pi** (Pi 4 or 5) runs an open hotspot with a friendly name and a
  captive portal. It needs **two WiFi radios**: one for the hotspot, one for the
  mesh (usually a USB dongle; see [WiFi radios](/hardware/wifi-radios.md)).
* The app is a **web app served by batnetd**. No install. Landing page: help,
  log in, create account. The house **admin approves** accounts.
* Inside: directory of people across houses, groups, everyone channel, text;
  voice messages (M2) and calls (M3).

# Consequences and known limits

See [browser limits offline](/research/browser-limits-offline.md):

* The microphone needs HTTPS → [0004](0004-self-signed-https.md).
* No notifications with the browser closed: a locked phone does not ring.
* The captive-portal pop-up is a stripped-down browser, so it only says
  "open http://batnet in your browser".
