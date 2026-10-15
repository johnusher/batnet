# batnet

**Keep your street talking when the phones and the internet go down.**

batnet turns small, cheap computers into a local chat network for your house,
your street or your village. People join from the phone or laptop they already
have. It works with no internet and no mobile signal.

> **Not ready yet.** batnet is still being designed, so there is nothing to
> install today. You can [try the simulator](#try-the-simulator) at the bottom
> of this page to see how it will work. To hear when it's ready, click
> **Watch** at the top of this page (you need a free GitHub account).

## What it does

* **One small computer per house.** Each house has a
  [Raspberry Pi](https://www.raspberrypi.com/): a computer about the size of a
  bank card that costs less than a phone. It makes its own WiFi network.
* **No app to download.** Anyone in the house joins that WiFi. There's no
  password. A page opens on its own, just like hotel or café WiFi.
* **Feels like WhatsApp.** You can message people in your house and in other
  houses. There are groups such as *Medical* and *Food*, and one channel for
  everyone. Houses close enough to each other can also make voice calls.
* **Houses connect to each other on their own.** Nearby houses (within a few
  hundred metres) link over WiFi. Houses further away (roughly 1–3 km) link
  over a small long-range radio added to the Pi. That radio carries text only,
  but a spoken message can be turned into text, sent, and read aloud at the
  other end.
* **Someone in each house approves new accounts,** so strangers can't join
  just by walking past.

## What you'll need

This list is the current plan. It may change before the first release, and
we'll add prices when it does ship.

| For each house | Why |
|---|---|
| A Raspberry Pi 4 or 5 | Runs the house network and the chat |
| Its power supply and a memory card (microSD, 32 GB) | The memory card holds batnet |
| A USB WiFi stick | One WiFi radio serves phones in the house, the other talks to neighbouring houses |
| *Optional:* a LoRa radio board for the Pi (an "SX1262 HAT") | Sends text to houses 1–3 km away. Only some houses need one |
| *Later:* a battery or a small solar panel | Keeps it running when the power is out |

Everyone else only needs a phone, tablet or laptop with a web browser.

If you have an older or smaller Pi (a Zero or a Pi 3), it can still help. It
works best as a relay, passing the signal between houses that are too far
apart to reach each other.

## How setting it up will work

This is the plan for the first release. These steps don't work yet.

1. **Copy batnet onto the memory card.** You'll use the free
   [Raspberry Pi Imager](https://www.raspberrypi.com/software/) on any computer.
2. **Put the card in the Pi and plug it in.** Plug the WiFi stick in too, and
   fit the radio board if you have one.
3. **Join the Pi's WiFi from your phone.** A set-up page opens. Name your house
   (for example "Miller House") and create the first account. That account
   approves everyone else in your house.
4. **Do the same at the next house.** Houses in range find each other
   automatically.

## How your household will use it

1. Join the WiFi called **batnet · *your house name*** (no password).
2. A page pops up. Follow it to open your normal web browser.
3. **The browser shows a "not private" warning, once.** This is expected:
   without the internet, the Pi can't get the usual security certificate. The
   page shows you how to continue, and the connection is still encrypted.
4. Create an account and wait for your house's approver to let you in.
5. Start chatting.

---

## Try the simulator

![The batnet village simulator: a map of houses linked by WiFi and radio, beside a phone showing the batnet welcome page](docs/img/village-sim-light.png#gh-light-mode-only)
![The batnet village simulator: a map of houses linked by WiFi and radio, beside a phone showing the batnet welcome page](docs/img/village-sim-dark.png#gh-dark-mode-only)

The simulator is a pretend village that runs entirely in your web browser. You
can:

* **Watch messages travel** between houses over WiFi and over the long-range
  radio.
* **See the app through any villager's phone:** joining, signing up, chatting,
  and the list of people in reach.
* **Change the village:** more houses, bigger households, fewer radios. You can
  also cut a house's power and watch what still gets through.

**Open it online:** <https://johnusher.github.io/batnet/>

**Or run it on your own computer, offline:**

1. At the top of this page, click the green **Code** button, then
   **Download ZIP**.
2. Unzip the file you downloaded.
3. Open the folder `prototypes`, then `village-sim`, and double-click
   `index.html`. It opens in your web browser.

The numbers in the simulator are estimates from a model, not measurements from
real radios. What we've learned from it so far is in
[the simulator findings](knowledge/research/village-sim-findings.md).

---

## For developers

The project's research, decisions and hardware notes are in
[`knowledge/`](knowledge/). Start with the [vision](knowledge/vision.md) and the
[decisions](knowledge/decisions/). The knowledge is stored as an
[Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
bundle, which both people and coding agents can read. Instructions for agents
are in [AGENTS.md](AGENTS.md).

**Set up a clone.** You need `git`, [Go](https://go.dev/dl/) 1.22 or newer, and
[gitleaks](https://github.com/gitleaks/gitleaks#installing):

```sh
git clone https://github.com/johnusher/batnet.git
cd batnet
git config core.hooksPath .githooks          # secret scan + knowledge check on every commit
cd tools/okflint && go run . ../../knowledge  # check the knowledge bundle by hand
```

This repository is public, so never commit keys, passwords, real names or
precise locations. See [secret hygiene](knowledge/security/secret-hygiene.md).

**Licence:** [MIT](LICENSE). You're free to use, copy, change and share batnet.
