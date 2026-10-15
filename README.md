# batnet

**Chat with your neighbours when the phone network and the internet are down.**
Use any phone, tablet or laptop, in its web browser. There's no app to download
and no account with any company.

### [▶ Try the app and a pretend village in your browser](https://johnusher.github.io/batnet/)

batnet turns small, cheap computers into a local chat network for a house, a
street, a village, a festival camp or a group on a hike.

> **Not ready yet.** batnet is still being designed, so there's nothing to
> install today. The link above shows how the app will look and work. To hear
> when it's ready, click **Watch** at the top of this page (you need a free
> GitHub account).

## At a glance

| | |
|---|---|
| **Range** | A few hundred metres from house to house over WiFi, with messages and calls. **1–3 km over the long-range radio**, text only. |
| **People** | About **60 phones** on a Pi 5 node, 40 on a Pi 4, 8 on a pocket Pi Zero 2 W. These are estimates, not measured yet. |
| **Cost** | About **€180–200 per house**, plus **€25** for the long-range radio. A pocket node is about **€65**. (EU prices, October 2026.) |
| **Power** | A pocket node runs about **1½ days on an ordinary 10,000 mAh power bank**. A house node uses about as much as a 4 W LED bulb left on all day. These are estimates. |
| **Setup** | Our goal is **under 15 minutes** per node, from download to first message. |

## Where it helps

* **At home, when the power or the network fails** after a storm, a flood or a
  long blackout. Each house keeps in touch with the houses around it: *who needs
  medicine, where there is water, who has a running freezer.*
