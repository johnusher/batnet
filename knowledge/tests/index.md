# Tests

Bench and field test write-ups, one per test, named `YYYY-MM-DD-slug.md`. Use
"house A", "a neighbour", rounded distances; real places, people and device IDs
stay in `private/`.

* [2026-10-21 pi7 hub bring-up](2026-10-21-pi7-hub-bring-up.md) - first real hub - a Pi 4 with a USB Meshstick runs meshtasticd, batnetd -hub and the HDMI kiosk; the radio, the panel, mesh mode switching, the watchdogs and a cold boot were checked
* [2026-10-21 the hub on a bike ride](2026-10-21-bike-ride-log.md) - the bench hub on a USB power bank for an hour, saying hello every 5 minutes: one probable answer at 1.7 km, heard but too weak to read
* [2026-10-21 the hub's power-cut kit on pi7](2026-10-21-hub-power-cut-kit.md) - the bench hub became its own WiFi and went back to the home WiFi from the panel, with and without a password (the firewall loaded only when open); power modes, LED and button pins, the jack, the terminal and the translator worked; the kiosk with no screen had been keeping a CPU core busy
* [2026-10-21 offline translator on pi7](2026-10-21-offline-translator.md) - Argos Translate's German-English models on CTranslate2 (int8, 2 threads) translate a short message in 0.5 to 1.1 s on a Pi 4, well; 464 MB on the card, about 400 MB of memory with both directions loaded
* [2026-10-21 MediumFast against LongFast, and the antenna's place](2026-10-21-presets-and-antenna.md) - at home in Berlin MediumFast heard 26 radios to LongFast's 5; the antenna brought inside heard nothing for an hour
