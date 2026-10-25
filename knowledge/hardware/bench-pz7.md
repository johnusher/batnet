---
type: Hardware
title: pz7, the second bench hub (a Pi Zero 2 W)
description: The Raspberry Pi Zero 2 W with an Elecrow Meshstick that became batnet's second bench hub on 2026-10-23 - a small hub with no screen kiosk and no translator.
tags: [bench, raspberry-pi, zero-2-w, hub, meshtasticd, meshstick, small-pi]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-23T05:30:00Z }
verified: { by: process:2026-10-23-pz7-deploy, at: 2026-10-23T05:20:00Z }
stale_after: 2027-04-23T00:00:00Z
---

"pz7" (Pi Zero 7) is the project's name for this board. It had an earlier use
in another project; its files from then stay in its home folder and nothing of
theirs starts on boot. Its address, login, MACs and radio node ID are private
and stay out of this repo ([secret hygiene](/security/secret-hygiene.md)).

# The machine

| Part | What |
|---|---|
| Board | Raspberry Pi Zero 2 W rev 1.0, 512 MB (416 MB usable), swap on zram |
| Storage | 8 GB microSD: 6.8 GB root. 1.1 GB free after set-up on the desktop image; 3.2 GB free once the desktop was removed |
| OS | Raspberry Pi OS (Debian 13 trixie, arm64), kernel 6.12. Came as the desktop image, booting to text; the desktop was removed on 2026-10-23 (below) |
| LoRa | Elecrow **Meshstick** (CH341, USB ID `1a86:5512`) on the Zero's one micro-USB data port; a USB WiFi stick for batman-adv would need a hub as well. Found by meshtasticd as soon as it was plugged in |
| WiFi | built-in only, on the home network: two profiles, the router (preferred) and its extender (same password, same network) |
| Screen | none: no kiosk on a small Pi |

# What runs

