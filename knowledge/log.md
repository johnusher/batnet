# Bundle update log

## 2026-10-15
* **Creation**: [To do](/todo.md) list; plan 3 review fixes and nine deferred findings in [known problems](/known-problems.md).
* **Update**: Plan 3 built: the LoRa bridge and priority run in the simulator ([findings](/research/village-sim-findings.md)); decision [0022](/decisions/0022-gateways-suppress-duplicates.md) supersedes 0017's assignment rule.
* **Creation**: Research note [EU868 radio basics](/research/eu868-radio-basics.md), behind the README's new radio section (band, rules, antennas, range, history, with diagrams); LongFast airtime for 150 bytes corrected to ~1.4 s in [LoRa airtime](/research/lora-airtime-eu868.md); glossary gained ERP, dBi, chirp, Fresnel zone and waterfall.
* **Creation**: [Known problems](/known-problems.md) list (new type `Issue List`); decision [0021 one-hop heartbeats](/decisions/0021-one-hop-heartbeats.md), which makes bang pass.
* **Update**: Plan 2 built: the real code runs in virtual time (`batnetd -sim`, CI, the simulator's real engine). [Findings](/research/village-sim-findings.md): flooded heartbeats saturate an 80-house mesh with no messages.
* **Creation**: Decisions [0018 notifications and house speaker](/decisions/0018-notifications-and-house-speaker.md), [0019 location](/decisions/0019-location.md), [0020 photos](/decisions/0020-photos.md); milestones updated.
* **Update**: Plan 1 of milestone 1 built: signed log, SQLite store, sealing, mesh sync with repair, and a mock village (`go run ./cmd/batnetd -mock 5`).
* **Update**: The [milestone 1 spec](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md) was approved; decisions 0007, 0008, 0012, 0013, 0015 and 0016 are now stable, and [0017](/decisions/0017-gateways-share-the-radio.md) supersedes 0007's single LoRa sender. Simulator findings gained priority and Meshtastic results. The [skill catalog](/skills/catalog.md) gained a tool that replaced a skill (`shoot.sh`) and three planned skills.
* **Creation**: Draft decision [0016 No-phone mode](/decisions/0016-no-phone-mode.md) and a no-phone section in the [vision](/vision.md); design sections 3 (web app) and 4 (set-up, keys, QR join) approved.
* **Creation**: Draft decision [0015 Message priority](/decisions/0015-message-priority.md), modelled in the village simulator.
* **Creation**: Draft decision [0014 Speech pipeline](/decisions/0014-speech-pipeline.md); 0012 updated on speech for the Zero 2 W.
* **Creation**: Research note [Speech pipeline](/research/speech-pipeline.md): VAD, denoiser, speech-to-text and text-to-speech per board, with sources; finds that denoising before speech-to-text hurts accuracy and that Moonshine Tiny may fit a Zero 2 W.
* **Creation**: [Power budget](/research/power-budget.md), [EU shopping list](/hardware/shopping-list-eu.md), [printable cases and festivals](/hardware/portable-cases.md), draft decision [0013 One download to install](/decisions/0013-one-download-installer.md).
* **Update**: WiFi stick and LoRa HAT advice corrected: MT7921AU has no mesh mode; meshtasticd marks RAK6421 official and the Waveshare SPI HAT deprecated.
* **Creation**: Draft decision [0012 Pocket node](/decisions/0012-pocket-node.md) and a portable-use section in the [vision](/vision.md).
* **Creation**: Decision [0011 MIT licence](/decisions/0011-mit-licence.md), as the repository went public.

## 2026-10-14
* **Creation**: Built the village simulator prototype and recorded its [findings](/research/village-sim-findings.md).
* **Update**: Baseline-tested a `capturing-knowledge` skill and found it unnecessary (see the [skill catalog](/skills/catalog.md)); added the `Test Report` type and the [tests](/tests/) section; marked planned skills as not yet existing.
* **Initialization**: Created the bundle from the first design session: [vision](/vision.md), decisions 0001–0010, research notes on radios, LoRa airtime, voice bitrates, on-device speech, existing projects, browser HTTPS limits, SDR scanning and WiFi mesh modes, the hardware catalogue, and the [secret hygiene policy](/security/secret-hygiene.md).
