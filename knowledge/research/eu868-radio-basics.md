---
type: Research Note
title: EU868 radio basics - band, rules, antennas, range, history
description: The facts behind the README's radio section - the 863-870 MHz slices and limits, Meshtastic's LongFast parameters, ERP and antennas, Fresnel zone and horizon, range records, and the history of the bands and LoRa.
tags: [lora, meshtastic, eu868, legal, antenna, range, history]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-15T10:00:00Z }
stale_after: 2027-04-15T00:00:00Z
sources:
  - id: srd
    resource: https://en.wikipedia.org/wiki/Short-range_device
    title: Short-range device (EU 863-870 MHz table from ERC/REC 70-03)
  - id: erc7003
    resource: https://docdb.cept.org/document/845
    title: ERC Recommendation 70-03 (Tromsø 1997 and subsequent amendments) relating to the use of short range devices
    author: team:cept-ecc
  - id: mesh-radio
    resource: https://meshtastic.org/docs/overview/radio-settings/
    title: Meshtastic radio settings (presets, EU_868 slot)
  - id: mesh-lora
    resource: https://meshtastic.org/docs/configuration/radio/lora/
    title: Meshtastic LoRa configuration (regions, duty cycle, ham mode)
  - id: mesh-fw
    resource: https://github.com/meshtastic/firmware/blob/master/src/mesh/RadioLibInterface.h
    title: Meshtastic firmware - sync word 0x2b, 16-symbol preamble; RadioInterface.cpp - SNR-weighted rebroadcast delay
  - id: mesh-range
    resource: https://meshtastic.org/docs/overview/range-tests/
    title: Meshtastic range tests and records
  - id: lwn
    resource: https://lwn.net/Articles/1009782/
    title: LWN - Meshtastic (331 km record between northern Italy and Slovenia; ham mode)
  - id: ism
    resource: https://en.wikipedia.org/wiki/ISM_radio_band
    title: ISM radio band (1947 Atlantic City; FCC spread spectrum rules of 9 May 1985)
  - id: semtech-8k
    resource: https://www.sec.gov/Archives/edgar/data/0000088941/000115752312001273/a50196224ex99_2.htm
    title: Semtech 8-K exhibit - acquisition of Cycleo SAS, 7 March 2012
    author: vendor:semtech
  - id: meshtastic-wiki
    resource: https://en.wikipedia.org/wiki/Meshtastic
    title: Meshtastic (created by Kevin Hester in early 2020)
  - id: ttn-sf
    resource: https://www.thethingsnetwork.org/docs/lorawan/spreading-factors/
    title: The Things Network - spreading factors and sensitivity
---

# The band

* Europe's 863–870 MHz short-range-device band is licence-free, sliced with a power and duty limit per slice.[^srd][^erc7003] Main slices: 865–868 MHz 25 mW ERP 1%; 868.0–868.6 MHz 25 mW 1% (LoRaWAN 868.1/868.3/868.5); 868.7–869.2 MHz 25 mW 0.1%; **869.4–869.65 MHz 500 mW ERP, 10% duty cycle or LBT+AFA**; slivers at 868.6–868.7, 869.2–869.4 and 869.65–869.7 MHz for alarms; 869.7–870 MHz 5 mW voice or 25 mW 1%.[^srd] ERC/REC 70-03 has been amended many times, most recently in 2026; check the current Annex 1 before relying on these.
* Meshtastic EU_868 = 869.4–869.65 MHz, one LongFast slot centred on 869.525 MHz, 27 dBm ERP, 10% hourly duty cycle on a rolling hour; the radio stops sending at the limit unless "override duty cycle" is set.[^mesh-radio][^mesh-lora] Turbo (500 kHz) presets are not offered in EU_868.[^mesh-radio]
* LoRaWAN gateways also use 869.525 MHz for their RX2 downlinks, so Meshtastic shares its slot with them.
* Wireless M-Bus meters (T-mode) send at 868.95 MHz.

# LongFast on the air

