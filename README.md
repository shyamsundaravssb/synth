# Synth

Intent-aware Git companion — capture why changes happen.

## What It Does
Synth sits on top of Git to capture developer intent. It tracks what changed, why it changed, and securely generates a semantic search index of your codebase history. A background daemon monitors for low-context files that need attention, keeping your team's knowledge fresh.

## Installation

### Homebrew (macOS and Linux)
(note: tap not yet public — coming soon)
```bash
brew tap shyamsundaravssb/synth
brew install synth
```

### Quick Install (Linux)
(requires GitHub CLI authenticated to this private repo — update this section when repo goes public to use a simpler curl approach)
```bash
bash scripts/install.sh
```

### From Source
```bash
go install github.com/shyamsundaravssb/synth/cmd/synth@latest
```

## Getting Started

```bash
# Initialize Synth in your Git repo
synth init

# Download the semantic search embedding model (one-time, ~87MB)
synth model download

# Start the background daemon
synth daemon start

# Capture a note about a file change
synth note

# Search intent notes semantically
synth search
```

## Requirements
- Git
- Go 1.26+ (only if installing from source)
- For semantic search: `synth model download` (one-time, ~87MB)

## VS Code Extension
A local `.vsix` extension exists in `plugins/vscode/`. You can load it by opening the extension folder in VS Code and pressing F5 to launch the Extension Development Host.
Note: it's not yet published to the VS Code Marketplace.

## Known Limitations
- macOS launchd service install (`synth daemon install-service`) untested on real Mac hardware
- VS Code extension not yet published to Marketplace
- Homebrew tap not yet public