* **Festivals and big events,** such as Burning Man and Glastonbury, where the
  mobile network collapses under the crowd. Each camp carries a pocket node.
  Burning Man already has a
  [citywide network of these radios](https://www.burningmesh.org/).
* **Camping, hiking and expeditions** beyond mobile coverage.
* **Volunteers and community groups** running first aid, food or shelter
  points during an emergency.

## How it works

* **Every node makes its own WiFi network.** A node is a
  [Raspberry Pi](https://www.raspberrypi.com/): a computer about the size of a
  bank card. Join the node's WiFi, called something like "batnet · Miller
  House". There's no password, and a page opens on its own, like hotel WiFi.
* **The app runs in the web browser.** Any phone, tablet or laptop works,
  whether it's an iPhone, an Android or something else. Nothing to install.
* **It feels like WhatsApp.** There's a list of people in your house and in
  other houses, groups such as *Medical* and *Food*, and a channel for
  everyone. Houses within WiFi reach of each other can also make calls.
* **Nodes link up on their own.** Close ones (within a few hundred metres) link
  over WiFi. Far ones (1–3 km) link over a small long-range radio, called
  LoRa, added to the Pi. That radio carries text only. A spoken message is
  turned into text, sent, and read aloud at the other end.
* **Someone in each house approves new accounts,** so strangers can't join just
  by walking past.
* **It works with [Meshtastic](https://meshtastic.org/),** the popular
  long-range radio messenger. People with a Meshtastic radio see batnet's
  public posts and can reply, and their radios help carry batnet's messages
  further, without being able to read the private ones.

## Ways to take part

| You have | What you can do |
|---|---|
| **Just a phone** | Join the nearest batnet WiFi and use everything: messages, groups, calls. |
| **A pocket node** (Pi Zero 2 W + radio + power bank) | Bring batnet to a camp, a hike or a small household. It serves about 8 phones and links to other nodes by radio, as text. |
| **A house node** (Pi 4 or 5) | Serve a whole household, link to neighbouring houses over WiFi, and add the radio to reach houses 1–3 km away. |
| **A Meshtastic radio** paired with your phone | Read and post on the public channel from anywhere in radio range, even with no node nearby. |

## What to buy (Europe)

To find each part, type the **article number** into the shop's search box.
Prices are from October 2026, include VAT and leave out shipping. Shops sell
out, so check before you order.

**A house node, about €180–200.** Use either a Pi 5 or a Pi 4, with the matching
power supply and case.

| Part | Shop | Article number | About |
|---|---|---|---|
| Raspberry Pi 5, 4 GB | [Berrybase](https://www.berrybase.de/raspberry-pi-5-4gb-ram) | `RPI5-4GB` | €119 |
| *or* Raspberry Pi 4 Model B, 4 GB | [Berrybase](https://www.berrybase.de/raspberry-pi-4-computer-modell-b-4gb-ram) | `RPI4B-4GB` | €106 |
| Power supply: 27 W for a Pi 5 | [Berrybase](https://www.berrybase.de/raspberry-pi-27w-usb-c-power-supply-netzteil-weiss) | `RPI5NT5AW` | €12 |
| *or* 15 W for a Pi 4 | [Berrybase](https://www.berrybase.de/offizielles-raspberry-pi-usb-c-netzteil-5-1v-3-0a-eu-weiss) | `RPI4NT3AW` | €8 |
| Memory card, SanDisk Ultra 32 GB | [Berrybase](https://www.berrybase.de/sandisk-ultra-microsdhc-a1-120mb-s-class-10-speicherkarte-adapter-32gb) | `619659184155` | €18 |
| Case: official Pi 5 case (has a fan) | [Berrybase](https://www.berrybase.de/offizielles-gehaeuse-fuer-raspberry-pi-5-rot-weiss) | `RPI5-CASE-RE` | €10 |
| *or* official Pi 4 case | [Berrybase](https://www.berrybase.de/offizielles-gehaeuse-fuer-raspberry-pi-4-rot-weiss) | `RPI4-CASE-RE` | €5 |
| WiFi stick ALFA AWUS036ACHM, to link to other houses | [wlan-shop24](https://www.wlan-shop24.de/alfa-network-awus036achm-wlan-usb-adapter-802-11ac-dual-band) | `ART7230` | €43 |

**Add the long-range radio, about €25.** Pick one:

| Part | Shop | Article number | About |
|---|---|---|---|
| RAK WisMesh Pi HAT (RAK6421) | [Hexaspot](https://hexaspot.com/products/wismesh-pi-hat-rak6421) | `RAK-610012` | €14 |
| + its radio module RAK13300, EU868, antenna included | [Hexaspot](https://hexaspot.com/products/module-for-lorawan-with-semtech-lora-chip-sx1262-rak13300) | `RAK-105006` | €10 |
| *or* Elecrow Meshstick (plugs into USB, antenna included) | [Berrybase](https://www.berrybase.de/elecrow-meshstick-usb-to-spi-sx1262-tcxo-lora-usb-stick-meshtastic-eu868-sma-22-dbm-5-v) | `EL-LMM15202D` | €25 |

**A pocket node, about €65,** plus a USB power bank you may already have:

| Part | Shop | Article number | About |
|---|---|---|---|
| Raspberry Pi Zero 2 W | [Berrybase](https://www.berrybase.de/raspberry-pi-zero-2-w) | `RPIZ-2W` | €19 |
| Memory card, SanDisk Ultra 32 GB | [Berrybase](https://www.berrybase.de/sandisk-ultra-microsdhc-a1-120mb-s-class-10-speicherkarte-adapter-32gb) | `619659184155` | €18 |
| Long-range radio (RAK6421 + RAK13300, as above) | Hexaspot | `RAK-610012` + `RAK-105006` | €25 |
| A [3D-printed case](#3d-printable-cases) | print your own, or ask a local makerspace | | €2 of plastic |

**Optional extras, about €20–50:**

| Part | Shop | Article number | About | Why |
|---|---|---|---|---|
| USB sound card + 8 Ω speaker | [Berrybase](https://www.berrybase.de/waveshare-usb-sound-card-stereo-codec-mikrofon-lautsprecheranschluss-abtastraten-8-48khz) | `WS-18833` + `125171` | €15 | The house speaker that announces messages |
| Lit arcade button, 30 mm | [Berrybase](https://www.berrybase.de/detail/019234a4ff0e726490baff8fbad83dda) | `AB30L5-G` | €2 | Blinks for unread messages; press to hear them |
| USB GPS receiver | [Berrybase](https://www.berrybase.de/adafruit-ultimate-gps-mit-usb-66-channel-w-10-hz-updates) | `ADA4279` | €30 | Gives the whole village the right time, and the house's position |

**Don't buy these, they won't work:**
* **WiFi sticks with an MT7921 chip,** such as the ALFA AWUS036AXM and AXML.
  They can't link houses together.
* **Most cheap WiFi sticks.** These use Realtek chips, which need extra drivers.
* **The Waveshare "SX1262 LoRa HAT"** (Berrybase `RPI-868LORA`). It looks right,
  but it's the wrong type of board for this radio software.

Want a much longer WiFi link, 1–3 km between two houses? A pair of TP-Link
CPE210 outdoor units does it, about €42 each at
[reichelt](https://www.reichelt.de/de/de/shop/produkt/wlan_access_point_2_4_ghz_300_mbit_s_outdoor-150844).

## Installing

These steps are the plan for the first release and don't work yet. Choose one
way:

### Way 1: download and install (no GitHub account needed)

1. **Download one file:** *batnet.zip*. (The download link will appear here
   with the first release.)
2. **Unzip it** and double-click the installer for your computer:
   *Install on Windows* or *Install on Mac*. It opens the free
   [Raspberry Pi Imager](https://www.raspberrypi.com/software/) with batnet
   already chosen, and offers to download Imager first if you don't have it.
3. **Put the memory card in your computer,** pick it in Imager, and click
   **Write**. This takes about 5 minutes.
4. **Put the card in the Pi,** plug in the WiFi stick and the radio, and switch
   it on.
5. **On your phone, join the WiFi "batnet · setup".** A set-up page opens.
   Name your house (for example "Miller House") and create the first account,
   which approves everyone else.
6. **At the next house, do the same.** Nodes in range find each other by
   themselves.

### Way 2: with GitHub, or on a Pi you already use

If your Pi already runs Raspberry Pi OS, one command will install batnet on it.
It will be published with the first release. Developers can clone this
repository instead: see [For developers](#for-developers).

### Then, for everyone in the house

1. Join the WiFi **batnet · *your house name*** (no password).
2. A page pops up. Follow it to open your normal web browser.
3. **The browser shows a "not private" warning, once.** This is expected:
   without the internet, the node can't get the usual security certificate.
   The page shows you how to continue, and the connection is still encrypted.
4. Create an account and wait for your house's approver to let you in.

### How you find out about new messages

Without the internet, a web page can't make a locked phone ring. So batnet
tells you in other ways:

* **Keep batnet open.** Your phone plays a sound for each new message, and
  vibrates on Android. Urgent messages are louder, and an alert from your
  house's approver sounds a siren.
* **Bedside mode.** At night, put the phone on charge and tap *Bedside mode* in
  Settings. The screen stays on, dimmed, so batnet can ring.
* **The house speaker.** Plug a small speaker into the house's Pi. It says who a
  message is for and who sent it ("Message for Ben, from Anna at Miller House"),
  never what it says, and only for people who switch this on. It stays quiet at
  night, except for urgent messages and alerts.
* **A button and a light on the Pi.** The light blinks while someone in the
  house has unread messages. Press the button to hear the announcements again.
* **Android phones, optional:** install the free
  [ntfy](https://ntfy.sh/) app *before* you need it. It can show real
  notifications from the house's Pi with no internet. iPhones can't do this.

## 3D-printable cases

Free designs that people have shared. Most are for non-commercial use: print
them for yourself, don't sell them. Pick one made for your radio board, because
boards that look alike differ in size.

* **Pocket node:**
  * [Pi Zero 2 W node with GPS](https://www.printables.com/model/909529-meshtastic-raspberry-pi-zero-2-w-based-node-with-g) (Waveshare radio board)
  * [base plate for a Pi Zero 2 W + RAK6421](https://www.printables.com/model/1750503-pi-zero-2w-rak6421-hat)
* **Pi plus a power bank:**
  [Rugged Case](https://www.printables.com/model/771172-rugged-case-for-raspberry-pi),
  which fits a Pi 4, Pi 3 or Zero 2 W with a 10,400 mAh battery. Drill a hole
  for the antenna.
* **House or field node:**
  * [Haven](https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle) (Pi 4 with an antenna on top)
  * [Haven Node for Pi 5](https://www.printables.com/model/1578286-haven-node-raspberry-pi-5-double-antenna)
  * [openMANET Radio Case](https://makerworld.com/en/models/2128181-openmanet-radio-case), which has its own batteries
* **On a pole or a tent:**
  * [sealed mast-mount box for a Pi Zero 2 W](https://www.printables.com/model/1722952-meshtastic-enclosure-for-meshadv-pi-hat-on-raspber)
  * [radome on a painter's-pole thread](https://www.printables.com/model/301926-100mm-radome-for-meshtastictbeamrak-wisbocksdrrasp)

## Power use

Estimates from published measurements, not yet tested on our own kit. Details
are in [the power budget](knowledge/research/power-budget.md).

| Use case | Node | Energy a day | Runs on a 10,000 mAh power bank |
|---|---|---|---|
| Festival camp, 300 messages a day, all by radio | Pocket node (Pi Zero 2 W) | 18 Wh | about 1½ days |
| Relay on a pole, WiFi only | Pi Zero 2 W | 16 Wh | about 2 days |
| 5 houses on WiFi, 1,000 messages a day between them | House node (Pi 4), each | 84 Wh | about 8 hours |
| The same with 10,000 messages a day | House node (Pi 4), each | 84 Wh | about 8 hours |
| House with radio: 1,000 messages and 100 voice messages a day | House node (Pi 5) | 92 Wh | about 8 hours |

**The number of messages hardly matters.** A text over WiFi costs about a
thousandth of a joule. The computer's own idle draw sets the total, so a Pi
Zero 2 W uses about a fifth of what a Pi 4 or 5 does. Turning speech into text
is the only per-message cost worth counting. A house node wants mains power, a
car or leisure battery (about 300 Wh runs it 3–4 days), or a solar panel.
Phones run on their own batteries.

---

## Try the simulator

[![The batnet village simulator: a map of houses linked by WiFi and radio, beside a phone showing the batnet welcome page](docs/img/village-sim-light.png#gh-light-mode-only)](https://johnusher.github.io/batnet/)
[![The batnet village simulator: a map of houses linked by WiFi and radio, beside a phone showing the batnet welcome page](docs/img/village-sim-dark.png#gh-dark-mode-only)](https://johnusher.github.io/batnet/)

**[Open the simulator](https://johnusher.github.io/batnet/)**: a pretend village
that runs entirely in your web browser.

* **See the app through anyone's phone:** joining, signing up, chatting, the
  list of people in reach, or a neighbour's Meshtastic app.
* **Watch messages travel** between houses over WiFi and over the long-range
  radio.
* **Change the village:** more houses, bigger households, fewer radios. You can
  also cut a house's power and watch what still gets through.

**Two engines.** *Model* is a quick JavaScript model of the whole system.
*Real batnet code* runs batnet's actual Go code for every house, compiled for
the browser, through prepared scenarios such as *bang* (everyone hears a bang
and posts at once) and *storm* (houses lose power and catch up). Today it covers
the WiFi mesh; the radio, accounts and the app join as they are built.

To run it offline:
1. Click the green **Code** button at the top of this page, then
   **Download ZIP**.
2. Unzip it, open the folder `prototypes`, then `village-sim`.
3. Double-click `index.html`.

The numbers in the simulator come from a model, not from real radios. What
we've learned from it is in
[the simulator findings](knowledge/research/village-sim-findings.md).

---

## For developers

The project's research, decisions and hardware notes are in
[`knowledge/`](knowledge/). Start with the [vision](knowledge/vision.md), the
[decisions](knowledge/decisions/) and the
[EU shopping list](knowledge/hardware/shopping-list-eu.md). The knowledge is
stored as an
[Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
bundle, which both people and coding agents can read. Instructions for agents
are in [AGENTS.md](AGENTS.md).

**Set up a clone.** You need `git`, [Go](https://go.dev/dl/) 1.26 or newer, and
[gitleaks](https://github.com/gitleaks/gitleaks#installing):

```sh
git clone https://github.com/johnusher/batnet.git
cd batnet
git config core.hooksPath .githooks          # secret scan + knowledge check on every commit
go test ./...                                 # run the tests
go run ./cmd/batnetd -mock 5 -loss 0.3        # a pretend village of 5 houses on this computer
go run ./cmd/batnetd -sim scenarios/*.json    # village scenarios with the real code, judged pass/fail
(cd tools/okflint && go run . ../../knowledge) # check the knowledge bundle by hand
```

This repository is public, so never commit keys, passwords, real names or
precise locations. See [secret hygiene](knowledge/security/secret-hygiene.md).

**Licence:** [MIT](LICENSE). You're free to use, copy, change and share batnet.
