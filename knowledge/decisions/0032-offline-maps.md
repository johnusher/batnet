---
type: Decision
title: "0032: Offline maps on the hub (Protomaps extracts, fetched by each hub)"
description: Each hub keeps vector map extracts of OpenStreetMap, cut from Protomaps' daily build with the pmtiles tool while it has internet - the area round the hub by itself, cities at a button - and the panel draws them with protomaps-leaflet, so the map shows streets in a power cut; no map data goes into the repository.
tags: [hub, map, offline, openstreetmap, protomaps]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-22T19:05:00Z }
---

# Context

The hub's map drew OpenStreetMap's own raster tiles from the internet: in a
power cut, with the router off, the radios sat on a blank grid. The owner
asked to "harden everything for offline", with maps for at least Berlin,
London, Paris and Barcelona, and not in the git repository. Nothing else in
the panel needed the internet (Leaflet, the help and the translator were
already on the hub).

OpenStreetMap's tile servers forbid bulk downloading ("tile scraping"), so
the hub cannot save their tiles. Protomaps publishes a daily planet build of
OpenStreetMap as one PMTiles file, meant to have areas cut out of it.

# Decision

* **Vector extracts from Protomaps' daily build**
  (`build.protomaps.com/YYYYMMDD.pmtiles`, the newest from its builds list),
  cut to zoom 15 (single streets) by the `pmtiles` tool (go-pmtiles, pinned
  and checksum-checked by `setup.sh`), which reads only the needed parts over
  HTTP range requests. Kept in `/var/lib/batnet/maps/<area>.pmtiles`.
* **Each hub fetches its own**, while it has internet: the area round the hub
  (about 55 km across) by itself once its place is marked, tried at most
  hourly; Berlin, London, Paris and Barcelona at a button under System,
  Offline maps; Update and Remove there too. One fetch at a time, never below
  2 GB free on the card. **No map data in the repository.**
* **batnetd serves them** at `/hub/maps/<area>.pmtiles` with range requests,
  no PIN (public data). The panel draws them with **protomaps-leaflet**
  (vendored, BSD licence) over the online tiles, each within its bounds, in
  the light style; dark mode inverts the map as it does the online tiles.
* **Attribution** as the data's licence asks: Protomaps and
  © OpenStreetMap contributors (ODbL), shown on the map.

# Consequences

* The map shows streets with no internet, for the areas the hub keeps:
  measured on the bench hub, round the hub 92 MB, Berlin 86 MB, London
  131 MB, Paris 105 MB, Barcelona 33 MB, all five in about 5 minutes.
* Maps do not update by themselves; Update fetches the day's build.
* Each hub depends on Protomaps' build server once per area; if it goes,
  `pmtiles extract` works from any planet PMTiles file, including one made
  with Planetiler from OpenStreetMap's own planet file.
* Vector maps are drawn in the browser: a little more work for the hub's own
  screen than pictures, none for the Pi.

# Alternatives

* **Saving OpenStreetMap's raster tiles:** against their tile usage policy.
* **Map files in the repository or a release:** hundreds of MB that age, for
  places most hubs never need.
* **MapLibre GL:** about six times larger, and its labels need font files
  served too; protomaps-leaflet keeps the existing Leaflet map.
