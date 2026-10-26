---
type: Research Note
title: Does the weather change what a hub hears? How to find out
description: What is known about weather and 868 MHz LoRa signals (the radio's own temperature matters, rain hardly, warm still air over cold can carry signals far), the free weather data for Berlin, what the hubs already record, and a plan for a study that compares signal strength with the weather.
tags: [lora, meshtastic, weather, propagation, ducting, study, measurement]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-26T12:00:00Z }
stale_after: 2027-04-26T00:00:00Z
sources:
  - id: boano2018
    resource: https://www.scitepress.org/Papers/2018/66056/
    title: Boano, Cattani, Römer - Impact of Temperature Variations on the Reliability of LoRa, an Experimental Evaluation (SENSORNETS 2018)
  - id: luomala2015
    resource: https://annals-csis.org/proceedings/2015/drp/241.html
    title: Luomala, Hakala - Effects of Temperature and Humidity on Radio Signal Strength in Outdoor Wireless Sensor Networks (FedCSIS 2015, 2.4 GHz)
  - id: london2026
    resource: https://users.cs.fiu.edu/~msha/publications/iotgreen2026.pdf
    title: A one-year LoRa field study with 15 sensors in a London park (IoTGreen 2026; read from a search summary only, the PDF did not download)
  - id: troppo
    resource: https://arxiv.org/pdf/2004.02802
    title: TROPPO LoRa - TROPospheric Personal Observatory using LoRa signals (refractivity and its gradient)
  - id: ttn-ducting
    resource: https://www.thethingsnetwork.org/forum/t/tropospheric-ducting-observations-at-968mhz/41035
    title: The Things Network forum - tropospheric ducting observations (LoRaWAN users hearing far gateways)
  - id: p453
    resource: https://itu.int/dms_pubrec/itu-r/rec/p/R-REC-P.453-9-200304-S!!PDF-E.pdf
    title: ITU-R P.453 - The radio refractive index, its formula and refractivity data
    author: team:itu-r
  - id: p838
    resource: https://www.itu.int/rec/R-REC-P.838/en
    title: ITU-R P.838 - Specific attenuation model for rain
    author: team:itu-r
  - id: dwd-10min
    resource: https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/10_minutes/air_temperature/DESCRIPTION_obsgermany_climate_10min_air_temperature_en.pdf
    title: DWD open data - 10-minute air temperature, humidity, pressure and dew point at German stations
    author: team:dwd
  - id: wetterdienst
    resource: https://wetterdienst.readthedocs.io/en/latest/data/provider/dwd/observation/10_minutes.html
    title: wetterdienst (Python) - DWD 10-minute observations
  - id: om-archive
    resource: https://open-meteo.com/en/docs/historical-weather-api
    title: Open-Meteo historical weather API (ERA5, ERA5-Land, ECMWF IFS; hourly; 5-day delay for ERA5)
  - id: om-dwd
    resource: https://open-meteo.com/en/docs/dwd-api
    title: Open-Meteo DWD ICON API (ICON-D2 2 km, 15-minutely; pressure levels 1000 to 850 hPa and up; past_days up to 92)
  - id: lindenberg
    resource: https://doi.pangaea.de/10.1594/PANGAEA.939305
    title: Radiosonde measurements from station Lindenberg (WMO 10393, 52.2° N 14.1° E), PANGAEA
  - id: hepburn
    resource: https://www.dxinfocentre.com/tropo_nwe.html
    title: Hepburn tropospheric ducting forecast, north-west Europe (hobby use only)
  - id: malla
    resource: https://malla.meshtastic.es/packet/13875690
    title: Malla, a Meshtastic MQTT packet viewer (ServiceEnvelope with gateway_id; rx_snr, rx_rssi, hop_start per gateway)
---

# The question

Does the weather change how well the hub hears the radios around it, and if so,
which weather and by how much? The owner wants to put the forecast and the
measured weather (temperature, humidity, rain, pressure) beside the signal
strengths at one or more hubs and see what moves together.

# What is already known

