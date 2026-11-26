# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

People on Windows 10/11 who use or want the go-things apps. They want one place to discover what the suite contains, install an app, open it, and remove it again without hunting through folders or Settings.

## Product Purpose

Go Hub is the desktop home for the Go Things suite. It presents each app with a real preview from the public catalog, downloads the Windows setup you choose, shows whether the app is installed and at which version, launches it, and uninstalls it. Success is one obvious action per state and no ambiguity about what is installed.

## Positioning

One install, an always-current suite: Go Hub reads `catalog.json` from the project's latest GitHub release, so new apps and updates appear without installing a new hub. No account or store service is involved; only the setup you choose is downloaded.

## Operating Context

Windows desktop app built with MyGo and a native UI, frameless with a custom titlebar. Apps install per user through NSIS into `%LOCALAPPDATA%\Programs`, register under `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall`, and expose an `Uninstall.exe`. Go Hub sits on the same machine and manages those installs directly.

## Capabilities and Constraints

- Windows only, per-user installs, no elevation.
- The suite currently has two apps: Go Post (HTTP workspace) and Go Mail (IMAP/SMTP mail client).
- The app list, versions, setup URLs and screenshot URLs come from `catalog.json` in the latest GitHub release; see `RELEASING.md`.
- Each app has a name, one-line description, version, preview image and https setup URL.
- Store, Library, and About are the only sections.
- Installed state and version are read from the Windows uninstall registry; no background service.
- Install downloads the setup to a local cache, runs it silently (`/S`), then waits for Windows to list the app. Uninstall runs silently too.
- If the catalog cannot be reached, the built-in list stays usable and the hub offers Retry.
- The hub itself is a MyGo app and ships with its own Windows installer.

## Brand Commitments

The go-things family look: Nocturne dark palette (`#0a0b0f` ground, `#15161c` panels) with a black-or-white hub accent (white on Nocturne, black on Daylight), Segoe UI, rounded panels, restrained 1px borders, bespoke frameless titlebar. Go Hub may borrow the storefront framing people know from Steam and Epic (featured app, preview art, library) but must stay inside the family's visual language.

## Evidence on Hand

- `go-post/resources/preview1.png`, `preview2.png`, `preview3.png`: real captures of Go Post.
- `go-mail/resources/preview1.png`: real capture of Go Mail.
- Each app's `resources/icon.png` is its own mark; Go Hub keeps only its own icon.
- `catalog.json`: the published catalog schema and the real URLs used by the hub.
- `RELEASING.md`: exact release assets and the publish flow.
- Setup executables and images are attached to the GitHub release, not committed.
- No customers, testimonials, benchmarks, or pricing claims exist; none may be invented.

## Product Principles

1. State first: installed, available, working, or failed is legible before any decoration.
2. One click to install, one click to open, one click to remove; every action names its result.
3. Show the real app: previews are captures of the software, never artist's impressions.
4. Keep discovery honest: two apps, described by what they do, with no invented proof.
5. Stay inside the suite's visual language so Go Hub feels shipped by the same hand.

## Accessibility & Inclusion

Keyboard reachable controls with visible focus, text contrast that holds in both palettes, and status conveyed by words as well as colour.
