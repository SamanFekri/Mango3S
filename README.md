<div align="center">

<img src="build/appicon.png" alt="Mango3S" width="112" height="112" />

# Mango3S

**The free desktop IDE for MongoDB.**<br />
Explore, query, aggregate and manage your databases in one fast app.

*Say it fast: Mango Trees.* 🌳🥭

[**Download at mango3s.com →**](https://mango3s.com)

[![Website](https://img.shields.io/badge/website-mango3s.com-ea580c)](https://mango3s.com)
[![Platforms](https://img.shields.io/badge/macOS%20·%20Windows%20·%20Linux-desktop-16a34a)](https://mango3s.com)
[![License: GPL v3](https://img.shields.io/badge/license-GPLv3-2563eb)](LICENSE)

<br />

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://mango3s.com/screenshots/collection-dark.webp" />
  <img src="https://mango3s.com/screenshots/collection-light.webp" alt="Browsing a collection in Mango3S" width="880" />
</picture>

</div>

## Why Mango3S

- **Free, for real.** No account, no license key, no trial that runs out.
- **Fast and native.** A real desktop app for macOS, Windows and Linux, not a browser tab.
- **Safe by default.** Anything that changes data shows you a preview first and backs up what it
  touches. Your passwords stay in your system's keychain.
- **Everything in one place.** From a quick look at a document to migrating a whole SQL database.

## What you can do

**🔌 Connect to anything.** Local servers, replica sets, sharded clusters and Atlas. SSH tunnels,
proxies, TLS certificates and every common way to sign in.

**🔍 Explore your data.** Browse documents as a tree, a table or JSON. Double-click any value to
edit it in place.

**⚡ Query your way.** Use mongosh syntax with autocomplete, build queries visually, or just write
SQL and let Mango3S translate it.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://mango3s.com/screenshots/aggregation-dark.webp" />
  <img src="https://mango3s.com/screenshots/aggregation-light.webp" alt="The aggregation editor" width="880" />
</picture>

**🧱 Build aggregations stage by stage.** Preview the output of every stage as you go.

**🧬 Understand your schema.** See every field, its types and how often it shows up, then get
index suggestions for your slow queries.

**💻 Turn queries into code.** Copy any query or pipeline as ready-to-run Node.js, Python, Java,
C#, Go or mongosh.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://mango3s.com/screenshots/schema-dark.webp" />
  <img src="https://mango3s.com/screenshots/schema-light.webp" alt="The schema explorer" width="880" />
</picture>

**📦 Move data around.** Import and export JSON and CSV, back up and restore databases, compare
and sync collections, and migrate from PostgreSQL, MySQL or SQLite.

**🛡️ Work with care.** Mask sensitive data for test environments, schedule recurring jobs, and
roll back changes from automatic backups.

**✨ Ask AI (optional).** Describe what you want in plain words and get a query back. It's off by
default and only ever sees field names, never your data or credentials.

**🔄 Sync across computers (optional).** Bring your connections to every machine, end-to-end
encrypted. Nothing in the app needs an account.

And yes, there's Mango 🥭, a small mascot who cheers you on. You can hide it in Settings.

## Get Mango3S

Go to **[mango3s.com](https://mango3s.com)**. It detects your system and suggests the right
download, including whether your Mac has Apple Silicon or an Intel chip.

| | |
|---|---|
| **macOS** (Apple Silicon or Intel) | Unzip and drag **Mango3S** to Applications. The first time, macOS may say it can't verify the app: open **System Settings → Privacy & Security** and click **Open Anyway**. |
| **Windows** (64-bit or 32-bit) | Extract the `.zip` and run **mango3s.exe**. If SmartScreen appears, click **More info → Run anyway**. |
| **Linux** (64-bit) | Extract the `.tar.gz` and run **mango3s**. It needs WebKitGTK (`libwebkit2gtk-4.1-0`), which most desktops already have. |

Every download has a SHA-256 checksum, and all versions are also on
[GitHub Releases](https://github.com/SamanFekri/Mango3S/releases).

Mango3S updates itself: it checks for new versions, verifies the download and asks before
installing.

**Try it in a minute:** click **New connection**, enter `mongodb://localhost:27017` (or your
Atlas connection string), then **Save & connect**.

## Contribute

Mango3S is open source, and contributions are very welcome: bug reports, ideas, docs and code.

- **Found a bug or have an idea?** [Open an issue](https://github.com/SamanFekri/Mango3S/issues).
- **Want to write code?** Here's how to run it locally:

```bash
git clone https://github.com/SamanFekri/Mango3S.git && cd Mango3S
make mongo-up   # optional: a local MongoDB with sample data (needs Docker)
make dev        # the app with hot reload
make test       # run before opening a pull request
```

You'll need Go 1.25+, Node.js 20+ and the [Wails CLI](https://wails.io) (`wails doctor` checks
your setup). Keep pull requests focused, add tests for new behavior, and describe what changed and
why.

The full feature list, build options, architecture and troubleshooting live in
**[docs/REFERENCE.md](docs/REFERENCE.md)**.

## License

Mango3S is free software under the [GNU GPL v3](LICENSE). Copyright © 2026 Saman Fekri.

<div align="center">
<br />
<a href="https://mango3s.com"><b>mango3s.com</b></a> · made by <a href="https://samanfekri.me">Saman Fekri</a>
</div>
