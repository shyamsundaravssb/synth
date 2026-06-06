# Synth

**The intent layer for Git.**

Synth is a system-level developer CLI tool that sits on top of Git. It captures developer intent — what changed, why it changed, by whom, and in what context — and uses that information to make collaborative development smarter.

## Status

Phase 0 — Foundation. Work in progress.

## Quick Start

```bash
# Build
make build

# Run
./bin/synth --help
./bin/synth --version

# Test
make test
```

## Development

```bash
# Build and show help
make dev

# Run tests with coverage
make test-coverage

# Lint (requires golangci-lint)
make lint

# Clean build artifacts
make clean
```

## License

All rights reserved.