`setup.sh` sees under 1 GB of memory (the kiosk's cut-off is 800 MB, the
translator's 1.5 GB) and leaves out both: no cage,
Chromium or emoji font, no `batnet-screen.service` (so batnetd never starts a
kiosk) and no offline translator. Everything else is as on the
[first bench hub](/hardware/bench-pi7.md): meshtasticd with the Meshstick
config, `batnet-hub.service` (the app on `http://<pi>/`, the panel on
`http://<pi>/hub/`), batman-adv tools, the power-cut kit, watchdogs, the
Meshtastic command-line tool and pmtiles. `BATNET_SMALL=0` or `1` overrides the
guess for both.

Measured on 2026-10-23: 158 MB of 416 MB in use with batnetd and no radio,
152 MB with the radio running, 264 MB available. With no Meshstick, meshtasticd
stops with "Could not initialize CH341 device!" and systemd starts it again
every 5 s; the panel shows no radio.

Under load ([test](/tests/2026-10-23-hub-load-test.md)): 200 pretend phones
on the app in every power mode with no errors, on average at most 18% of the CPU while
chatting, at least 149 MB free; a sign-up takes 1.7 s (2.8 s in low).

# Radio settings

A new Meshstick has no region and will not send ("lora tx disabled: Region
unset"). Set on 2026-10-23 with the Meshtastic command-line tool, as on the
first hub: EU_868, MediumFast, hop limit 3, transmit power the region's default;
node name "lorabatnet2", short name LB2. It heard 16 packets and 6
radios in its first 90 s. The same second channel as the first hub,
**BerlinMesh**, with the key copied from it (not in this repo). It is quiet: the first hub logged 3 messages on it in two days against 48 on the open channel. Since
2026-10-23 it shares the same fixed house position as the first hub (they sit in
one house), as the first hub does: on the primary channel only, rounded to
precision 16 (about 364 m, the nearest Meshtastic step to 350 m), once an hour.
Its primary channel had been at 13 (about 2.9 km); BerlinMesh shares no
position.

The two hubs hear each other straight. On 2026-10-23 a private message from
this hub to the first, sent with the Meshtastic command-line tool, arrived and
was acknowledged (PKI: each had the other's public key from its
name card).

# One village with the first hub

`village.key` was copied from the first hub on 2026-10-23 (by hand, B37), so
both hubs seal and open the same village's log. The key it had made for itself
and its empty log went into a dated folder beside them in `/var/lib/batnet/`;
its own `node.key` stayed.

# The desktop removed

The card came with Raspberry Pi OS's desktop image: Chromium, Firefox, the
desktop, VLC, Thonny, Geany, RealVNC, Imager and the user guide, about 650
packages with what only they needed, 2.2 GB. A hub never starts any of it, so
on 2026-10-23 they were purged, after marking what the hub needs as wanted
(NetworkManager, ssh, avahi, polkit, meshtasticd, Python and pipx, every WiFi
firmware for a USB stick, the kernels, git). Everything ran on and the Pi came
back by itself after a reboot.

# In the loft

On 2026-10-23 it moved to the loft, on mains, under the roof, with its antenna
on top of the house on a 1 m extension; the first hub stays downstairs as the
house hub (screen, translator). Its built-in WiFi reached the home router from
there, measured from a laptop with the same tests as in the router's room:

| | Router's room | Loft |
|---|---|---|
| Router signal | -62 dBm | -75 dBm |
| Extender signal (scan, 0 to 100) | 70 | 44 |
| 30 MB over ssh, down / up | 19 / 18 Mbit/s | 6.1 / 6.7 Mbit/s |
| Ping to the router | no loss | 1 of 30 lost, 6 ms average |
| App page | 200 | 200 in 0.07 s |

Enough for the app and the panel; a map download is slower. NetworkManager
does not move between two network names by signal, so the stronger in the loft
(the router) has the higher autoconnect priority and the extender is the
fallback. Its address differs on each, so reach it by name, not number. In a
power cut the loft socket and the router go off too: it needs a battery of its
own ([power budget](/research/power-budget.md)), and reaches the house hub by
radio only.

The map's zone needs the panel's own place as well as the radio's (B67): set
to the house's on 2026-10-23. The readings it had from the room were moved
aside when the antenna went up (B68), so its zone is the roof's alone.

Its first 90 minutes up there (to 13:40): five radios heard straight, against
three besides the first hub in the room, and four of the five new; the
strongest about 6 dB stronger than anything the room heard. Only one of the
five counts for the zone: two share no place, one only to about 2.9 km at
2.4 km, too rough to say which way. So the zone was one petal of about 1.5 km,
against the first hub's 15 km from a day of readings. Not a poor antenna: the
same radios against the first hub's readings with this antenna before it moved
were as strong or stronger (the strongest 4 dB up, three within 0.5 dB), and it
heard one straight 11 km to the north-west that the first hub never did. It
lacked places: it knew 37 to the first hub's 77, among them not that far one's
(B70). Since 2026-10-23 the hubs share the places their radios heard: a minute
after the change it held 25 places from the first hub, and the radio 11 km away
gave its zone a petal of 11.5 km to the north-west.

The south is its blind side, as it was for the first hub with this antenna:
every radio heard straight with a known place lies between 287° and 327°, and
two to the south-south-west (a Heltec V3 at 3.3 km, a T1000-E tracker at about
6 km) come only through relays. A chimney stands due south of the antenna. At
868 MHz the signal needs about 0.6 to 0.8 m of clearance over an obstacle 1 to
2 m from the antenna (the first Fresnel zone; 60% of it is enough for most of
the signal), so the antenna's lower end should sit at least that far above the
chimney's top. To judge a move: *Start the zone again* afterwards (the readings
before are kept aside), then `tools/hubcompare` against them.

The first hub's readings with this antenna were kept on it as
`/var/lib/batnet/antenna-links-2026-10-23.json` before it went back downstairs
(its live ones now fill with indoor readings). After a day or two on the roof,
`tools/hubcompare/compare.py pz7 pz7:/var/lib/batnet/first-hub-antenna-links-2026-10-23.json` (the first hub's readings, copied to the second as the first was switched off)
compares the radios both heard straight. On 2026-10-23 at 13:50: four in common,
the roof a median 0.2 dB stronger (one 4 dB), one heard only from the roof
(11 km), seven only before (most heard a few times in two days). The antenna stands about 20° off
vertical: about 0.5 dB lost to polarisation, more in some directions if it is
a high-gain one with a flat beam.

On 2026-10-25, after two days up there: five in common, a median 0 dB (from
-2.25 to +1.5 dB), three heard straight only from the roof, six only before.
The zone is a petal of about 13 km to the north-west, and the 81 radios with a
place it heard lie mostly between west and north-north-west
([two days on the roof](/tests/2026-10-25-two-days-on-the-roof.md)).

# How far it reaches: the reach test

Heard is one way; a message back is the other. From 2026-10-23 at 21:00
`tools/reachtest/reachtest.py` (installed on the hub as `batnet-reachtest`, run
every 10 minutes by a timer made with `systemd-run`, which a reboot ends) sends
one friendly private message in German to the next farthest radio: first those
with a known place at least 8 km away, farthest first, then those with no
known place that came through at least two relays; never one that came
through more relays than the hub's own messages may take (hop limit 3), so the
radios six or seven hops away are left out. Each radio gets one message, 30
in all. With a place it says how far: "Du bist etwa 25 km entfernt". A radio
counts as reached when its radio confirms the message, when its name card
comes back after the hub asked for it, or when someone answers. Results:
`sudo batnet-reachtest report` on the hub (it names real radios: not here). It
stopped at 01:55 with 29 tried: none confirmed, two passed on by a relay, 16
out of tries though a neighbour passed them on ([B73](/known-problems.md)), ten
with no key, no answers ([test](/tests/2026-10-24-presets-and-reach-from-the-roof.md)).

The first message, by hand, to a radio 25 km to the north-north-west heard
once through two relays, failed at once with error 39: the hub had never heard
that radio's name card, so it had no key for it ([B72](/known-problems.md)). Of
the 112 radios the hub knew after the change, 87 had a key.

# Who is on the other presets: the night survey

Berlin's mesh is on MediumFast, the hub's home preset; who is left on the
others is not known from here. `tools/presetsurvey/survey.py` (on the hub as
`batnet-presetsurvey`) listens on one preset after another and goes back to
MediumFast at the end, putting the zone's readings back so other presets'
signals do not reach the map. On each preset the hub says one hello that asks
for answers; it sends no messages. Set for the night of 2026-10-24 from 02:00,
55 minutes each on LongFast, LongModerate, LongSlow, MediumSlow, ShortSlow and
ShortFast (ShortTurbo's 500 kHz does not fit the 250 kHz band at 869.525 MHz),
back about 07:45. While it runs the hub cannot hear or answer anyone on
MediumFast, and the reach test pauses. A guard timer (`restore`, at boot and at
08:15, written as a unit file so a reboot does not lose it) brings the radio
back if the survey stops half-way; it goes once the survey is done.

A three-minute trial on LongFast at 21:30 on 2026-10-23 worked end to end (the
change takes about 20 s): the radio received no packet at all, channel use 0%,
as on 2026-10-21 indoors ([presets in Berlin](/research/meshtastic-presets-berlin.md)).
The radio's own log confirmed the panel's 27 dBm is sent as 22 ([B69](/known-problems.md)).
The trial restarted the radio ten seconds after a reach test message had gone
out, so that message may have missed its repeats.

The night itself ([test](/tests/2026-10-24-presets-and-reach-from-the-roof.md)):
LongFast from 02:00 read 5 packets from 2 radios in 55 minutes. At 02:56 the
Meshstick froze as the radio was switched to LongModerate and stayed frozen,
through a restart of meshtasticd and with its saved preset set back, until a
USB reset (`sudo usbreset 1a86:5512`) at 08:01 ([B74](/known-problems.md)). A
copy of the saved config from that night is kept beside it in meshtasticd's
`prefs/` folder. The guard was removed the same morning.

# Its own WiFi, and the logs

On 2026-10-25 it became its own WiFi for 20 minutes and an iPhone joined and
stayed 9 minutes; back on the home WiFi 3 s after being asked
([test](/tests/2026-10-25-pz7-own-wifi-and-health.md)). The same morning's
health check found it healthy, but its logs only in memory (7 hours' worth):
they are now on the card, at most 64 MB ([B78](/known-problems.md)).

# Limits

* No translator and no screen ([L26](/known-problems.md)).
* Offline maps fit only without the desktop: a hub keeps 2 GB free
  ([0032](/decisions/0032-offline-maps.md)), which left 1.1 GB on the desktop
  image and leaves 1.2 GB now, enough for all five areas (about 450 MB).
  Berlin's (87 MB) was fetched on 2026-10-23 and is served with range requests
  like the first hub's; 3.2 GB stayed free.

# Rebuilding it

`deploy/pi/deploy.sh <ssh-host>` from a laptop, as for the first hub. The first
run on this card failed on out-of-date package lists; `setup.sh` now runs
`apt-get update` first.
