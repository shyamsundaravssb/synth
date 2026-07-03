# Synth

The intent layer for Git. Capture *why* changes
happen alongside *what* changed — then find
anything instantly with natural language search.

## What It Does

Synth sits on top of Git and records developer
intent: what changed, why it changed, and what
else it might affect. Notes are stored locally,
embedded into 384-dimension semantic vectors,
and searchable in plain English.

When you type `synth search "why did we remove
email verification"`, Synth finds the relevant
notes by meaning — not just keyword matching.
When files accumulate uncommitted changes without
any notes, Synth flags them so nothing goes
undocumented.

Everything runs locally. No account required.
No data leaves your machine.

## Installation

### Homebrew (macOS and Linux)

  brew tap shyamsundaravssb/synth
  brew install synth

### Quick Install Script (Linux and macOS)

  curl -fsSL https://raw.githubusercontent.com/\
shyamsundaravssb/synth/main/scripts/install.sh \
  | bash

To install a specific version:

  SYNTH_VERSION=v0.1.0 curl -fsSL \
  https://raw.githubusercontent.com/\
shyamsundaravssb/synth/main/scripts/install.sh \
  | bash

### From Source (requires Go 1.26+)

  go install \
  github.com/shyamsundaravssb/synth/cmd/synth@latest

## Getting Started

  # 1. Initialize in any Git repository
  cd your-project
  synth init

  # 2. Download the embedding model (one-time, ~87MB)
  #    Required for semantic search
  synth model download

  # 3. Start the background daemon
  synth daemon start

  # 4. Capture intent when you make a change
  synth note

  # 5. Search by meaning, not just keywords
  synth search "why did we remove rate limiting"

  # 6. See what needs documentation
  synth status --low-context

## Requirements

- Git
- Go 1.26+ (only if installing from source)
- Linux (amd64, arm64) or macOS (amd64, arm64)
- For semantic search: run `synth model download`
  once after installation (~87MB, one-time)

## VS Code Extension

A VS Code extension is included in `plugins/vscode/`
providing a live status bar, daemon health indicator,
and in-editor semantic search.

To load it locally:
  1. Open `plugins/vscode/` in VS Code
  2. Press F5 to launch the Extension
     Development Host
  3. Run "Synth: Search Intent Notes" from
     the Command Palette (Ctrl+Alt+S)

The extension is not yet published to the
VS Code Marketplace.

## Known Limitations

- macOS `synth daemon install-service` (launchd
  auto-start) has not been verified on physical
  Mac hardware
- VS Code extension is not yet published to
  the Marketplace
- Windows is not supported

## License

MIT — see [LICENSE](LICENSE)
