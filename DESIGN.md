---
name: Go Hub
description: The home of the Go Things suite on Windows — reads a public catalog, installs, opens, and removes the apps.
colors:
  primary: "#ffffff"
  primary-hover: "#e6e6e6"
  primary-pressed: "#cccccc"
  primary-light: "#000000"
  primary-hover-light: "#1a1a1a"
  primary-pressed-light: "#333333"
  success: "#59d39b"
  warning: "#f2b35d"
  danger: "#ff6b70"
  danger-hover: "#ff8286"
  neutral-bg: "#0b0b0b"
  neutral-panel: "#161616"
  neutral-field: "#1d1d1d"
  neutral-hover: "#242424"
  neutral-border: "rgba(255, 255, 255, 0.075)"
  neutral-text: "#f4f4f6"
  neutral-muted: "#8a8f9f"
  neutral-text-light: "#1a1c22"
  neutral-bg-light: "#e9ebef"
  neutral-panel-light: "#ffffff"
  neutral-field-light: "#f2f3f6"
  neutral-border-light: "rgba(20, 24, 34, 0.12)"
typography:
  display:
    fontFamily: "Segoe UI"
    fontSize: "27px"
    fontWeight: 720
    lineHeight: 1.1
    letterSpacing: "-0.6px"
  headline:
    fontFamily: "Segoe UI"
    fontSize: "17px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.2px"
  title:
    fontFamily: "Segoe UI"
    fontSize: "14.5px"
    fontWeight: 680
    lineHeight: 1.25
    letterSpacing: "-0.15px"
  body:
    fontFamily: "Segoe UI"
    fontSize: "12.5px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Segoe UI"
    fontSize: "10.5px"
    fontWeight: 550
    lineHeight: 1.3
    letterSpacing: "0.2px"
  micro:
    fontFamily: "Segoe UI"
    fontSize: "9.5px"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "1.2px"
rounded:
  sm: "7px"
  md: "8px"
  lg: "10px"
  xl: "12px"
  2xl: "14px"
  3xl: "16px"
  full: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  2xl: "24px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "0 14px"
    height: "34px"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "0 14px"
    height: "34px"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.neutral-text}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "34px"
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "0 13px"
    height: "34px"
  chip:
    backgroundColor: "{colors.neutral-panel}"
    textColor: "{colors.neutral-muted}"
    rounded: "{rounded.md}"
    padding: "0 9px"
    height: "24px"
  card:
    backgroundColor: "{colors.neutral-field}"
    rounded: "{rounded.3xl}"
    padding: "12px 14px"
  stage:
    backgroundColor: "{colors.neutral-panel}"
    rounded: "{rounded.3xl}"
    padding: "22px 24px 28px 24px"
  tile:
    backgroundColor: "{colors.primary}"
    rounded: "26%"
    size: "34px"
---

# Design System: Go Hub

## Overview

**Creative North Star: "The Suite Storefront"**

A storefront for a two-app suite, not a download page. The featured app owns the top of the stage with a real screen, and the suite index sits directly below. It refuses the category default of a whole page made of same-size icon cards and a wall of marketing copy. Nocturne ground `#0b0b0b`, a sidebar on the ground, a stage panel at radius 18, cards at radius 12–14, a theme-extreme accent (white on Nocturne, black on Daylight), Segoe UI, one-pixel white-alpha borders, offset soft shadows, a frameless window with the brand in the sidebar top and the window controls in the stage header.

**Key Characteristics:**

- Storefront framing: apps marked `featured` in the catalog open the page with a real capture, then the suite index follows.
- Every app opens a details page: a whole card (hero or row) is one click target; the action buttons inside keep their own clicks.
- Honest copy: no ratings, prices, download counts, or invented claims.
- State-driven actions: install, open, update, uninstall — one primary per state.
- Nocturne by default, Auto and Daylight selectable, persisted locally.
- Every preview is a real capture of the built program, loaded from the catalog's URLs.
- The store checks the latest release on launch; a failed check shows the built-in list and a Retry, never invented data.
- Install downloads the named setup to a cache, then runs it silently.

## Colors

The palette is a two-theme system: Nocturne (default, dark) and Daylight, plus Auto which follows the desktop. The hub's accent is the theme's own extreme — white on Nocturne, black on Daylight — and is used sparingly; its rarity is the point.

### Primary

- **White / Black** (`#ffffff` on Nocturne, `#000000` on Daylight): the one accent, used for primary actions, the brand tile, selection and focus. The ink on it is chosen by contrast (`#16171b` on white, `#ffffff` on black).

### Neutral

- **Nocturne ground** (`#0b0b0b`): window background; the sidebar sits directly on it.
- **Stage panel** (`#161616`): the rounded working surface beside the sidebar.
- **Field** (`#1d1d1d`): card and row backgrounds; the quietest level above the panel.
- **Hover** (`#242424`): row and button hover ground.
- **Border** (`rgba(255,255,255,0.075)`): one-pixel hairline used everywhere on dark grounds.
- **Text** (`#f4f4f6`): primary ink on dark grounds.
- **Muted** (`#8a8f9f`): secondary ink, subtitles, meta lines.
- **Daylight ground** (`#e9ebef`): the light window background.
- **Daylight panel** (`#ffffff`): the light stage surface.
- **Daylight field** (`#f2f3f6`): light card and row background.
- **Daylight border** (`rgba(20,24,34,0.12)`): light hairline.
- **Daylight text** (`#1a1c22`): primary ink on light grounds.
- **Daylight muted** (`#5d6472`): secondary ink on light grounds.

