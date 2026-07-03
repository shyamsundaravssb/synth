# Synth

Git tracks what changed. Synth captures why.

[![Release](https://img.shields.io/github/v/release/shyamsundaravssb/synth?color=blue)](https://github.com/shyamsundaravssb/synth/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey)](https://github.com/shyamsundaravssb/synth/releases)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)

---

Code history tells you what changed. It rarely tells you why a decision was made, what trade-offs were considered, or which other files a change was expected to affect. Synth adds that layer. You attach intent notes to commits, Synth embeds them as 384-dimension semantic vectors, and later you find them by meaning — not by guessing the exact words you used six months ago. Everything runs locally. No account, no cloud, no data leaving your machine.

---

## 🎬 Demo

> *Screenshot coming soon*

---

## ✨ Features

| Feature | Description |
|---|---|
| **Intent capture** | `synth note` records why a change was made, what it affects, and links it to the current Git state |
| **Semantic search** | `synth search` finds notes by meaning using local embeddings — no keyword guessing required |
| **Low-context detection** | `synth status --low-context` flags files that have accumulated changes without any recorded intent |
| **Background daemon** | A lightweight daemon watches file saves, indexes notes, and keeps embeddings current |
| **VS Code extension** | Live status bar, daemon health indicator, and in-editor semantic search via Command Palette |
| **Local-first** | No account required. No data leaves your machine. The model runs on-device (~87 MB, one-time download) |

---

## 📦 Installation

### Homebrew (macOS and Linux)

```bash
brew tap shyamsundaravssb/synth
brew install synth
```

### Quick Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/shyamsundaravssb/synth/main/scripts/install.sh | bash
```

To install a specific version:

```bash
SYNTH_VERSION=v0.1.0 curl -fsSL \
  https://raw.githubusercontent.com/shyamsundaravssb/synth/main/scripts/install.sh | bash
```

Requires `curl` and `sha256sum` (Linux) or `shasum` (macOS) — both are standard on their respective platforms.

### From Source

Requires Go 1.26+.

```bash
go install github.com/shyamsundaravssb/synth/cmd/synth@latest
```

---

## 🚀 Getting Started

```bash
# 1. Move into any Git repository
cd your-project

# 2. Initialize Synth — creates .synth/ to store notes and the local DB
synth init

# 3. Download the embedding model (one-time, ~87 MB)
#    Required for semantic search. Runs entirely on-device.
synth model download

# 4. Start the background daemon
#    Watches for file saves and keeps the intent index current
synth daemon start

# 5. Capture intent after making a change
#    Attach a note to the current Git state
synth note --file auth.go \
           --what "removed rate limiting" \
           --why "API is internal-only, no longer needed"

# 6. Find notes by meaning, not keywords
synth search "why did we remove rate limiting"

# 7. See which files have changes with no recorded intent
synth status --low-context
```

---

## 📖 Commands Reference

| Command | Description | Example |
|---|---|---|
| `synth init` | Initialize Synth in a Git repository | `synth init` |
| `synth note` | Attach an intent note to the current Git state | `synth note --file auth.go --what "fix auth bug"` |
| `synth log` | List intent notes, optionally filtered by file or date | `synth log --file main.go` |
| `synth status` | Show daemon status and file coverage | `synth status` |
| `synth status --low-context` | List files with changes but no recorded intent | `synth status --low-context` |
| `synth search` | Search notes by meaning using semantic embeddings | `synth search "why did we change auth"` |
| `synth daemon start` | Start the background daemon | `synth daemon start` |
| `synth daemon stop` | Stop the background daemon | `synth daemon stop` |
| `synth daemon install-service` | Install as a system service (auto-starts on login) | `synth daemon install-service` |
| `synth model download` | Download the on-device embedding model (~87 MB) | `synth model download` |
| `synth model status` | Show embedding model status and path | `synth model status` |

---

## 🧩 VS Code Extension

The extension lives in `plugins/vscode/` and is not yet published to the VS Code Marketplace.

It adds a **live status bar** showing daemon health, an **intent note panel** for capturing notes without leaving the editor, and a **Command Palette entry** (`Ctrl+Alt+S`) that runs `synth search` against your query inline.

To load it locally:

1. Open `plugins/vscode/` in VS Code
2. Press `F5` to launch the Extension Development Host
3. Open a Git repo that has been initialized with `synth init`

---

## ⚙️ Requirements

- **Git** — Synth operates on Git repositories
- **Linux** (amd64, arm64) or **macOS** (amd64, arm64)
- **Go 1.26+** — only required when installing from source
- **Embedding model** — run `synth model download` once after installation (~87 MB); required for `synth search`

---

## ⚠️ Known Limitations

- `synth daemon install-service` (launchd auto-start on macOS) has not been verified on physical Mac hardware
- The VS Code extension is not yet published to the Marketplace
- Windows is not supported

---

## 🤝 Contributing

Issues and pull requests are welcome. If you hit a bug or want to propose a feature, open an issue at [github.com/shyamsundaravssb/synth](https://github.com/shyamsundaravssb/synth/issues). For larger changes, open an issue first to discuss the direction before sending a PR.

---

## 📄 License

MIT — Bandaru Shyam Sundara Venkata Satya Sai. See [LICENSE](LICENSE).
