# MyGo Store

English · [中文](README.zh-CN.md)

A native cross-platform app marketplace for projects built with [MyGo](https://github.com/egoist/mygo), inspired by [Applite](https://github.com/milanvarady/Applite). It browses the [Awesome MyGo showcase](https://awesomemygo.dev/showcase): 504 apps, grouped by category, with scores, screenshots and links to each project. It runs on macOS, Windows and Linux.

![MyGo Store](docs/screenshot.png)

![An app detail page](docs/detail.png)

## Catalog updates

The catalog and images are bundled with the app, so it opens instantly and works offline. `scripts/sync.py` refreshes them from the upstream catalog.

At runtime the app also keeps its catalog fresh:

- About 10 seconds after launch, then every 6 hours, it downloads `apps.json` from the fixed [`catalog-latest`](https://github.com/tk103331/mygo-store/releases/tag/catalog-latest) release.
- Conditional requests through the ETag skip the download when nothing has changed.
- Only a catalog newer than the current one is used; invalid or older files are ignored.
- A successful check is cached as `catalog.json` in the config directory and preferred at the next launch.
- The sidebar footer shows the catalog status; click it to check immediately.

The workflow in `.github/workflows/catalog.yml` runs every day at 03:17 UTC, and can also be started from the Actions page:

1. Run `scripts/sync.py` to refresh the catalog, icons and screenshots.
2. Commit `data/` and `assets/` only when they changed, so an unchanged upstream does not create a daily commit.
3. Make sure the `catalog-latest` release exists, and upload `apps.json` when it changed or is missing. The tag is never marked as the latest release, so `releases/latest` keeps pointing at the app.

`scripts/sync.py` keeps the previous `syncedAt` when the source, date and apps are unchanged, so an idle day neither commits nor republishes the catalog.

## Run

Requires Go 1.27 or later. Everything else, including the UI toolkit, is a Go dependency.

```sh
go tool mygo dev     # development app, rebuilds on change
go run .             # a quick run without the app bundle
```

## Build

```sh
go tool mygo build                                  # the current platform
go tool mygo build -platform darwin/universal       # macOS, Apple silicon and Intel
go tool mygo build -platform windows/amd64,windows/arm64
go tool mygo build -platform linux/amd64,linux/arm64
```

Packaged apps go to `build/<os>-<arch>/`. The app's name, identifier, version, icon and update settings are in `mygo.json` and `resources/icon.png`.

A build with signed update files needs the update key:

```sh
MYGO_UPDATER_PRIVATE_KEY="$(cat "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key")" \
  go tool mygo build -platform darwin/universal
```

## Updates

Since 0.2.0 the app updates itself through MyGo's signed update feed:

- It checks about 10 seconds after launch, then once a day while it runs.
- Choose **Check for Updates…** in the app menu, or **检查更新** at the bottom of the sidebar, to check manually.
- Updates are signed with Ed25519; the app accepts only an update that matches the public key in `mygo.json`.
- A downloaded update can be installed and the app restarted right away, or it takes effect at the next launch.
- Development builds and apps installed by a package manager do not update themselves.

Publishing an update needs the `MYGO_UPDATER_PRIVATE_KEY` repository secret. The private key is written to `~/Library/Application Support/mygo/update-keys/mygo-update.key`; keep it safe, because losing it strands installed apps.

## Release

Push a `v*` tag and GitHub Actions does the rest:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The workflow in `.github/workflows/release.yml` validates the tag, runs the tests, creates a draft release, and builds every platform in parallel:

- macOS: `darwin/universal`, with a `.dmg`.
- Windows: `windows/amd64` and `windows/arm64`, with installers.
- Linux: `linux/amd64` and `linux/arm64`, with `.deb`, `.tar.gz` and `install.sh`.

Each platform uploads its installers, signed update archives, deltas and `update-<platform>.json`. The workflow then publishes the release and merges `SHA256SUMS`. Tags with a suffix such as `v0.2.0-beta.1` become prereleases. You can also run it from the Actions page with an existing tag.

- The version is written into `mygo.json` only for that build; nothing is committed back.
- The release contains the macOS disk image, Windows installers, Linux packages, `install.sh`, `install.ps1`, `SHA256SUMS`, and the update manifests.
- macOS builds use MyGo's default ad-hoc signature. To distribute without Gatekeeper warnings, configure `macos.signingIdentity` and `macos.notarize` in `mygo.json`, add the Developer ID certificate and notary credentials as Actions secrets, and import them in the workflow.
- Add the update key before the first release:

```sh
gh secret set MYGO_UPDATER_PRIVATE_KEY < "$HOME/Library/Application Support/mygo/update-keys/mygo-update.key"
```

## Data

`data/apps.json` is generated from [`yldm-tech/awesome-mygo`](https://github.com/yldm-tech/awesome-mygo), which also builds the showcase site. The score is a port of the site's `packages/catalog/src/score.ts`, so the numbers match awesomemygo.dev.

```sh
python3 scripts/sync.py                  # refresh catalog, icons and screenshots
python3 scripts/sync.py --no-assets      # data only, keep existing images
python3 scripts/sync.py --force-assets   # download images again
python3 scripts/sync.py --catalog DIR    # use an unpacked catalog/data directory
```

Only Python 3.11's standard library is needed. Icons and screenshots go to `assets/icons` and `assets/shots` and are embedded with `go:embed`. Apps whose images are missing fall back to a generated letter tile and a gradient placeholder.

## How it works

- **Discover** — top-scored apps and a shelf per category.
- **Browse** — a virtualized grid per category, the favorites, the acquired apps, or a search. Filter by native/web and platform, and sort by score, date or likes.
- **Detail** — icon, tags, score, screenshot, description, facts, source link and the author's other apps.
- **获取** — detects the current OS and architecture, queries the app's GitHub Releases, and downloads the best matching installer (`.dmg`/`.pkg` on macOS, `.exe`/`.msi` on Windows, `.AppImage`/`.deb`/`.rpm` on Linux). The file is saved to `~/Downloads/MyGo Store`, then revealed in Finder; the button becomes **打开安装包**. If a release has no matching asset, or the project has no repository, the official page opens instead. Installers are never run automatically.
- **Updates** — checks for a new version shortly after launch and once a day, then offers to download, install and restart. The app menu and the sidebar's **检查更新** check manually. See [Updates](#updates).
- **Catalog** — starts from the bundled snapshot and refreshes the app list from the `catalog-latest` release in the background.

Favorites, acquired apps and downloaded installer paths are saved to `state.json` under the user's config directory (`~/Library/Application Support/MyGo Store` on macOS). Set `MYGO_STORE_CONFIG` to use another directory, and `MYGO_STORE_DOWNLOADS` to change the download folder.

| Variable | |
|---|---|
| `MYGO_STORE_CONFIG` | Override the config directory |
| `MYGO_STORE_DOWNLOADS` | Override the installer download directory |
| `MYGO_STORE_CATALOG_URL` | Override the catalog URL |
| `MYGO_STORE_CATALOG_INTERVAL` | Catalog check interval, for example `6h`; `0` disables it |
| `MYGO_STORE_CATALOG_DISABLE` | Set to `1` to disable runtime catalog sync |
| `MYGO_UPDATER_PRIVATE_KEY` | The update key used when building signed updates |
| `GH_TOKEN` / `GITHUB_TOKEN` | Used when uploading a release |

GitHub's unauthenticated API allows 60 requests an hour. A request is made only when you press 获取, and the app keeps the downloaded file so later presses open it directly.

| Shortcut | Action |
|---|---|
| `⌘F` | Focus the search field |
| `⌘1` / `⌘2` / `⌘3` | Discover / Favorites / Acquired |
| `⌘[` or `Esc` | Back from a detail page |

## Layout

| File | |
|---|---|
| `main.go` | App state, window, sidebar and title bar |
| `update.go` | Automatic update plugin and the **Check for Updates…** menu item |
| `catalog.go` | Embedded catalog, groups, filtering and sorting |
| `catalog_sync.go` | Runtime catalog refresh, cache and status |
| `discover.go` | Discover page and app rows |
| `browse.go` | Category, search, favorites and acquired grids |
| `detail.go` | App detail page and installer actions |
| `acquire.go` | 获取 flow, download seeding and fallback |
| `release.go` | GitHub Releases client, asset matching and downloads |
| `assets.go` | Embedded images, bitmap cache and fallbacks |
| `prefs.go` | Favorites, acquired apps and installer paths |
| `icons.go` | Inline SVG icons |
| `scripts/sync.py` | Catalog and image sync |
| `*_test.go` | Headless interaction tests and snapshots |

## Snapshots

The views render without a window, which makes them easy to inspect in CI:

```sh
SNAP_DIR=/tmp/mygo-shots go test -run TestSnapshots .
```

## License

The app's own code is MIT. App icons, screenshots, names and descriptions belong to their authors.