### Semantic

- **Good** (`#59d39b` / `#159664`): installed state, success dot.
- **Warn** (`#f2b35d` / `#c47b16`): working state, update available.
- **Bad** (`#ff6b70` / `#c73f45`): errors, removal, danger.

**The One Voice Rule.** The primary accent appears on primary actions, the brand tile, focus rings, and selection only — never as a background wash or decorative stripe. Its rarity is the point.

## Typography

**Display Font:** Segoe UI (with system fallback)
**Body Font:** Segoe UI (with system fallback)

**Character:** A single sans family at varied weights, leaning on weight and size rather than a second typeface. Tight letter-spacing on large sizes, generous on micro labels.

### Hierarchy

- **Display** (720, 27px, 1.1, −0.6px): app name in the hero.
- **Headline** (720, 22px, 1.2, −0.4px): app name on the details page.
- **Title** (680, 14.5px, 1.25, −0.15px): row and card titles.
- **Body** (400, 12.5px, 1.5): taglines, descriptions, body copy; max ~65ch.
- **Label** (550, 10.5px, 1.3, 0.2px): meta lines, chips, fact rows.
- **Micro** (700, 9.5px, 1.3, 1.2px): section labels, uppercase.

## Layout

Sidebar 246px wide on the ground, stage panel fills the rest. Window 1180×760, minimum 1020×680. Spacing rhythm is 4px-based (`xs 4, sm 8, md 12, lg 16, xl 20, 2xl 24`). Cards use 12–14px padding, the stage 22 top / 24 sides / 28 bottom, the sidebar 8×8×12×10. Hero is 258px tall. Store rows are 126px tall; library rows 76px. Details screens are 300×188 in a horizontal scroller. Max content width in About is 620px.

The stage opens with a 52px bar on the ground that carries nothing but the window controls: pages are named by the sidebar and by their own content, never by a heading above the panel. The controls are the family's: three 36×30 targets with 7px radii and 2px gaps, muted icons that lift to the text colour on hover, and close turning red, 8px in from the window edge and centred in the bar exactly as in Go Post and Go Mail.

## Elevation & Depth

Offset soft shadows on the preview frames (`0 10 24 0, rgba(0,0,0,0.28)`); the rest of the UI is flat and relies on tonal layering and one-pixel hairlines. Shadows describe containment, not elevation — they appear on framed previews only.

**The Flat-By-Default Rule.** Surfaces are flat at rest. Shadows appear only on framed previews, never on buttons, cards, or rows.

## Shapes

Corner language is small and consistent: 7px for icon buttons, 8px for buttons and chips, 10px for the app tile, 12–14px for cards and rows, 16px for the stage panel, 13px for empty-state icon tiles. The app tile uses a 26% radius. Hairlines are one pixel.

## Components

### Buttons

- **Shape:** 8px radius, 34px tall, 0 12–14px horizontal padding.
- **Primary (accent):** white fill on Nocturne, black fill on Daylight, contrast ink, 650 weight, 13px icon + 12.5px label. Hover moves 10% away from the face's extreme; pressed 20%.
- **Quiet:** transparent fill, one-pixel border, text ink. Hover fills with the hover ground; danger variant tints the border and hover ground with the bad color.
- **Danger (filled):** bad-color fill, contrast ink, used only after an inline confirmation.
- **Icon:** 30×30 square, 7px radius, quiet treatment, tooltip.

### Chips

- **Style:** panel ground, one-pixel border, muted ink, 7px radius, 24px tall.

### Cards / Rows

- **Corner Style:** 12–14px radius.
- **Background:** field ground on dark, white on light.
- **Border:** one-pixel hairline.
- **Internal Padding:** 12–14px.

### App tile

- **Shape:** 26% radius square, brand tile filled with the theme's accent (white on Nocturne, black on Daylight) and the line glyph; when the catalog supplies an icon, it fills the tile edge to edge with no border.

### Navigation

- **Style:** 34px tall rows, 9px radius, muted ink default, text ink when active or hovered, active ground is the field color.

### State pill

- **Style:** 6px radius, 20px tall, tinted background (color at 14% alpha), ink darkened on light grounds; a 5px dot plus a 9.5px/650 label.

## Do's and Don'ts

### Do:

- **Do** use real captures for every preview; nothing is a mockup.
- **Do** name every action by its result ("Install Go Post", "Open Go Post").
- **Do** write status in words, not only color.
- **Do** keep the accent rare; it carries the primary action.
- **Do** persist the theme choice locally.

### Don't:

- **Don't** invent ratings, prices, download counts, or claims.
- **Don't** add a separate titlebar; the brand lives in the sidebar top and the controls in the stage header.
- **Don't** ship a modal for uninstall; use the two-step inline confirm.
- **Don't** use a second typeface; Segoe UI carries everything.
- **Don't** put shadows on buttons, cards, or rows.