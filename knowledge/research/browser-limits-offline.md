---
type: Research Note
title: What a browser-only app can and cannot do offline
description: Secure context for the microphone, no notifications without a service worker, and the limits of captive-portal pop-ups.
tags: [web-app, https, webrtc, captive-portal, notifications]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
---

# Claims (from general knowledge - each needs a test on real phones)

* `getUserMedia` (microphone, camera) and WebRTC need a secure context (HTTPS or localhost).
* After accepting a self-signed certificate warning, Chrome, Firefox and Safari still treat the page as a secure context, so the microphone works. **Test first** ([decision 0004](/decisions/0004-self-signed-https.md)).
* Service workers do not register on a page with a certificate error, so no web push, no background ringing, no installable PWA.
* iOS's captive-portal assistant is a restricted browser (no microphone; cookies not kept), so the portal page must send people to the real browser.
* A phone on WiFi without internet may warn or prefer mobile data; in a grid-down outage mobile data is usually gone too.

# Test plan

One iPhone (current iOS) and one Android (current Chrome): join an open hotspot,
captive-portal pop-up, open `https://batnet` past the warning, record audio, make
a WebRTC call to a laptop. Record pass/fail per claim here.
