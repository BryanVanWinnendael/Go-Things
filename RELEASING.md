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

go-mail/resources/
  icon.png
  preview1.png
```

Use the same convention for every app: `icon.png` plus numbered `previewN.png` files. `preview1.png` is the featured/hero shot; the rest appear in the SCREENS gallery.

`catalog.json` points straight at those repository files with raw URLs such as:

`https://raw.githubusercontent.com/BryanVanWinnendael/Go-Things/main/go-post/resources/preview1.png`

Pictures are **not** release assets. Commit them to the repository and keep the paths in `catalog.json` in sync with the `resources` folders. The URLs use the `main` branch; if your default branch is named differently, update that part of each URL.

## What to attach to a release

Every release that should be "latest" must contain the full set of installers, because Go Hub resolves setup names against that one release:

- `catalog.json` — copy of the file at the root of this repository.
- One Windows setup per app, named exactly as the `setup` URL in the catalog, for example `go-post-setup-1.0.0-amd64.exe`.
- `go-hub-setup-1.0.0-amd64.exe` if you want people to be able to install or reinstall the hub from the same release.

Build outputs stay out of Git. They are uploaded here, not committed.

## Publish an update or a new app

1. Build the changed app and its setup in that app's folder:
   `go run github.com/egoist/mygo/cmd/mygo build -platform windows/amd64`
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

## Catalog rules

- `schemaVersion` must be `1`.
- Set `"featured": true` on each app that should appear in the large featured section at the top of the store. Apps without the flag still appear in ALL APPS. Marking more than one app shows more than one hero card.
- `id` is lowercase letters, digits and hyphens; it must be unique.
- `setup` must be an `https` URL to a Windows installer in the release.
- `preview`, `hero`, `icon` and each `screenshots` entry may be omitted, but if present they must be `https` raw URLs to that app's `resources/` folder in this repository.
- `accent` is optional and must look like `#10b981`.
- `identifier` is the registry key the setup writes; `exe` is the installed program's filename. Keep both in sync with the app's `mygo.json`.

Installers are cached under `%LOCALAPPDATA%\go-hub\downloads`, keyed by app id and version, so a reinstall does not download the same file twice.
