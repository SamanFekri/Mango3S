# Releasing Mango3S

Everything is built by GitHub Actions and published as files on a **GitHub
Release** (tag `v<version>`, e.g. `v0.9.4`), where people download them:

| File | For |
|---|---|
| `mango3s-<version>-macos-arm64.dmg` | macOS, Apple Silicon: open it and drag Mango3S into Applications |
| `mango3s-<version>-macos-amd64.dmg` | macOS, Intel (same) |
| `mango3s-<version>-windows-amd64-setup.exe` | Windows 64-bit installer |
| `mango3s-<version>-windows-amd64.exe` | Windows 64-bit, portable (no install) |
| `mango3s-<version>-windows-386.exe` | Windows 32-bit, portable |
| `mango3s-<version>-linux-amd64.deb` | Debian / Ubuntu |
| `mango3s-<version>-linux-amd64.tar.gz` | Other Linux (needs `libwebkit2gtk-4.1`) |

The macOS and Windows builds also come as a `.zip`. Every file has a matching
`.sha256` checksum.

The newest release's files have fixed links, for the website:
`https://github.com/<owner>/<repo>/releases/latest/download/<file>`. The file
names contain the version, so the site needs updating with each release.

## Publishing a release

**The quick way: push a tag.**

1. Set the version in `wails.json` (`info.productVersion`) and push.
2. `git tag v0.9.4 && git push origin v0.9.4`

That builds every platform and publishes release `v0.9.4`. A version with a
suffix (`v0.9.4-beta.1`) becomes a pre-release.

**By hand:** GitHub › **Actions** › **Build executables** › **Run workflow**:
- Tick the platforms to build (all are ticked by default).
- **Version**: empty uses `wails.json`.
- **Publish**: tick it to put the files on the release. Without it, the
  files are only kept on the run page for testing (GitHub always zips them
  there; the release has the plain `.dmg`, `.exe` and other files).
- Optional: a different tag, or **draft** to check the release before
  making it public.

## When a platform fails

The other platforms are still published. The run shows a warning, and its
summary page has a table of which platforms the release has and which are
missing. To add a missing one:

- **Build it again:** Run workflow on **Build executables** with only that
  platform ticked, the same version, and **Publish** ticked. Its files are
  added to the existing release; nothing else is touched.
- **Already built, just not published** (e.g. it was a run without Publish):
  Actions › **Publish release** › Run workflow, and enter that run's ID (the
  number at the end of its URL, `…/actions/runs/<ID>`). Files are kept on a
  run page for 90 days.

Publishing again always replaces files with the same name and keeps the rest,
so it's safe to repeat.

## How the workflows fit together

| Workflow | Does |
|---|---|
| `build.yml` (**Build executables**) | Picks the platforms and version, builds each one, then publishes if asked |
| `build-platform.yml` | Builds and packages one platform (used by `build.yml`) |
| `publish-release.yml` (**Publish release**) | Puts built files on a release; used by `build.yml`, or run by hand for an earlier run |

The web platform address built into the app is `https://mango3s.com`. To use
another one, set the repository variable `PLATFORM_BASE_URL` (Settings ›
Secrets and variables › Actions › Variables).

## The macOS disk image

The `.dmg` opens a window with the app, an arrow and an Applications shortcut.
It's made by [dmgbuild](https://github.com/dmgbuild/dmgbuild) from
`build/darwin/dmg-settings.py`. The background is `build/darwin/dmg-background.svg`:
after changing it, run `make dmg-background` (needs `brew install librsvg`)
and commit the `.tiff`.

## macOS builds

A macOS app can only be built on macOS. The repository is public, so GitHub's
macOS runners are free. If you'd rather use a Mac you own (the workflow still
runs in GitHub Actions, but the macOS jobs execute on that Mac): Settings ›
Actions › Runners › **New self-hosted runner** › macOS, run the commands it
shows on the Mac (keep the default labels), then run the workflow with
**Where macOS builds run** = `self-hosted`.

## First launch for users

The builds aren't code-signed yet, so the release notes tell users:
- macOS: right-click Mango3S › Open › Open (once).
- Windows: SmartScreen › More info › Run anyway.
