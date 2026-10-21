---
type: Decision
title: "0027: The Meshtastic preset follows the local mesh"
description: The hub's Meshtastic preset is a village setting chosen from what the people nearby use (LongFast where nothing says otherwise); in a city split between presets, the village picks the one its neighbours are on, and a second radio can listen to the other.
tags: [lora, meshtastic, presets, interop]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-21T15:00:00Z }
---

# Context

[Decision 0008](/decisions/0008-longfast-single-radio.md) puts both channels on
one radio on LongFast. Berlin's Meshtastic users are split between LongFast and
MediumFast ([research](/research/meshtastic-presets-berlin.md)), and the two do
not hear each other. A hub on the wrong one is alone on the public channel.

# Decision

* **The preset is a village setting**, set once at village set-up and the same
  on every gateway of the village (the private channel needs it too). The
  default stays LongFast, Meshtastic's own default and the one most of Europe
  uses.
* **The hub measures before the village chooses:** it listens on each EU
  preset in turn (a roving second radio, or the one radio for an hour at a
  time) and the panel shows how many radios each preset heard.
* **A second radio, where there is one, listens on the other preset**: receive
  only, so a village on MediumFast still hears the LongFast radios nearby.
* When accepted, this supersedes 0008's "LongFast by default" in part: 0008's
  one-radio design stays, its preset becomes this setting.

Splitting a mesh has a cost: in one city a move to ShortFast left the edges
and the LongFast users cut off ([research](/research/meshtastic-presets-berlin.md)).
A village follows its neighbours' preset; it does not start a new one.

# Revisit when

Meshtastic gains a way to bridge presets on one radio, or a region agrees on
one preset.
