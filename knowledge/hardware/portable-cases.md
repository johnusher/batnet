---
type: Hardware
title: 3D-printable cases and festival meshes
description: Printable cases for pocket and house nodes with LoRa antennas, and where LoRa meshes have run at festivals.
tags: [portable, 3d-printing, cases, festival, pi-zero]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T05:46:37Z }
stale_after: 2027-04-15T00:00:00Z
sources:
  - id: p909529
    resource: https://www.printables.com/model/909529-meshtastic-raspberry-pi-zero-2-w-based-node-with-g
    title: Meshtastic Raspberry Pi Zero 2 W based node, with GPS
  - id: p1750503
    resource: https://www.printables.com/model/1750503-pi-zero-2w-rak6421-hat
    title: Pi Zero 2W + RAK6421 HAT base plate
  - id: p1722952
    resource: https://www.printables.com/model/1722952-meshtastic-enclosure-for-meshadv-pi-hat-on-raspber
    title: Mast-mount enclosure for MeshAdv Pi Hat on Pi Zero 2 WH
  - id: p1468595
    resource: https://www.printables.com/model/1468595-haven-case-for-raspberry-pi-based-manet-by-paralle
    title: Haven - case for a Raspberry Pi based MANET
  - id: p1578286
    resource: https://www.printables.com/model/1578286-haven-node-raspberry-pi-5-double-antenna
    title: Haven Node Raspberry Pi 5, double antenna
  - id: mw2128181
    resource: https://makerworld.com/en/models/2128181-openmanet-radio-case
    title: openMANET Radio Case (Pi 4 + 3x18650 UPS)
  - id: p771172
    resource: https://www.printables.com/model/771172-rugged-case-for-raspberry-pi
    title: Rugged Case for Raspberry Pi (with 10400 mAh battery)
  - id: p301926
    resource: https://www.printables.com/model/301926-100mm-radome-for-meshtastictbeamrak-wisbocksdrrasp
    title: 100 mm radome with painter's-pole mount
  - id: burningmesh
    resource: https://www.burningmesh.org/
    title: Burning Mesh
  - id: bwb
    resource: https://burnerswithoutborders.org/projects/meshtastic-meets-burning-man/
    title: Meshtastic meets Burning Man
  - id: festmesh
    resource: https://github.com/adamsthws/Festival_Mesh_Networking
    title: Festival_Mesh_Networking
---

Model pages checked through the Printables and MakerWorld public APIs on
2026-10-15. No official RAK or Meshtastic enclosure exists for a Pi HAT. Most
designs are non-commercial licences (fine to link, not to sell prints).

# Cases

| Use | Design | Fits |
|---|---|---|
| Pocket node | Meshtastic Pi Zero 2 W node with GPS[^p909529] | Zero 2 W + Waveshare SPI LoRaWAN HAT, SMA bulkhead |
| Pocket node | Pi Zero 2W + RAK6421 base plate[^p1750503] | Zero 2 W + RAK6421 (our recommended HAT); a plate, not a closed case |
| Pole / outdoor | Mast-mount MeshAdv enclosure[^p1722952] | Zero 2 WH, sealed, cable gland |
| House / field | Haven[^p1468595] (most popular, CC0) | Pi 4 + HAT stack, SMA through the top |
| House / field | Haven Node Pi 5 remix[^p1578286] | Pi 4 or 5 + UPS HAT, two antenna ports |
| Self-powered | openMANET Radio Case[^mw2128181] | Pi 4 + 3×18650 UPS; licence forbids re-hosting, link only |
| Pi + power bank | Rugged Case[^p771172] | Pi 4/3/Zero 2 W + 10,400 mAh battery; drill an SMA hole |
| Pole / tent | 100 mm radome[^p301926] | painter's-pole thread; needs a custom Pi insert |

Cases are made for a specific board: the Waveshare UART and SPI SX1262 boards
differ in size, so match the case to the radio.

# Festivals

* Burning Man has a citywide Meshtastic network, Burning Mesh, with event
  firmware.[^burningmesh] One participant's four solar nodes in 2025 saw 1,606
  nodes on the playa.[^bwb]
* A UK guide reports EU868 ShortFast settings tested with more than 100 nodes at
  Shambala festival (author's claim).[^festmesh]
* No source found for Glastonbury.

Consequence for batnet: a festival brings crowded LoRa channels. Pocket nodes
should follow the event's published preset, and the airtime limits in the
[village simulator findings](/research/village-sim-findings.md) matter more there.
