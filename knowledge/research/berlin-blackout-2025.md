---
type: Research Note
title: The Berlin blackout of September 2025 - the facts behind the film's opening
description: What is on the public record about the arson attack on two 110 kV pylons in Johannisthal on the night of 8/9 September 2025 and the blackout that followed, as used in the film's Berlin scene; and where the scene's map comes from.
tags: [berlin, blackout, film, grid-down, map]
status: stable
generated: { by: claude-code/claude-fable-5-1, at: 2026-10-15T16:00:00Z }
sources:
  - id: bz-stand
    resource: https://www.berliner-zeitung.de/article/stromausfall-in-berlin-nach-brandanschlag-das-ist-der-stand-am-mittwoch-2355733
    title: Berliner Zeitung - Stromausfall in Berlin nach Brandanschlag, der Stand am Mittwoch
  - id: berlin-de
    resource: https://www.berlin.de/aktuelles/9890881-958090-stromausfall-in-treptowkoepenick-noch-ru.html
    title: berlin.de - Stromausfall in Treptow-Koepenick
  - id: tagesspiegel
    resource: https://www.tagesspiegel.de/berlin/stromausfall-am-dritten-tag-sind-noch-mehr-als-10000-menschen-ohne-strom-14310101.html
    title: Tagesspiegel - Stromausfall, am dritten Tag sind noch mehr als 10.000 Menschen ohne Strom
  - id: stz
    resource: https://www.stuttgarter-zeitung.de/panorama/stromnetz-berlin-laengster-blackout-seit-25-jahren-79155899.html
    title: Stuttgarter Zeitung - Stromnetz Berlin, laengster Blackout seit 25 Jahren
  - id: carto
    resource: https://carto.com/basemaps/
    title: CARTO basemaps (dark_all): an API key is required since 2025
  - id: osm-copyright
    resource: https://www.openstreetmap.org/copyright
    title: OpenStreetMap copyright and licence (ODbL)
---

# What happened

* In the night of 8 to 9 September 2025 (a Tuesday) fire was set at the foot
  of two 110 kV pylons in the Koenigsheide, Johannisthal, in the district of
  Treptow-Koepenick; accelerants were used and a left-extremist claim appeared
  online. The police treat it as a politically motivated arson attack.[^bz-stand][^stz]
* The grid operator spoke of about 50,000 customers without power; the fire
  brigade counted 43,000 households, 3,000 businesses and two care homes.
  Traffic lights, trams and the S-Bahn in the south-east stopped.[^bz-stand][^berlin-de]
* Localities named as affected: Johannisthal, Adlershof, Niederschoeneweide,
  Altglienicke, and parts of Gruenau and Rudow.[^bz-stand][^berlin-de]
* On the third day more than 10,000 people were still without power; the
  operator called it the longest outage in Berlin for at least 25 years and
  aimed to have everyone back by Thursday evening, about 60 hours after the
  fire.[^tagesspiegel][^stz]

# What the film says

"Berlin, September last year. One fire at the foot of two pylons in
Johannisthal, and fifty thousand homes and businesses go dark for two and a
half days." The on-screen figures are 50,000 customers, 2.5 days, and
"0 bars" for the mobile masts, whose battery backup is a matter of hours (the
film does not give a figure for Berlin's masts; the public reports describe
mobile coverage failing during the outage). The first cut wrongly said
"January" and "a cable tunnel"; corrected 2026-10-15 from the sources above.

# The map in the scene

`video/fetch_map.py` fetches, once, 84 tiles of OpenStreetMap's standard
style (zoom 12, 256 px) covering Berlin from tile.openstreetmap.org, with a
named User-Agent as the tile usage policy asks, and the outlines of Berlin, of
Treptow-Koepenick and of the six localities above from OpenStreetMap's
Nominatim; the scene inverts the tiles to a dark style on the canvas. The
files are gitignored: they are OpenStreetMap data under the
ODbL,[^osm-copyright] fetched fresh rather than kept in the repo, and the
repo's secret-hygiene check rightly refuses a file full of precise
coordinates. The frame carries "Map (c) OpenStreetMap contributors". (CARTO's
dark basemap was tried first; since 2025 its tiles need an API key and come
back watermarked without one.[^carto]) The outlines are public administrative
boundaries, the fire's position is rounded to about a kilometre, and nothing
in the scene marks a person or an address.
