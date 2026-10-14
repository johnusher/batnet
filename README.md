# batnet

*Working name.* Off-grid community communications from Raspberry Pis you already own.

When the grid and the internet are down, each house runs a Pi with an open WiFi
hotspot. Anyone with a phone or laptop joins it, opens the browser and gets a
WhatsApp-like app: a directory of people across houses, groups (medical, food),
a channel for everyone, text, voice messages and calls. No app to install.

Houses link to each other over a **batman-adv WiFi mesh** (full IP, calls work)
and, further away, over **Meshtastic LoRa** (text; speech travels as text and is
read aloud at the far end). A scan mode shows who else is on the air.

**Status:** design phase. Read the [vision](knowledge/vision.md), the
[decisions](knowledge/decisions/) and the [research](knowledge/research/).

## For contributors

* Enable the hooks once: `git config core.hooksPath .githooks` (needs `gitleaks` and Go).
* Knowledge lives in [`knowledge/`](knowledge/) as an [Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md) bundle.
* Agents: see [AGENTS.md](AGENTS.md).

Licence: to be decided before the first public release.
