---
type: Test Report
title: "2026-10-29: why the roof hub rebooted itself, and whether its signal fell"
description: The roof hub rebooted with no shutdown three times in three days. Cause - meshtasticd 2.7's library for the Meshstick's USB chip strands a thread's stack at every radio interrupt, about 12 MB more memory an hour, until the Zero 2 W stalls past its 15 s watchdog; a guard now restarts meshtasticd in time. The signal had not fallen - each nearby relay within about 2 dB over three days, the strongest still near 0 dB; most traffic arrives at -9 to -14 dB, normal for a city mesh. A channel message called "not delivered" had been answered.
tags: [field, roof, meshtasticd, meshstick, memory, watchdog, signal, zero-2-w]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-29T20:30:00Z }
sources:
  - id: issue-10468
    resource: https://github.com/meshtastic/firmware/issues/10468
    title: "meshtastic/firmware #10468 - meshtasticd (portduino/Linux) VSZ and swap grow without bound under sustained load"
  - id: libch341-2
    resource: https://github.com/meshtastic/libch341-spi-userspace/pull/2
    title: "libch341-spi-userspace #2 - detach the poll thread when deattach_interrupt runs on it, don't strand its stack"
---

# Setup

The roof hub [pz7](/hardware/bench-pz7.md): a Raspberry Pi Zero 2 W (416 MB)
with a Meshstick (CH341 USB-to-SPI, SX1262), meshtasticd 2.7.26, MediumFast,
antenna on the roof since 25 October. The owner asked whether the antenna had
fallen over (the signal looked low, around -9 dB), and why a message to the
public channel said *Not delivered* when a bot had answered it. Sources:
meshtasticd's journal from 27 October 01:09 to 29 October 21:10, the
panel's status, `/proc` on the hub, and the weekly stats
(`tools/meshstats`).

# The signal

| | 27 Oct | 28 Oct | 29 Oct |
|---|---|---|---|
| Median SNR of unreadable copies (3-hour steps) | -12.0 to -13.5 dB | -10.2 to -12.2 dB | -10.2 to -13.8 dB |
| Packets read per 3 hours | 289 to 476 | 408 to 510 | 435 to 523 |
| Noise floor | -110.2 dBm | | -110.2 dBm |

* **Each nearby relay held its level.** The copies' `relay=` byte names the
  radio that sent each copy; the seven busiest stayed within about 2 dB of
  themselves in 6-hour steps over three days. The strongest stayed between
  -2.5 and 0 dB; one went from about -13.5 to -9 dB. A fallen antenna would
  have taken several dB off all of them at once.
* **-9 dB is normal here.** Most of what the hub hears comes from radios
  kilometres away, through relays, at -9 to -14 dB; MediumFast reads down to
  about -15 dB. Only a strong close neighbour arrives near 0 dB, as the
  strongest did right after the antenna went up
  ([antenna raised](/tests/2026-10-25-roof-antenna-raised.md)). 13 radios were
  heard straight, against 7 the week before.

# The reboots

* **Three reboots with no shutdown**: 27 October 16:58 and 29 October 04:42,
  with nobody working on the hub, and 29 October 21:19, during a
  `systemctl daemon-reload` while meshtasticd was being restarted. Each time
  the log simply stops; no undervoltage was flagged afterwards.
* **Before them**, the radio stick's USB timed out (`libusb: async cancel
  failed -5`, 13 times in the two hours before 04:42) and once the WiFi chip
  (`brcmf_sdio_read_control ... failed`). The same USB warnings came while
  3 to 4 days of journal were read out on the hub.
* **meshtasticd held 200 MB after 16.6 hours**: 4 MB in memory and 198 MB
  swapped to zram (compressed about tenfold), in 31,765 anonymous mappings:
  about 15,900 stranded 8 MB thread stacks with their guard pages (130 GB of
  address space), each with about 12 kB touched. Short-lived threads appeared
  every few seconds, a burst with each packet; the memory grew about 12 MB an
  hour.
* **The cause is known upstream**: the CH341 library meshtasticd uses runs the
  radio's interrupt on a poll thread, and when the interrupt is taken off from
  inside that thread, the thread is never joined and its stack stays mapped
  [^issue-10468] [^libch341-2]. Fixed in the library in September 2026, it is
  in meshtasticd 2.8's alphas (from 1 October), not in the stable 2.7.26.
* **How it ends in a reboot**: the two unattended resets came after 18 and
  21 hours of meshtasticd (about 215 and 250 MB). With that much swapped, a
  stall of PID 1 (a reload, a burst of swapping) passes the hardware
  watchdog's 15 s (the most the chip allows) and the Pi is rebooted. A
  restart of meshtasticd at 200 MB set off the third.

# What changed

* `batnet-radio-guard` (`deploy/pi/`, a timer every 10 minutes, installed by
  `setup.sh`): restarts meshtasticd once it holds more than a quarter of the
  Pi's memory, about every 8 hours on the roof, some 15 s off the air; it
  logs a warning when it does. Tested on the hub: meshtasticd restarted, the
  panel reconnected, the packet trace kept writing.
* meshtasticd runs at nice -10 and first for the card; the stats tools read
  the journal at the lowest priority.
* The real fix is meshtasticd 2.8 once it is stable ([to-do](/todo.md)); then
  the mappings should stay flat.

# The message that was answered

The owner wrote *nacht^2* to the public channel. The hub sent it and
listened for a radio nearby passing it on (a channel has no receipts); it
heard none, not even a garbled copy, and after three tries gave up (error 5),
17 s later. 3 s after that a bot two relays away answered *Schlaf gut*. The
panel had said *Not delivered*; it now says *No radio heard passing it on (it
may still have arrived)*, in amber, with the reason
([B80](/known-problems.md)).

# Conclusions

* The antenna is in place; the signal is what it was.
* The reboots were meshtasticd's memory leak with a USB radio stick, not the
  power or the card. The guard keeps meshtasticd small until 2.8 is stable.
* A Pi with more memory leaks the same, only for longer before it matters;
  the guard's limit scales with the memory.
