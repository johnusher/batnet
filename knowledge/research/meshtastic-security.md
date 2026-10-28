---
type: Research Note
title: What Meshtastic does for privacy, keys and trust - and what batnet must do itself
description: Meshtastic's two layers - channels under one shared AES key that anyone holding it can read and forge, and direct messages under X25519 and AES-CCM since 2.5 - with signed broadcasts since 2.8, no chain of trust (keys trusted on first sight), bare headers, and the 2025 key advisories; what that leaves for batnet's own layer.
tags: [meshtastic, security, encryption, keys, trust, privacy]
status: draft
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-24T12:00:00Z }
stale_after: 2027-04-24
sources:
  - id: mt-enc
    resource: https://meshtastic.org/docs/overview/encryption/
    title: Meshtastic docs - Encryption
  - id: mt-enc-tech
    resource: https://meshtastic.org/docs/development/reference/encryption-technical
    title: Meshtastic docs - Encryption, technical reference
  - id: mt-sec-cfg
    resource: https://meshtastic.org/docs/configuration/radio/security/
    title: Meshtastic docs - Security configuration
  - id: mt-pkc-blog
    resource: https://meshtastic.org/blog/introducing-new-public-key-cryptography-in-v2_5/
    title: Meshtastic blog - Introducing new public key cryptography in v2.5 (2024-09-12)
  - id: ghsa-keys
    resource: https://github.com/meshtastic/firmware/security/advisories/GHSA-gq7v-jr8c-mfr7
    title: Meshtastic firmware advisory GHSA-gq7v-jr8c-mfr7 - Repeated public/private keypairs (CVE-2025-52464, 2025-06-18)
  - id: cve-downgrade
    resource: https://cvedb.shodan.io/dashboard/cve/CVE-2025-53627
    title: CVE-2025-53627 - a direct message without the pki_encrypted flag falls back to the channel key
---

# The question

A friend's remark on the project - "maybe needs a privacy layer, key exchange
mechanisms, chain of trust" - raised what the radio layer already gives us.
This note records what Meshtastic does, as of its 2.7 and 2.8 firmware, so that
batnet's own security design ([B50](/known-problems.md), [B51](/known-problems.md),
plan 3b) builds on facts. The bench hubs run meshtasticd 2.7.x
([LoRa radios](/hardware/lora-radios.md)).

# Two layers

Meshtastic encrypts in two different ways, and only one of them involves a key
exchange.

## Channels: one shared key, no exchange

* Every channel has a pre-shared key (PSK). Messages are AES-CTR; a 16-byte key
  gives AES-128, a 32-byte key AES-256, an empty key no encryption [mt-enc]
  [mt-enc-tech].
* The default channel's key is published (`AQ==`), so the public mesh is in
  effect plain text: "anyone can read traffic on it" [mt-enc]. A private channel
  is as private as the QR code that carries its key.
* The nonce is the 64-bit packet ID, the sender's node number and a block
  counter [mt-enc-tech].
* AES-CTR has no integrity check: anyone holding the key, or guessing a message's
  content, can alter or forge a packet, claiming to be any node, and the default
  policy accepts it. The docs say to "treat unsigned channel messages as claims,
  not proof" [mt-enc].
* Since firmware 2.8.1 a channel can switch to AES-CCM ("Use AEAD", experimental,
  off by default): a 12-byte tag per packet, the sender and destination numbers
  authenticated. All nodes on the channel must agree on the setting [mt-enc]
  [mt-enc-tech].
* No forward secrecy and no revocation: a recorded night is readable by whoever
  gets the key later; the only way to remove someone is a new key for everyone.
  The docs ask for periodic key changes and no private channels on unattended
  nodes [mt-enc].

## Direct messages: real public-key cryptography since 2.5

* Each node makes an X25519 key pair once its LoRa region is set
  [mt-sec-cfg]. Its public key travels in its NodeInfo broadcast; a node will
  not send a direct message to a node whose key it lacks [mt-enc]. (That is the
  *error 39: no key* the roof hub's reach test met, [log](/log.md) 2026-10-23.)
* The sender's private key and the recipient's public key give an X25519 shared
  secret, hashed with SHA-256 into a 32-byte key, recomputed per packet. The
  message is AES-256-CCM with an 8-byte tag and 4 random nonce bytes, 12 bytes
  extra on the air; channel hash 0 and the `pki_encrypted` flag mark it
  [mt-enc-tech].
* Only the two can read it, and the recipient knows it came from the other
  holder of the shared key. Direct messages are not signed: either party could
  have made the tag, so it proves nothing to a third party [mt-enc-tech].
* Before 2.5 (September 2024), direct messages were channel-encrypted, so
  everyone on the channel could read them; to a node on 2.4.3 or older they still
  are [mt-pkc-blog] [mt-enc].
* Since 2.8.1 an acknowledgement can carry a proof, made with the shared key,
  that the recipient really got the message [mt-enc].

## Signed broadcasts since 2.8

* Broadcasts are signed with XEdDSA, an Ed25519 signature from the same X25519
  key pair; 66 bytes on the air, so a packet too big for them goes unsigned
  [mt-enc-tech].
* A receiver that knows the sender's key drops a packet whose signature fails.
  Unsigned packets follow the *packet signature policy*: the default accepts
  them; BALANCED and STRICT reject unsigned broadcasts from nodes known to sign
  [mt-enc] [mt-sec-cfg].
* STM32WL nodes neither sign nor verify [mt-enc].

# Chain of trust: there is none

* Keys are trusted on first sight. "Nodes trust the first public key they
  receive for a number, since there's no central authority" [mt-enc]. Nothing
  signs keys, there is no web of trust, no directory, no revocation list.
* Identity is the key: a node's number is the CRC-32 of its public key (before a
  key exists, from the MAC). A new key is a new node. The firmware checks that
  the number matches the key only for a signed first-contact NodeInfo
  [mt-enc-tech].
