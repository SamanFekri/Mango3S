# Releasing Mango3S

Everything is built by GitHub Actions and published as files on a **GitHub
Release** (tag `v<version>`, e.g. `v0.9.0`), where people download them:

| File | For |
|---|---|
| `mango3s-<version>-macos-arm64.zip` | macOS, Apple Silicon |
| `mango3s-<version>-macos-amd64.zip` | macOS, Intel |
| `mango3s-<version>-windows-amd64.zip` | Windows 64-bit |
| `mango3s-<version>-windows-386.zip` | Windows 32-bit |
| `mango3s-<version>-linux-amd64.tar.gz` | Linux 64-bit |

Each file has a matching `.sha256` checksum.

## Publishing a release

1. Set the version in `wails.json` (`info.productVersion`) and push.
2. GitHub › **Actions** › **Build executables** › **Run workflow**:
   - **Platform to build**: `all` (or a subset).
   - **Publish to a GitHub Release**: tick it.
   - **Release tag**: leave empty for `v<version>`. Optional: pre-release, draft.
3. When the run finishes, the files are on the release page
   (`https://github.com/<owner>/<repo>/releases`).

The release is created if it doesn't exist; running again replaces files with
the same name, so platforms can also be published in separate runs (e.g.
`all-except-macos` first, `macos (both)` later). Without the release box
ticked, files are only kept as run artifacts for testing.

## macOS builds

A macOS app can only be built on macOS, so the two macOS jobs need a macOS
runner — Linux or Windows runners can't produce it. If they never start (they
stay queued or fail before any step runs), the usual cause is billing:
on a **private** repository GitHub charges macOS minutes at 10× the Linux rate,
so they stop first when the free minutes or the spending limit run out. The
job page shows the exact reason. Ways to get them running:

- **Make the repository public** — GitHub's runners, macOS included, are free
  for public repositories.
- **Allow Actions spending**: Settings (account or organization) › Billing ›
  set a spending limit above $0 for Actions.
- **Use a Mac you own as the runner**: the workflow still runs in GitHub
  Actions, but the macOS jobs execute on that Mac for free. In the repository:
  Settings › Actions › Runners › **New self-hosted runner** › macOS, and run the
  commands shown on the Mac (keep the default labels). Then run the workflow
  with **Where macOS builds run** = `self-hosted`. One Apple Silicon Mac builds
  both macOS files. Use this only on a private repository.

## First launch for users

The builds aren't code-signed yet, so the release notes tell users:
- macOS: right-click Mango3S.app › Open › Open (once).
- Windows: SmartScreen › More info › Run anyway.
