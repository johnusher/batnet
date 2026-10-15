---
type: Decision
title: "0023: Hold, don't cancel, on another gateway's first fragment"
description: A gateway that hears another gateway start sending a post it holds waits for the rest; the whole post heard cancels its copy, and if the rest never comes its copy goes back in line. Refines 0022.
tags: [lora, gateway, suppression, fragments]
status: stable
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T09:30:30Z }
---

# Context

[Decision 0022](0022-gateways-suppress-duplicates.md) lets every gateway of a
WiFi group send a post after a short per-post delay, and cancels the others
when they hear it. A post longer than one LoRa frame goes as several
fragments. Cancelling only when the whole post was heard let two gateways both
send a two-fragment post. Cancelling at the first fragment (plan 4b, first
version) lost posts for good when that sender's later fragments collided or it
lost power: in the simulator, more posts went missing on the LoRa-only farms.

# Decision

* Hearing the **first fragment** of a post this gateway is about to send
  **holds** its copy back.
* Hearing the **whole post** cancels the held copy.
* If the rest isn't heard within about **6 fragment times** (each fragment's
  airtime plus the listen-before-talk wait; about 21 s per fragment on
  LongFast), the copy goes back in line with a fresh suppression delay.

# Evidence

The farms scenario over six seeds (simulator, model numbers): cancelling at the
first fragment left 8 posts missing and 59 late; holding for 1 fragment time,
13 and 44; for 3, 4 and 38; for 6, 2 and 26. Village, bang, storm and street
pass either way. Test: `TestHeldEntryResumesWhenTheOtherSenderStops`.
