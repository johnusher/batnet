---
type: Decision
title: "0018: Notifications: in the app, bedside mode, and a speaker, button and LED on the house Pi"
description: Browsers can't ring a locked phone offline, so batnet rings inside the open app, keeps the screen on in bedside mode, and announces messages on a speaker on the house Pi; ntfy is an optional Android helper.
tags: [notifications, speaker, gpio, accessibility, no-phone]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T06:58:54Z }
verified: { by: human:johnusher, at: 2026-10-15T06:58:54Z }
---

# Context

Offline and behind a self-signed certificate, a browser has no service worker
(Android needs one for system notifications) and no push (iPhones only show web
notifications through Apple's push servers). So a phone rings only while batnet
is open. See [browser limits](/research/browser-limits-offline.md). Houses without
phones need another way too ([0016](0016-no-phone-mode.md)).

# Decision

1. **In the app:** a sound and vibration (Android) per message, louder for High,
   a siren for Alert; a banner; the unread count in the title; mute per chat.
2. **Bedside mode:** keeps the screen on and dimmed (Screen Wake Lock) while
   charging, so the phone rings overnight.
3. **House speaker (M1):** a USB speaker (any Pi) or the Pi 4's audio jack.
   * Chime plus spoken "Message for Ben, from Anna at Miller House": who for and
     who from, never what it says. High: "Urgent message for Ben". Alert: siren,
     then the full text.
   * Announces direct and group messages for people who opt in (Settings);
     everyone-channel posts only when High; Alerts always. Quiet hours,
     default 22:00-07:00, except High and Alert.
   * Speech by espeak-ng in M1 (a separate process, GPL), Piper in M2.
4. **Button and LED on the house Pi (M1):** the LED blinks while someone in the
   house has unread messages; pressing the button reads waiting announcements
   aloud again (and, in no-phone mode, wakes the screen).
5. **ntfy (optional, Android):** the README recommends installing the free ntfy
   app *before* an outage; batnetd offers a per-person ntfy topic over the local
   WiFi, carrying only "New message from Anna" because the hotspot is unencrypted.

# To test

Desktop browsers' pop-up notifications on a page past the certificate warning;
how long Android keeps the connection alive in a background tab; Wake Lock on
iOS Safari.
