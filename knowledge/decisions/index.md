# Decisions

Architecture decision records. `draft` = proposed, not yet agreed.

* [0001 Two network layers](0001-two-layer-network.md) - batman-adv WiFi mesh with full IP nearby, Meshtastic LoRa with text only further away
* [0002 SX1262 SPI HAT](0002-sx1262-spi-hat.md) - each gateway Pi carries an SPI SX1262 HAT running meshtasticd; USB boards supported
* [0003 House Pi, browser-only app](0003-house-node-browser-app.md) - open hotspot and captive portal per house, phones in a browser, admin-approved accounts
* [0004 Self-signed HTTPS](0004-self-signed-https.md) - one browser warning so the microphone and calls work; verify on real phones first
* [0005 Pure-Go core](0005-pure-go-core.md) - one CGo-free batnetd binary on every node, roles from config, helpers for heavy work
* [0006 Private + public channels](0006-private-and-public-channels.md) - private batnet Meshtastic channel plus public LongFast as a Neighbourhood group
* [0007 Replicated log (draft)](0007-replicated-log.md) - immutable entries broadcast live and synced with version vectors; LoRa gets a compact subset
* [0008 LongFast, one radio (draft)](0008-longfast-single-radio.md) - both channels on one HAT at LongFast; a second faster radio as an upgrade
* [0009 Scanner reports activity only](0009-scanner-activity-only.md) - frequency, strength, time and label, never content
* [0010 Milestone order](0010-milestone-order.md) - text first, then voice messages, calls, scanner
* [0011 MIT licence](0011-mit-licence.md) - the whole repository is MIT-licensed
* [0012 Pocket node (draft)](0012-pocket-node.md) - Pi Zero 2 W + LoRa HAT + power bank for festivals, camps and hikes
* [0013 One download to install (draft)](0013-one-download-installer.md) - one zip from a fixed link with Imager-based install scripts; one command for existing Pis
* [0014 Speech pipeline (draft)](0014-speech-pipeline.md) - Moonshine on the raw audio, denoise only what people hear, the phone's own voice reads text aloud