* So an impostor can present a new key under a new number with someone else's
  name, and the docs' answer is "confirm a recipient's public key through another
  channel before sending anything sensitive" [mt-enc].
* The one place keys are checked against a list is remote administration: a node
  accepts admin messages only as encrypted direct messages from up to three
  listed public keys, each change carrying a session passkey younger than five
  minutes against replay [mt-sec-cfg] [mt-pkc-blog]. *Managed mode* locks the
  node's settings to those keys.

# What is never hidden

* Packet headers are clear so relays can forward what they cannot read: sender,
  destination, packet ID, hop counts, channel hash. Who talks to whom, when and
  how much is open to anyone listening [mt-enc].
  The roof hub shows it (2026-10-27): in 35 hours its log named the sender
  and recipient of private messages it could not read for 118 pairs of
  radios seen three times or more, 6 of them writing both ways, from the
  headers of copies too garbled to read alone. batnet keeps such pairs off
  every page and report; counts only.
* A node decrypts before handing a packet to its client over Bluetooth, serial
  or TCP [mt-enc]: meshtasticd gives batnetd plain text, which is why a hub's
  own security is its own problem.
* The docs' own verdict: not comparable to WPA3, TLS 1.3 or Signal; no forward
  secrecy; X25519 not quantum-resistant and nothing that is fits a LoRa packet
  [mt-enc].

# The 2025 key advisories

* **Repeated and weak keys** (CVE-2025-52464, critical, June 2025): several
  vendors' flashing procedures cloned the same key pair onto many devices, and
  the crypto library's randomness pool was not initialised on some boards, so
  keys were weak. Direct messages from such a node can be read by anyone holding
  the list of bad keys; a bad key as a remote admin hands the node over. Fixed in
  2.6.11 (keys made only once the region is set, a warning on a known bad key,
  more randomness) and 2.6.12 (known bad keys wiped) [ghsa-keys]. Old keys
  linger on nodes never updated, and the mesh is full of those.
* **Downgrade** (CVE-2025-53627): a direct message arriving without the
  `pki_encrypted` flag fell back to the channel key; fixed in 2.7.15
  [cve-downgrade].

# What this means for batnet

batnet's private channel ([0006](/decisions/0006-private-and-public-channels.md))
rides the channel layer: one key for the village, shared by QR, readable and
forgeable by anyone holding it, no forward secrecy, no way out but a new key.
That matches [B51](/known-problems.md) exactly: "within the village key, anyone
is anyone". Meshtastic's direct-message layer is node-to-node between radios,
not between people, and batnet's people are phones behind a hub, so it does not
carry batnet's identities either.

What Meshtastic gives us, then, is confidentiality of the channel against
outsiders and a working pattern to copy: a key pair per node, the public key
announced, a shared secret per pair, admin rights as a short list of public
keys. What it leaves to batnet is everything the friend named:

* **A privacy layer between people:** batnet already seals traffic with the
  village key (`internal/seal`, XChaCha20-Poly1305 with HKDF) and signs every
  ledger entry with the node's ed25519 key ([spec §4](https://github.com/johnusher/batnet/blob/main/docs/specs/2026-10-15-m1-design.md)).
  Person-to-person secrecy inside the village, and HTTPS on the open WiFi
  ([B50](/known-problems.md)), are not built.
* **Key exchange:** today the village key is copied by hand ([B37](/known-problems.md));
  the spec's QR with a one-time join token is the plan. Nothing exchanges keys
  per person or per house yet.
* **A chain of trust:** nothing ties a person's name to a key, an account to a
  house, or a house to an admin's approval (`house.join` and `house.approve`
  exist but are never applied, [B51](/known-problems.md)). A village needs at
  least: the admin's key as the root, houses admitted by a signature from it,
  people vouched for by their house, and a way to revoke a lost phone or a
  stolen Pi without re-keying the village.

These are gathered as one idea in [ideas](/ideas.md) under *Admin and village*,
to become a decision when picked.
