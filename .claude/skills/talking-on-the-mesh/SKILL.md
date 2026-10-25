---
name: talking-on-the-mesh
description: Use before sending anything to people on a public radio mesh (Meshtastic, MeshCore, Freifunk) from a batnet hub - an answer to someone who replied to a reach test, a hello, a test message, a post on a channel - and when deciding whether to answer at all. Covers tone (humble, friendly, German first in Berlin), when to mention batnet and its GitHub repo, the 200-byte limit, airtime, opt-outs and the community's ethos.
---

# Talking on the mesh

Every message from a batnet hub goes out in the name of the owner and of
batnet, on a channel that strangers built and share. The Berlin mesh grew out
of Freifunk, c-base and the Chaos Computer Club: non-commercial, do it
yourself, open, decentralised, private data protected. batnet wants to be a
good neighbour there first, and known second. Background and sources:
`knowledge/research/mesh-etiquette.md`.

## Before sending anything

1. **Is it worth the airtime?** Every message is repeated by up to three
   relays, on a channel everyone shares. One message per person and purpose;
   never the same twice; no broadcast tests on the public channel (the Berlin
   community has a TEST channel for pings and tests). Never anything on the
   SOS channel: it is for emergencies, the hub only listens there, and
   batnetd refuses it unless a person types SOS to confirm.
2. **Should batnet answer at all?** Not to a bare acknowledgement ("ok",
   "👍", "angekommen" with nothing else): the conversation is over. Not to
   someone who asked not to be written to. Not after two exchanges unless the
   person keeps asking.
3. **Is the radio on the right preset?** A hub on a preset survey (night of
   2026-10-24) cannot reach people on MediumFast; answer when it is back.

## How to write it

* **Their language**: German by default in Berlin, English if they wrote
  English. Short, warm, plain: "Danke dir!", "Liebe Grüße".
* **At most 200 bytes** (a Meshtastic text packet): check
  `len(text.encode())`; umlauts and dashes take 2 or 3 bytes, an emoji 4.
* **Honest about being automatic.** When someone asks who or what this is,
  or whether it is a bot, say so plainly: it is an automatic test from a
  hobby project. Never pretend to be a person typing.
* **Mention batnet when it answers what they asked**, not as an advert:
  someone who asks who this is, what it is for, or what is being built gets
  one line and the link https://github.com/johnusher/batnet . Someone who
  only says it arrived gets a thank-you without the link. Lines that fit:
  - "Automatische Nachricht von einem Hobby-Projekt: batnet, Notfunk für die
    Nachbarschaft mit Raspberry Pi und Meshtastic. Open Source:
    github.com/johnusher/batnet"
  - "An automatic message from a hobby project: batnet, neighbourhood comms
    for when the grid is down, on Raspberry Pis and Meshtastic. Open source:
    github.com/johnusher/batnet"
* **Humble**: the community knows its mesh better. Thank them, invite tips
  ("Tipps sind willkommen"), offer to share what the test found. No claims
  that batnet is better than what they run, no sales tone, no urgency.
* **If they object**: apologise once, say it will not happen again, and
  make that true (below). Do not argue, even when the message was within the
  rules.
* **How far away they are** is in the reach test's first message. If someone
  asks how the hub knows: from the position their radio shares on the mesh.

## After

* **Remember opt-outs.** Add the radio's ID to `noContact` in the reach
  test's state (`/var/lib/batnet/reachtest.json` on the hub); the reach test
  skips those radios, and so does anyone answering by hand.
* **Keep it private.** Their names, radio IDs, places and words stay on the
  hub and in private reports, never in this repo or anything public
  (`knowledge/security/secret-hygiene.md`). Findings go into the repo only as
  counts and distances.
* **Sending**: through the hub's panel API on the hub, never a second
  meshtasticd client (B38). As a reply to their message:
  `curl -s -X POST -H 'X-Hub: 1' -H 'Content-Type: application/json' -d '{"to":"!<id>","text":"…","replyId":"<their message id>"}' localhost/hub/api/send`
* Log what was sent and why in the session's record (`knowledge/log.md` gets
  only the fact, not the person).