* SF11, 250 kHz, CR 4/5, 1.07 kbit/s, link budget 153 dB at 22 dBm and 0 dBi.[^mesh-radio] Symbol time 2^11 / 250 kHz = 8.192 ms.
* Preamble 16 symbols (LoRa's default is 8) and sync word `0x2b`.[^mesh-fw] So the preamble alone is ~165 ms.
* Airtime by the LoRa formula (explicit header, CRC, no low-data-rate optimisation): 60 B ≈ 0.68 s, 100 B ≈ 1.0 s, 150 B ≈ 1.38 s. This supersedes the ~1.2 s for 150 B in [LoRa airtime](/research/lora-airtime-eu868.md), which left out the longer preamble.
* Managed flooding: a node that hears a new packet waits a contention window scaled by SNR, low SNR (far away) shortest, and drops its repeat if it hears another node repeat first.[^mesh-fw] Hop limit 3 by default, 7 at most.[^mesh-lora]
* Decoding: the receiver multiplies by a falling chirp (dechirp), turning each symbol into a steady tone whose bin is the value, then runs a 2^SF-point FFT over 2^SF samples at the bandwidth's sample rate. Processing gain 10·log10(2^SF): 33 dB at SF11. A time offset and a frequency offset both shift the bin; up-chirps give time + frequency, the sync's down-chirps give frequency − time, which separates them. Semtech's datasheets give a demodulation floor of about −17.5 dB SNR at SF11 (from memory, not opened here). Explained in the README's hobbyist section with `dechirp` and `sync` diagrams.
* Sensitivity at 125 kHz runs from −123 dBm (SF7) to −137 dBm (SF12); each SF step halves the rate.[^ttn-sf]

# Power, ERP and antennas

* `ERP(dBm) = P(dBm) + G(dBi) − cable loss − 2.15`. Limit 27 dBm ERP. A 22 dBm SX1262 with a 2 dBi whip ≈ 21.9 dBm (153 mW) ERP; with 6 dBi ≈ 25.9 dBm (385 mW); above about 7 dBi plus cable loss, lower the power.
* CEPT's decision for the 500 mW slice expects an integral or dedicated antenna (ERC/DEC/(01)04, quoted in search results, not opened). Hobbyists swap antennas freely; the ERP limit is what we tell people to keep to.
* Quarter wave at 869.5 MHz = 8.6 cm; a radiator is usually cut ~5% shorter.
* Cable loss at 900 MHz, typical datasheet values: RG174 ≈ 0.9 dB/m, LMR-400 ≈ 0.13 dB/m. Not checked against a specific datasheet here.
* Ham mode: other frequencies and more power for licensed amateurs, but encryption off and call sign sent every ten minutes.[^lwn][^mesh-lora]

# Range

* Free-space path loss at 869.5 MHz: 111 dB at 10 km. A 153 dB budget is >1,000 km in free space, so terrain, the horizon and clutter set real range.
* Radio horizon `d ≈ 4.12(√h1 + √h2)` km: 2 m antennas ≈ 12 km, 10 m ≈ 26 km.
* First Fresnel zone radius at mid-path `r ≈ 8.66√(d_km / f_GHz)` m: 13.1 m on a 2 km link at 869.5 MHz.
* Records: ground 331 km on 868 MHz, Very Long Slow (BW 62.5, SF12, CR 4/8), RAK4631 boards with external antennas, mountaintops in northern Italy and Slovenia; earlier 254 km on 915 MHz LongFast; air 206 km by balloon on stock antennas.[^mesh-range][^lwn]
* The README's range bands (0.5–2 km pocket in town … 10–50 km mast or hilltop) are rules of thumb, as in [radio links compared](/research/radio-links-compared.md). Replace them with field tests.

# History

* 1947: ISM bands set at the ITU conference in Atlantic City, for microwave heating among others.[^ism]
* 9 May 1985: the FCC allows unlicensed spread spectrum in the ISM bands, after Michael Marcus proposed it in 1980.[^ism] 902–928 MHz is a Region 2 (Americas) ISM band.[^ism]
* 1997: CEPT ERC Recommendation 70-03 ("Tromsø 1997") becomes Europe's common SRD rule book.[^erc7003]
* 7 March 2012: Semtech buys Cycleo SAS of Grenoble, whose long-range IP for smart metering became LoRa.[^semtech-8k] The LoRa Alliance was founded in 2015 (Semtech's own account).
* Early 2020: Kevin Hester creates Meshtastic, with hikers and paragliders in mind.[^meshtastic-wiki]

# Pictures

The README's diagrams come from `docs/img/radio/make.py` using the numbers above. The waterfall diagram is an illustration, not a recording. The two photos are from Wikimedia Commons: a Meshtastic waterfall by Tinux (CC0) and a rooftop node by -stk (CC BY-SA 4.0), credited under each picture.
