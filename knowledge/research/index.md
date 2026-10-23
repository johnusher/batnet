# Research

What is out there and what the numbers say. Most notes are `draft`: gathered from
sources, not yet confirmed on our hardware.

* [Radio links compared](radio-links-compared.md) - range, usable rate and what each link can carry - Pi WiFi with batman-adv, LoRa, WiFi HaLow, point-to-point WiFi
* [LoRa airtime in EU868](lora-airtime-eu868.md) - how many batnet messages per hour a village can push over Meshtastic LongFast, and why text fits but voice does not
* [Voice bitrates](voice-bitrates.md) - why speech-to-text-to-speech is the way to carry speech over LoRa, and why WiFi keeps real audio
* [On-device speech](on-device-speech.md) - which Pi can run which speech models, from Moonshine and Whisper on the Pi 5 down to nothing on the Zero W
* [Speech pipeline](speech-pipeline.md) - which voice-activity detector, denoiser, speech-to-text and text-to-speech model to run on a Pi 5, Pi 4, Zero 2 W and in the phone browser, with the measured numbers, who measured them, and licences
* [Existing projects](existing-projects.md) - Meshtastic, MeshCore, Reticulum and Pi walkie-talkie projects - what each does and what batnet takes from them
* [Browser limits offline](browser-limits-offline.md) - secure context for the microphone, no notifications without a service worker, and the limits of captive-portal pop-ups
* [SDR scanning](sdr-scanning.md) - what a cheap RTL-SDR on a house Pi can detect and the legal line between activity and content
* [WiFi mesh modes](wifi-mesh-modes.md) - brcmfmac support for ad-hoc and 802.11s, running a hotspot at the same time, and which USB dongles to use
* [OKF and agent knowledge](okf-and-agent-knowledge.md) - why this bundle follows Google's OKF v0.2, what the spec requires, and the practices taken from it
* [Village simulator findings](village-sim-findings.md) - findings from the in-browser village model - LoRa duty limits at busy gateways, WiFi groups with no LoRa radio, and recovery traffic
* [EU868 radio basics](eu868-radio-basics.md) - the facts behind the README's radio section - the 863-870 MHz slices and limits, Meshtastic's LongFast parameters, ERP and antennas, Fresnel zone and horizon, range records, and the history of the bands and LoRa
* [The Berlin blackout of September 2025](berlin-blackout-2025.md) - the public record behind the film's opening (two 110 kV pylons in Johannisthal set on fire the night of 8/9 September 2025, about 50,000 customers, about 60 hours, six localities), and where the scene's map and outlines come from
* [Etiquette on Berlin's community mesh](mesh-etiquette.md) - what Freifunk, the Pico Peering Agreement, the CCC's hacker ethics, Berlin Chaos Mesh and Meshtastic say about sharing the mesh, and how batnet behaves there
* [Meshtastic presets in Berlin](meshtastic-presets-berlin.md) - the city is split between LongFast and MediumFast, which share 869.525 MHz in the EU but cannot hear each other; what it means for a hub
* [Power budget](power-budget.md) - daily energy for pocket, house and relay nodes; the Pi's idle draw dominates and messages cost almost nothing
