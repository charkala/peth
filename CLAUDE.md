# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Directory

**All file operations must stay within the project root (`/Users/charkala/projects/charkala/peth/`).** Do not read, write, create, or modify files outside this directory. Do not `cd` out of it. If a task would require touching files outside the project boundary, stop and ask for confirmation first.

## Project Overview

peth is a Go CLI that wraps [Pinchtab](https://pinchtab.ai) browser automation with web3 support — wallet connections, transaction signing, chain switching, and dApp interactions. Browser commands pass through to the `pinchtab` binary; web3 commands are handled natively.

## How It Works

```
peth <command> [args...]
    │
    ├── Pinchtab command? → exec pinchtab <command> [args...] (passthrough)
    │   nav, snap, click, type, press, fill, hover, scroll, select,
    │   focus, text, tabs, ss, eval, pdf, health, quick
    │
    └── peth command? → handled natively by peth
        wallet, chain, tx, token, dapp, run, wait, assert,
        devchain, start, stop, version, help
```

**Example end-to-end flow:**
```bash
peth start --headless
peth wallet create my-wallet
peth wallet use my-wallet
peth nav https://app.uniswap.org
peth dapp connect
peth chain switch optimism
peth fill ref:tokenAmount "1.0"
peth click ref:swapButton
peth assert tx $HASH --status success
```

## Design Principles

peth must produce a **light, efficient, and performant binary**. Every dependency and abstraction must justify its cost:

- **Minimal binary size** — the build is stripped and optimized (`-ldflags="-s -w"`); avoid heavy dependencies that bloat the binary
- **No external runtimes** — no Node.js, no bundled interpreters; use Go-native libraries (e.g., `go-ethereum`)
- **Performance-first** — prefer zero-allocation patterns, avoid unnecessary goroutines or reflection; match Pinchtab's small bundle philosophy
- **Lean dependencies** — evaluate every new `go get` for size and transitive dependency impact
- **Pinchtab passthrough** — browser commands shell out to `pinchtab` binary (not reimplemented via HTTP); `internal/client` is only used internally by packages like `dapp` and `script`

## Commands

```bash
# Build
make build                    # Compile to bin/peth (stripped, optimized)
make clean                    # Remove bin/, coverage files

# Test
make test                     # Run all unit tests
make test-run T=TestName      # Run a specific test by name
make test-v                   # Verbose output
make test-race                # With race detector

# Coverage (minimum threshold: 80%)
make test-cover               # Run tests + print coverage %
make cover-func               # Per-function coverage breakdown
make cover-html               # Generate HTML coverage report
make cover-check              # Fail if coverage < 80%
make cover-check COVER_MIN=90 # Override threshold

# Integration tests (requires running Pinchtab instance)
go test -tags integration ./...
```

## Architecture

```
cmd/peth/
  main.go           — CLI entry point, flag parsing, command dispatch
  passthrough.go    — Pinchtab command passthrough (exec pinchtab)
internal/
  client/           — Pinchtab HTTP client (used internally by dapp, script)
  wallet/           — EVM keystore, Solana keystore, EIP-1193 provider, MetaMask automator
  chain/            — EVM chain registry, switching, custom RPC, devchain (Anvil/Hardhat)
  tx/               — Transaction builder, signing, interception, ERC-20, simulation, gas
  dapp/             — Connect wallet flow, SIWE support
  script/           — Declarative YAML workflow runner
  event/            — Contract event listener with polling and wait conditions
  lifecycle/        — Pinchtab process management, Chrome extension loader
  mcp/              — MCP tool server for AI agent integration
testutil/           — Shared test helpers and fixtures
```

## Development Workflow

This project follows strict TDD (Red → Green → Refactor):

1. Write a failing test first
2. Implement minimum code to pass
3. Refactor while keeping tests green
4. **Commit frequently** — each passing Green or Refactor step is a good commit point. Aim for mid-sized commits that capture one coherent change (e.g., "add wallet keystore with tests"), not single-line tweaks or giant multi-feature dumps.
5. Verify with `make cover-func` and `make cover-check`

## Testing Conventions

- Tests live next to source files (`_test.go` suffix, same package)
- Use **table-driven tests** for multi-case scenarios
- Use **interfaces + mock structs** for Pinchtab client and network dependencies (keeps unit tests fast and deterministic)
- `run()` in main.go accepts injectable dependencies (`passthroughFunc`, `appConfig`) for testability
- Integration tests use `//go:build integration` build tag
- Coverage skip list: `main.go` entry point, generated code, thin CLI wiring, third-party library calls

## Releasing

When tagging a new release:

1. `git tag vX.Y.Z && git push origin main --tags`
2. Update `.agents/skills/peth/SKILL.md` — bump the `version` field and ensure commands, workflows, and examples reflect any added, changed, or removed features

## Key Dependencies

- **Go 1.21+** (module: `github.com/charkala/peth`)
- **Pinchtab** — browser automation engine (must be installed and on PATH)
- **Standard library only** — crypto primitives use P-256/SHA-256 placeholders (swap to go-ethereum's secp256k1/keccak256 when needed)
