---
type: Decision
title: "0033: The away note (an automatic answer to a private message the owner has not answered)"
description: When someone writes to the hub alone and the owner has not answered after 15 minutes, the hub sends one short note in simple German that says it is automatic; once per radio a week, at most 10 a day, never on a channel, never to a bare acknowledgement, a machine's message or a radio that opted out; off unless switched on in the panel.
tags: [hub, radio, etiquette, messages]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-24T12:30:00Z }
---

# Context

The owner wants to make friends on Berlin's mesh. Someone who writes to an
unknown radio and hears nothing rarely writes again, and the owner is not
always at the panel. The hub already chimes, blinks and wakes its screen for
a private message ([0018](/decisions/0018-notifications-and-house-speaker.md)),
but the roof hub has no speaker or LED and nobody sits by it. On the mesh
batnet is a guest ([etiquette](/research/mesh-etiquette.md)): an automatic
answer must be honest, rare and never start a loop with another machine.

# Decision

* **After a wait, if nobody answered.** A private text to this hub starts a
  wait (15 minutes, or 5, 30 or 60 in the panel). Anything the hub sends to
  that radio in the meantime ends it: the owner's answer, a reach test's
  message, or the note itself. The note is a reply to their latest message.
* **The note**, unless the owner writes their own (up to 200 bytes):
  "Hallo! Das ist eine automatische Antwort. Ich bin gerade nicht am
  Funkgerät. Ich schreibe dir bald zurück. Liebe Grüße" (122 bytes). Simple
  German, plain that it is automatic, no link: batnet and its repository are
  for when someone asks.
* **Rare.** Once per radio a week, kept across restarts in
  `/var/lib/batnet/away.json` (radio IDs and times only, on the hub); at
  most 10 notes a day; a note the radio could not send within two hours of
  the wait is dropped.
* **Not to**: a channel or group message; a message of only
  acknowledgements, greetings with them, emoji or signs ("ok", "danke dir",
  "Hallo, angekommen!", 👍; a bare "Hallo" does get one); a message that
  looks automatic (a status post, another away note, a bot); a radio in the
  reach test's `noContact` list.
* **Off unless switched on**, in Settings, Away note, with the wait and the
  text.

# Consequences

* A stranger's first private message gets an answer within the wait while
  the owner is away; the owner still answers themselves, which is what
  makes friends.
* Airtime: at most one 122-byte message per radio a week.
* Two hubs with the note on answer each other at most once a week each, and
  the second sees "automatische Antwort" and does not answer.
* Not yet: telling the owner's phone when someone writes (a push through
  ntfy or the like means a service on the internet;
  [ideas](/ideas.md)).
