<div align="center">

<a href="https://mango3s.com"><img src="build/appicon.png" alt="Mango3S app icon" width="112" height="112" /></a>

<a href="https://mango3s.com"><img src="docs/media/banner.svg" alt="Mango3S: the free desktop IDE for MongoDB" width="880" /></a>

*Say it fast: Mango Trees.* 🌳🥭

[![Latest release](https://img.shields.io/github/v/release/SamanFekri/Mango3S?label=latest&color=f08c2a&logo=github)](https://github.com/SamanFekri/Mango3S/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/SamanFekri/Mango3S/total?color=16a34a&label=downloads)](https://github.com/SamanFekri/Mango3S/releases)
[![License: GPL v3](https://img.shields.io/badge/license-GPLv3-2563eb)](LICENSE)
<br />
[![macOS · Windows · Linux](https://img.shields.io/badge/macOS%20·%20Windows%20·%20Linux-desktop-555)](https://github.com/SamanFekri/Mango3S/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/SamanFekri/Mango3S?logo=go&logoColor=white)](go.mod)
[![Built with Wails](https://img.shields.io/badge/built%20with-Wails-df0000)](https://wails.io)
[![Last commit](https://img.shields.io/github/last-commit/SamanFekri/Mango3S?color=8b5cf6)](https://github.com/SamanFekri/Mango3S/commits/main)

<br />

[![Download for macOS](https://img.shields.io/badge/Download-macOS-111?style=for-the-badge&logo=apple&logoColor=white)](https://github.com/SamanFekri/Mango3S/releases/latest)
[![Download for Windows](https://img.shields.io/badge/Download-Windows-0078d4?style=for-the-badge&logo=windows&logoColor=white)](https://github.com/SamanFekri/Mango3S/releases/latest)
[![Download for Linux](https://img.shields.io/badge/Download-Linux-f08c2a?style=for-the-badge&logo=linux&logoColor=white)](https://github.com/SamanFekri/Mango3S/releases/latest)

**Free. No account. No license key. No trial that runs out.**

<br />

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://mango3s.com/screenshots/collection-dark.webp" />
  <img src="https://mango3s.com/screenshots/collection-light.webp" alt="Browsing a collection in Mango3S" width="880" />
</picture>

</div>

## ✨ New in the latest version

- **📈 Live server monitor.** Operations per second, connections, cache, replica-set lag and the
  operations running right now, with a button to stop a runaway one.
- **📊 Charts from your data.** Turn any query or aggregation into a bar, line or pie chart.
- **🟨 A real JavaScript shell.** Variables, loops and functions against your database, or run your
  own `mongosh`.
- **🌲 Explain that finds the slow part.** The query plan as a tree, a side-by-side comparison, and
  a one-click index suggestion.
- **👀 Watch changes live**, **🗂️ browse GridFS files**, **🧪 generate test data**, and manage
  **sharded clusters**.

<div align="center">
<img src="docs/media/explain.svg" alt="Explain turns a COLLSCAN of 1,204 ms into an IXSCAN of 4 ms" width="880" />
</div>

See every version's changes on the [releases page](https://github.com/SamanFekri/Mango3S/releases).

## 🥭 Why Mango3S

- **Free, for real.** Everything is in the free app. There's nothing to unlock.
- **Fast and native.** A real desktop app for macOS, Windows and Linux, not a browser tab.
- **Safe by default.** Anything that changes data shows you a preview first and backs up what it
  touches. Your passwords stay in your system's keychain.
- **Everything in one place.** From a quick look at a document to migrating a whole SQL database.

## 🧰 What you can do

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

<details>
<summary><b>And a lot more…</b></summary>
<br />

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

**🧹 Keep your disk tidy.** Settings › Storage shows what Mango3S keeps on your computer and clears
caches and logs in one click.

The full list is in **[docs/FEATURES.md](docs/FEATURES.md)**.

</details>

And yes, there's Mango 🥭, a small mascot who cheers you on. You can hide it in Settings.

## <img src="build/appicon.png" alt="" width="28" height="28" align="top" /> Get Mango3S

Download the latest version from **[GitHub Releases](https://github.com/SamanFekri/Mango3S/releases/latest)**
or **[mango3s.com](https://mango3s.com)**, which detects your system and picks the right file.

| System | Download | Install |
|---|---|---|
| **macOS**, Apple Silicon (M1 and later) | `…-macos-arm64.dmg` | Open it and drag **Mango3S** into **Applications**. |
| **macOS**, Intel | `…-macos-amd64.dmg` | Same. |
| **Windows** 64-bit | `…-windows-amd64-setup.exe` | Run the installer. Or use the portable `.exe`. |
| **Windows** 32-bit | `…-windows-386.exe` | Portable: just run it. |
| **Linux** (Debian, Ubuntu) | `…-linux-amd64.deb` | `sudo apt install ./mango3s-*.deb` |
| **Linux** (other) | `…-linux-amd64.tar.gz` | Extract and run `mango3s`. Needs WebKitGTK (`libwebkit2gtk-4.1-0`). |

<details>
<summary><b>"Can't be opened" or "Windows protected your PC"?</b></summary>
<br />

Mango3S isn't code-signed yet, so your system asks once:

- **macOS:** right-click Mango3S › **Open** › **Open**. Or open **System Settings › Privacy &
  Security** and click **Open Anyway**.
- **Windows:** in the SmartScreen dialog, click **More info › Run anyway**.

Every download has a `.sha256` checksum if you want to verify it.

</details>

Mango3S updates itself: it checks for new versions, verifies the download and asks before
installing.

**Try it in a minute:** click **New connection**, enter `mongodb://localhost:27017` (or your
Atlas connection string), then **Save & connect**.

## 🤝 Contribute

Mango3S is open source, and contributions are very welcome: bug reports, ideas, docs and code.

- **🐛 Found a bug or have an idea?** [Open an issue](https://github.com/SamanFekri/Mango3S/issues).
- **⭐ Like it?** [Star the repo](https://github.com/SamanFekri/Mango3S): it really helps other
  people find it.
- **🧑‍💻 Want to write code?** Here's how to run it locally:

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
**[docs/REFERENCE.md](docs/REFERENCE.md)**. Releases are explained in
**[docs/RELEASING.md](docs/RELEASING.md)**.

## 👋 Made by

<a href="https://github.com/SamanFekri"><img src="https://github.com/SamanFekri.png?size=120" alt="Saman Fekri" width="60" height="60" align="left" /></a>

Mango3S is built by **[@SamanFekri](https://github.com/SamanFekri)**. Say hi, share what you
built with it, or tell me what to add next.<br />
[![Follow @SamanFekri](https://img.shields.io/github/followers/SamanFekri?label=Follow%20%40SamanFekri&style=social)](https://github.com/SamanFekri)
[![Website](https://img.shields.io/badge/samanfekri.me-ea580c)](https://samanfekri.me)

<br clear="left" />

## ⭐ Star history

<a href="https://star-history.com/#SamanFekri/Mango3S&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=SamanFekri/Mango3S&type=Date&theme=dark" />
    <img alt="Star history of Mango3S" src="https://api.star-history.com/svg?repos=SamanFekri/Mango3S&type=Date" width="600" />
  </picture>
</a>

## 📄 License

Mango3S is free software under the [GNU GPL v3](LICENSE). Copyright © 2026 Saman Fekri.

<div align="center">
<br />
<a href="https://mango3s.com"><b>mango3s.com</b></a> · made with 🥭 by <a href="https://github.com/SamanFekri">@SamanFekri</a>
</div>