* **The radio's own temperature matters, and it is easy to mistake for weather.**
  In a temperature chamber, the signal a LoRa radio reports falls as the radio
  warms: about 3 to 4 dB from 0 to 50 °C on a HopeRF RFM95, 5 to 6 dB on a
  Semtech SX1272, in steps; enough to turn a link at the edge of range from
  perfect to almost useless.[^boano2018] The cause is in the receiver's
  amplifiers, so it is the *receiving* radio's temperature that counts (and the
  sender's, for its power amplifier). The roof hub's Meshstick sits in a loft,
  which is hot on sunny afternoons and cold on winter nights: any study must
  record its temperature.
* **Outdoor field studies disagree on size.** At 2.4 GHz over a summer and a
  winter, temperature explained the signal's changes best of the weather
  variables.[^luomala2015] A one-year LoRa study in a London park found distance
  and ground cover dominated, and temperature, humidity and season had weak or
  no effect (from a summary; the paper was not read).[^london2026]
* **Rain itself does almost nothing at 868 MHz.** Rain attenuation grows with
  frequency; from ITU-R P.838's coefficients near 1 GHz, even heavy rain
  (50 mm/h) costs about a thousandth of a dB per km.[^p838] What rain *can* do
  is wet the leaves on the path and the antenna and its cover; that is a guess
  to test, not a known number.
* **Warm, still air over cold can carry signals far (ducting).** Radio waves
  bend with the air's refractivity N, which depends on pressure, temperature
  and water vapour: N = 77.6/T · (P + 4810 e/T), with P and the vapour
  pressure e in hPa and T in kelvin.[^p453] What matters is how fast N falls
  with height: normally about −40 per km; below −157 per km the signal is
  bent back to the ground and can travel far beyond the horizon (a duct).[^p453][^troppo]
  Ducts form on clear calm nights and mornings with a temperature inversion,
  under high pressure in autumn and winter, and over water. LoRaWAN users have
  reported hearing far gateways during ducting.[^ttn-ducting] For a hub it
  would show as more far radios heard straight, and long links getting
  stronger, while short links do not change. A lower frequency needs a thicker
  duct, so not every duct seen at VHF/UHF reaches down to 868 MHz.
* **Humidity enters mainly through N.** Water vapour is the part of N that
  changes most from day to day, so a humid layer with dry air above it is a
  classic duct. Humidity at the ground alone says little; its change with
  height says more.

So the expected order of effects: the hub's own temperature (a few dB, every
day), refractivity layers (rare but large, on long links), wet leaves and
antennas (unknown, probably small), rain itself (nothing).

# What the hubs already record

* **meshtasticd's journal** has a line per packet with the time, the sender, the
  packet ID, the hop fields, the last relay's byte, SNR and RSSI;
  `tools/meshstats` reads it. On the roof hub ([pz7](/hardware/bench-pz7.md))
  the journal is capped at 64 MB, which held about a day and a half of radio
  lines on 2026-10-26. **A study needs a logger** that keeps the packet lines
  for weeks.
* **The zone's readings** (`links.json`) keep the last few SNRs per radio heard
  straight, but no times: no good for a time series.
* **The last hop is a straight link even when the packet was relayed.** The
  relay byte names the radio that handed the packet on; the SNR is of that last
  hop. The relays that bring most packets are fixed radios on roofs, the best
  beacons there are: always on, never moving, sending often.
* **The noise floor** is RSSI minus SNR on weak packets (SNR below 0);
  `meshstats` already prints it by hour. It tells a change in the signal apart
  from a change in the noise (interference rises by day).
* **The Pi's CPU temperature** is one command away (`vcgencmd measure_temp`;
  36.5 °C on pz7 at the time of writing) and is a rough stand-in for the
  Meshstick's temperature, since the stick plugs into the Pi.

# Weather data, free, for Berlin

