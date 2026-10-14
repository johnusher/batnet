---
type: Research Note
title: WiFi mesh modes on Raspberry Pi (IBSS, 802.11s, hotspot alongside)
description: brcmfmac support for ad-hoc and 802.11s, running a hotspot at the same time, and which USB dongles to use.
tags: [wifi, batman-adv, ibss, 80211s, brcmfmac]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-14T18:10:00Z }
stale_after: 2027-04-14T00:00:00Z
sources:
  - id: batman-ml
    resource: https://lists.open-mesh.org/mailman3/hyperkitty/list/b.a.t.m.a.n@lists.open-mesh.org/thread/2SWXZUTA4SSDPAKVZ34FPYZINK33KX7S
    title: batman-adv mailing list - brcmfmac and mesh modes
  - id: batman-zero
    resource: https://lists.open-mesh.org/mailman3/hyperkitty/list/b.a.t.m.a.n@lists.open-mesh.org/thread/CIZRYSGMU2LLM67QO3EOL52PZK2SDNVG
    title: batman-adv mailing list - Pi Zero W ad-hoc reports
---

# Known

* IBSS (ad-hoc) + batman-adv works on Pi Zero W and Pi 3 in user reports, with MTU warnings.[^batman-zero]
* Do not bridge the wireless interface; run batman-adv on top of the IBSS or 802.11s interface.[^batman-ml]
* A 2017 maintainer opinion: fullmac chips like brcmfmac are more likely to work with 802.11s than with IBSS.[^batman-ml]
* The author's earlier project ran IBSS on channel 8 (2447 MHz), `bat0` in 172.27.0.0/16, UDP broadcast on port 4200.

# Unknown - test on the bench

* IBSS vs 802.11s on Pi 4 / Pi 5 with current kernels.
* Can the built-in radio run the mesh and a hotspot at once? Assumed no → USB dongle.
* Which USB dongles do IBSS/802.11s/AP reliably (ath9k_htc and mt76 families are the usual candidates).
