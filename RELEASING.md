# Releasing Go Things apps

Go Hub reads one file from the newest GitHub release:

`https://github.com/BryanVanWinnendael/Go-Things/releases/latest/download/catalog.json`

Install it once. After that, publishing a new release is all it takes to add an app or ship an update. Go Hub itself ships only its own icon; it does **not** bundle other apps' installers or pictures.

## Where pictures live

Each app owns its pictures in its own folder in this repo:

```
go-post/resources/
  icon.png
  preview1.png
  preview2.png
  preview3.png
  preview-light1.png
  preview-light2.png
  preview-light3.png

go-mail/resources/
  icon.png
  preview1.png
  preview2.png
  preview3.png
  preview-light1.png
  preview-light2.png
  preview-light3.png
```

Use the same convention for every app: `icon.png` plus numbered `previewN.png` files. `preview1.png` is the featured/hero shot; the rest appear in the SCREENS gallery. A `preview-lightN.png` file is the same screen in Daylight mode; Go Hub shows it when the user picks Day (or Auto on a light desktop). Light files are optional — a catalog without them keeps using the dark pictures in both themes.

`catalog.json` points straight at those repository files with raw URLs such as:

`https://raw.githubusercontent.com/BryanVanWinnendael/Go-Things/main/go-post/resources/preview1.png`

Each app also has `previewLight`, `heroLight` and `screenshotsLight` pointing at its `preview-lightN.png` files. Go Hub downloads both sets at once and only repaints when the theme changes.

Pictures are **not** release assets. Commit them to the repository and keep the paths in `catalog.json` in sync with the `resources` folders. The URLs use the `main` branch; if your default branch is named differently, update that part of each URL.

## What to attach to a release

Every release that should be "latest" must contain the full set of installers, because Go Hub resolves setup names against that one release:

- `catalog.json` — copy of the file at the root of this repository.
- One Windows setup per app, named exactly as the `setup` URL in the catalog, for example `go-post-setup-1.0.0-amd64.exe`. `build.ps1` leaves it in `go-post/build/windows-amd64/`.
- `go-hub-setup-1.0.0-amd64.exe` if you want people to be able to install or reinstall the hub from the same release.

Build outputs stay out of Git. They are uploaded here, not committed.

## Publish an update or a new app

1. Build all apps by running `.\build.ps1` from the repository root. It clears each old `build` folder, builds every app, and leaves the setups in `<app>/build/windows-amd64/`. Pictures stay in `resources/` and are never bundled into a setup.
2. Put the app's pictures in `<app>/resources/` using the naming convention above.
3. Update `catalog.json`:
   - bump the app's `version`;
   - change `setup` to the new installer's asset name;
   - add or renumber `previewN.png` URLs if the pictures changed;
   - add a new object to `apps` for a new app and set `featured: true` if it should get a hero card;
   - update `updated`.
4. Create a GitHub release and upload **all** assets listed above. The pictures stay in the repository.
5. Publish it as a normal release, not a draft or a pre-release. GitHub's `latest` release is what Go Hub reads.

Go Hub checks the catalog on launch. `Ctrl/Cmd+R` in the hub forces another check. If GitHub cannot be reached, the hub keeps working with the built-in list and shows a Retry button; nothing is invented or downloaded in the background.

## Updating Go Hub itself

`catalog.json` has a small `hub` object with just its `version` and `setup` URL. Go Hub is **not** in the `apps` list, so it does not appear in the store or Library like Go Post and Go Mail. When `hub.version` is newer than the running hub, the store shows an Update banner; the setup closes the running hub before replacing its files, so the update finishes after the hub reopens.

To ship a hub update, bump **all three** version numbers together:

1. `go-hub/mygo.json` — `version`.
2. `go-hub/internal/catalog/catalog.go` — `Version`, used by a hub running from source.
3. `catalog.json` — the `hub.version` and `hub.setup` URL.

Then build, and upload the new `go-hub-setup-<version>-amd64.exe` and the catalog to a release, as above. A mismatch between these values makes the update either invisible or offered forever.

## Catalog rules

- `schemaVersion` must be `1`.
- Set `"featured": true` on each app that should appear in the large featured section at the top of the store. Apps without the flag still appear in ALL APPS. Marking more than one app shows more than one hero card.
- `id` is lowercase letters, digits and hyphens; it must be unique.
- `setup` must be an `https` URL to a Windows installer in the release.
- `preview`, `hero`, `icon` and each `screenshots` entry may be omitted, but if present they must be `https` raw URLs to that app's `resources/` folder in this repository.
- `accent` is optional and must look like `#10b981`.
- `hub` is optional and holds only `version` and an `https` `setup` URL; it is not an app in the `apps` list.
- `identifier` is the registry key the setup writes; `exe` is the installed program's filename. Keep both in sync with the app's `mygo.json`.

Installers are cached under `%LOCALAPPDATA%\go-hub\downloads`, keyed by app id and version, so a reinstall does not download the same file twice.
