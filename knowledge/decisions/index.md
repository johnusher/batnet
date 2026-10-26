# Decisions

Architecture decision records. `draft` = proposed, not yet agreed. The approved [milestone 1 spec](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md) builds on them.

* [0001 Two network layers](0001-two-layer-network.md) - batman-adv WiFi mesh with full IP nearby, Meshtastic LoRa with text only further away
* [0002 SX1262 SPI HAT](0002-sx1262-spi-hat.md) - each gateway Pi carries an SPI SX1262 HAT running meshtasticd; USB boards supported
* [0003 House Pi, browser-only app](0003-house-node-browser-app.md) - open hotspot and captive portal per house, phones in a browser, admin-approved accounts
* [0004 Self-signed HTTPS](0004-self-signed-https.md) - one browser warning so the microphone and calls work; verify on real phones first
* [0005 Pure-Go core](0005-pure-go-core.md) - one CGo-free batnetd binary on every node, roles from config, helpers for heavy work
* [0006 Private + public channels](0006-private-and-public-channels.md) - private batnet Meshtastic channel plus public LongFast as a Neighbourhood group
* [0007 Replicated log](0007-replicated-log.md) - immutable entries broadcast live and synced with version vectors; LoRa gets a compact subset
* [0008 LongFast, one radio](0008-longfast-single-radio.md) - both channels on one HAT at LongFast; a second faster radio as an upgrade
* [0009 Scanner reports activity only](0009-scanner-activity-only.md) - frequency, strength, time and label, never content
* [0010 Milestone order](0010-milestone-order.md) - text first, then voice messages, calls, scanner
* [0011 MIT licence](0011-mit-licence.md) - the whole repository is MIT-licensed
* [0012 Pocket node](0012-pocket-node.md) - Pi Zero 2 W + LoRa HAT + power bank for festivals, camps and hikes
* [0013 One download to install](0013-one-download-installer.md) - one zip from a fixed link with Imager-based install scripts; one command for existing Pis
* [0014 Speech pipeline (draft)](0014-speech-pipeline.md) - Moonshine on the raw audio, denoise only what people hear, the phone's own voice reads text aloud
* [0015 Message priority](0015-message-priority.md) - High, Normal, Low (and admin Alert) mapped to 0-9 for the radio queue; High rate-limited
* [0016 No-phone mode](0016-no-phone-mode.md) - screen and keyboard on a Pi 4/5: the same app full-screen as a shared terminal or notice board
* [0017 Gateways share the radio](0017-gateways-share-the-radio.md) - entries spread over a WiFi group's gateways by key; supersedes 0007's single sender
* [0018 Notifications and house speaker](0018-notifications-and-house-speaker.md) - ring in the open app, bedside mode, a speaker, button and LED on the house Pi; ntfy optional on Android
* [0019 Location](0019-location.md) - off by default, per message, from the phone or the node's GPS; tracking and zones later
* [0020 Photos](0020-photos.md) - metadata stripped, resampled above 500 KB, full over WiFi, blurred preview over LoRa (M2)
* [0021 One-hop heartbeats](0021-one-hop-heartbeats.md) - heartbeats reach direct neighbours only; flooded ones saturated a 40-house mesh
* [0022 Gateways suppress duplicates](0022-gateways-suppress-duplicates.md) - any gateway may send after a per-post delay; hearing it sent cancels the rest; supersedes 0017's hash assignment
* [0023 Hold on a first fragment](0023-hold-on-a-first-fragment.md) - hearing another gateway start a post holds ours; the whole post cancels it; if the rest never comes, ours goes; refines 0022
* [0024 Ask soon for what was lost](0024-ask-soon-for-what-was-lost.md) - a gateway holding part of a message, or hearing a beacon ahead of its own log, beacons within two minutes; farms and festival pass; refines 0022
* [0025 Hub panel and mesh modes](0025-hub-panel-and-mesh-modes.md) - batnetd -hub serves the app and a control panel on the LAN and on HDMI; the WiFi mesh runs off, mock or real
* [0026 Recognising batnet radios](0026-recognising-batnet-radios.md) - no extra handshake: the private channel and signed entries mark batnet nodes; nothing extra on the public channel
* [0027 The preset follows the local mesh](0027-preset-follows-the-local-mesh.md) - the Meshtastic preset is a village setting, measured first; a second radio listens to the other preset (draft)
* [0028 The hub's own WiFi](0028-hub-own-wifi.md) - every boot joins the home WiFi; a button, the hub's button or 3 minutes without home WiFi start its own (password, QR); "Let neighbours in" opens it behind a firewall; back home always works (draft)
* [0029 Power modes](0029-hub-power-modes.md) - high, medium, low: CPU speed, WiFi power saving, the screen's sleep and a reduced screen, the Pi's lights; the screen runs only while one is plugged in (draft)
* [0030 Offline translator](0030-offline-translator.md) - Argos models on CTranslate2 in a helper process: German and English on the hub, no internet (draft)
* [0031 Root actions through polkit](0031-root-actions-through-polkit.md) - batnetd stays unprivileged; polkit allows NetworkManager, a few batnet units, reboot, power off, the clock; the terminal is the Pi's own console (draft)
* [0032 Offline maps](0032-offline-maps.md) - Protomaps extracts of OpenStreetMap, fetched by each hub while it has internet (round the hub by itself, cities at a button), drawn by protomaps-leaflet; no map data in the repository (draft)
* [0033 The away note](0033-away-note.md) - a private message unanswered after 15 minutes gets one short note in simple German that says it is automatic; once per radio a week, never on a channel, never to an "ok" or a machine; off unless switched on (draft)
* [0034 Hop limit 5](0034-hop-limit-five.md) - the roof hub's own packets may be passed on 5 times, not 3: in Berlin, where most send with 7, 3 reached about 60 % of the radios it hears and 5 about 88 %; 7 is left to the city; elsewhere 3 unless the neighbours use more (stable)
