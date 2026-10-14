---
type: Decision
title: "0004: Self-signed HTTPS with one browser warning"
description: Each house Pi makes its own certificate; the help screen walks people through the one-time browser warning so the microphone and calls work.
tags: [https, web-app, microphone, webrtc]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
verified: { by: human:johnusher, at: 2026-10-14T18:10:00Z }
---

# Context

Browsers allow `getUserMedia` (microphone, camera) only in a secure context.
Offline there is no publicly trusted certificate. Options weighed: self-signed,
a real certificate obtained while online (expires ~90 days into an outage),
installing a batnet CA profile per phone, plain HTTP (no microphone).

# Decision

Each house Pi generates a self-signed certificate. The help screen explains the
"not private" warning (Advanced → Proceed). A real certificate may be added later
as an option for houses that have one.

# To verify first

Before building on it: confirm on a current iPhone (Safari) and Android (Chrome)
that the microphone and WebRTC work after accepting the warning. Record the
result as a `verified` entry here. If it fails, revisit this decision.
