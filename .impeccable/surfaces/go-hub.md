---
version: 1
slug: "go-hub"
primary_target: "go-hub"
related_targets: []
---

# Go Hub - Store surface

Mode: Operate. Windows desktop suite hub, built with MyGo. Audience: people who use the go-things apps and want one place to install, open, and remove them. The client pinned the visual family to Go Post/Go Mail and asked for storefront framing like Steam or Epic Games, so the direction is brief-pinned rather than rolled.

## Direction contract

THESIS: A storefront for a two-app suite, not a download page. Apps flagged `featured` in the catalog own the top of the stage with a real screen, and the suite index sits directly below. It refuses the category default of a whole page made of same-size icon cards and a wall of marketing copy.

OWN-WORLD: Nocturne ground `#0b0b0b`, sidebar on the ground, stage panel `#161616` at radius 18, cards `#1d1d1d` at radius 12 (rows use `#1d1d1d` fields), a theme-extreme accent (white on Nocturne, black on Daylight), Segoe UI, 1px white-alpha borders, offset soft shadows, frameless window with brand in the sidebar top and minimize/maximize/close in the stage header.

STORY: The visitor understands what the suite is within one viewport, sees which app is featured and whether it is installed, and can install, launch, or remove it without leaving the screen. A second glance answers "what else is in the suite".

FIRST VIEWPORT: 246px sidebar on the ground (brand tile + "Go Hub / Go Things suite" at the top, draggable; Store, Library, About nav with an installed count badge; a "SUITE" section listing each app with its install dot and version; a three-way Auto/Night/Day theme toggle at the bottom) beside a main column whose 52px bar on the ground carries only the frameless window controls on the right, above a rounded stage panel (radius 18, `#161616`) holding the scrollable page content. No page titles or subtitles: the nav and the content itself name the page. The hero is 258px tall: a left text column with the app name at 27/720, a one-line tagline, three fact chips, the install state line, and install/open/update + uninstall actions; a right column split off by a 1px rule, with a full-bleed real app capture (no frame, no shadow). Below, an "ALL APPS" label and two capsule rows, each with a real preview image, the app name, its state pill, tagline, honest small print, and a single state-driven action. The whole hero and the whole row are click targets that open that app's details page; the action buttons inside stay their own targets, so installing or opening never navigates.

DETAILS: one app per page, reached from a store card and left with a Back to store button. App tile, name at 22/720, state pill, tagline, fact chips, honest small print, and the same state-driven actions; a horizontally scrolling "SCREENS" gallery of real captures at 300x188, a "FEATURES" list of what the app actually does, and a "DETAILS" block of version, identifier, installed version/location, and setup size. Store stays highlighted in the nav while a details page is open.

FORM: Storefront feature + index inside the established go-things family world. The world was pinned by the brief, so surface concept-seed was not rolled; composition is derived from the task (feature the suite, one action per state). seed: none (unavailable).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## States

- Available: install primary, description of the app, version shipped.
- Downloading: primary replaced by "Downloading <app>…"; the installer is cached before the state moves on.
- Installing: primary replaced by an indeterminate working state, actions locked.
- Installed: launch primary, uninstall quiet action, installed version shown.
- Update available: install primary labelled "Update", installed and shipped versions both shown.
- Failed: inline message naming the problem and the recovery.

## Copy rules

Controls name their result: "Install Go Post", "Open Go Post", "Uninstall Go Post", "Update Go Post". Status is written, not only coloured. No invented claims, ratings, download counts, or prices.

## Constraints

Windows only; per-user NSIS installs read from `HKCU\...\Uninstall\<identifier>`; apps, versions, setup URLs and screenshot URLs come from `catalog.json` in the project's latest GitHub release. Installers are downloaded to a local cache and run silently; previews are real captures from the catalog. A failed catalog check falls back to the built-in list and offers Retry.