| Source | What | Time step | Use |
|---|---|---|---|
| DWD open data, 10-minute stations[^dwd-10min] | Measured temperature at 2 m and 5 cm, humidity, dew point, pressure; rain and wind in sister folders | 10 min; *now* hourly, *recent* daily, *historical* quality-checked | The ground truth near the hub; pick the nearest Berlin station with `wetterdienst`[^wetterdienst] |
| Open-Meteo, ICON-D2[^om-dwd] | Forecast and recent past: temperature, humidity and height at 1000, 975, 950, 925, 850 hPa | 15 min to hourly, 2 km, `past_days` up to 92 | **N at several heights, so its gradient**: the ducting signal |
| Open-Meteo archive[^om-archive] | ERA5 and ECMWF IFS reanalysis at the ground: temperature, humidity, dew point, pressure, rain, cloud, wind, radiation | hourly; ERA5 5 days late | Long gap-free series; checks DWD |
| Lindenberg radiosonde[^lindenberg] | Balloon profiles of temperature and humidity, about 70 km south-east of Berlin | twice a day | The real profile on chosen nights, to check the model's gradients |
| Hepburn's tropo forecast[^hepburn] | A 0 to 10 index of bending, as maps | 3 to 6 h, six days ahead | Picking nights to watch; hobby use only, so not data to store |
| Others' radios | Meshtastic nodes with a sensor send temperature, humidity and pressure as telemetry | their interval | Free local stations on the hub's own channel; batnetd does not decode them yet |

# How to compare hubs

* **The same packet at two hubs.** When two hubs hear one packet (same sender
  and packet ID), the difference of their SNRs cancels anything at the sender
  (its power, its battery, its temperature) and leaves the two paths. With
  [pi7](/hardware/bench-pi7.md) back on air indoors or at another height, the
  difference between a roof path and a lower one against N's gradient is the
  cleanest ducting test there is. `tools/hubcompare` does the static version of
  this; the study needs it per packet over time.
* **Public MQTT gateways** upload packets with their own SNR and RSSI and the
  packet ID,[^malla] so the same packet is seen by many receivers across the
  city. That is other people's data: aggregate only, nothing named, and check
  first what the public server still carries.

# The study

**Measure** (one row per heard packet, kept private):
time, sender, packet ID, last relay byte, hops, SNR, RSSI, readable or garbled,
and every 10 minutes the Pi's temperature (better, a cheap temperature sensor
on the Meshstick).

**Join** each hour with: outdoor temperature, dew point, humidity, pressure,
rain and wind (DWD); N at the ground and its gradient from 1000 to 975 hPa
and 975 to 950 hPa (ICON-D2); the hour of day; the number of packets.

**Derive per hour:**

* for each fixed straight link (sender or last relay that never moves): its
  SNR minus its own median of the last 7 days, so slow changes (leaves falling,
  an antenna moved) do not pass for weather;
* the noise floor;
* how many different radios were heard straight, and the farthest of them;
* new far radios never heard before.

**Analyse:**

1. Plot first: the residuals of a few strong links, the noise floor and the
   Pi's temperature against outdoor temperature and N's gradient over the
   weeks.
2. Separate the hub's own temperature from the weather: the loft follows the
   outdoors, so put both in one regression (SNR residual against the radio's
   temperature, the weather and the hour of day, a separate offset per link).
   If short links move with the radio's temperature as much as long ones, it is
   the hardware.
3. Long against short links: ducting should lift long links (beyond about
   10 km) and the count of far radios, and leave short ones alone.
4. Events: compare inversion nights (N's gradient below about −100 per km)
   with ordinary nights, and rain fronts with the dry hours before them.
5. Correlations with weather lagged by a few hours, and only a few tests named
   before looking, so chance alone does not make a finding.

**How long:** four weeks at least, through changing weather; autumn and winter
high pressure brings the inversions. The leaves falling in October and
November are a slow change of their own; the 7-day median soaks most of it up.

**Pitfalls:**

* SNR comes in quarter-dB steps and saturates on strong signals; RSSI is the
  better measure above an SNR of about +5.
* A sender that changes its power or firmware, or moves, breaks its link;
  keep links with a steady median only, and mark known changes (antenna moves,
  presets, firmware) as breaks.
* Packets that came over MQTT are not radio: drop them (the journal flags
  them).
* Busy hours bring more collisions and more garbled packets: keep the packet
  count in the model.
* The data names real radios and places: it stays in `private/`, and only
  sizes of effects, counts and plots with no names come into this bundle.

# What would settle it

A test report in `tests/` with the effect sizes: dB per °C of the radio,
dB per unit of N's gradient on long links, the change in far radios heard on
inversion nights, and an upper bound for rain. Then the panel could say
"good night for range" from a forecast, and the hub's map could stop blaming
a radio for a cold loft.
