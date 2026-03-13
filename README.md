# peth

A lightweight CLI wrapper for [Pinchtab](https://github.com/pinchtab/pinchtab) with web3 support. Ships as a single Go binary with no external runtime dependencies.

| | |
|---|---|
| **Language** | Go |
| **Binary size** | ~6 MB (stripped) |
| **Tests** | 223 tests, 73% coverage |
| **Lines of code** | ~11K (53% tests) |
| **Dependencies** | stdlib only |

## The Problem

Browser automation tools work great for traditional web apps but break down with web3. Wallet extensions (MetaMask, Phantom), transaction signing, chain switching, and dApp interactions can't be automated from the CLI — there's no programmatic interface to the decentralized web stack.

## What peth Does

peth wraps Pinchtab's browser automation and adds first-class web3 support:

- **Connect wallets** — Inject EIP-1193 providers or automate wallet extensions from the CLI
- **Sign transactions** — Approve, reject, or simulate transactions without manual intervention
- **Switch chains** — Automate multi-chain workflows across EVM networks
- **Interact with contracts** — Read and write contract state as part of browser flows
- **Support Solana** — Phantom-compatible provider injection with ed25519 keystore
- **Run in CI** — Proper exit codes, `peth assert` commands, and headless mode

## Requirements

- **[Pinchtab](https://github.com/pinchtab/pinchtab)** installed and running
- **Chrome/Chromium** (managed by Pinchtab)

## Installation

### Download a release binary

Grab the latest from the [releases page](https://github.com/charkala/peth/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/charkala/peth/releases/latest/download/peth-darwin-arm64 -o peth
chmod +x peth && sudo mv peth /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/charkala/peth/releases/latest/download/peth-darwin-amd64 -o peth
chmod +x peth && sudo mv peth /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/charkala/peth/releases/latest/download/peth-linux-amd64 -o peth
chmod +x peth && sudo mv peth /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/charkala/peth/releases/latest/download/peth-linux-arm64 -o peth
chmod +x peth && sudo mv peth /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/charkala/peth.git
cd peth
make build    # Binary at bin/peth
```

## Quick Start

```bash
peth start --headless
peth wallet create my-wallet
peth nav https://app.uniswap.org
peth dapp connect
peth chain switch optimism
peth fill ref:tokenAmount "1.0"
peth click ref:swapButton
peth assert tx $HASH --status success
```

## Commands

### Browser (Pinchtab passthrough)

```bash
peth nav <url>             # Navigate to URL
peth snap                  # Accessibility snapshot
peth click <ref>           # Click element
peth type <ref> <text>     # Type into element
peth press <key>           # Press key
peth fill <ref> <text>     # Fill form field
peth hover <ref>           # Hover element
peth scroll <ref|pixels>   # Scroll
peth select <ref> <value>  # Select dropdown
peth focus <ref>           # Focus element
peth text [--raw]          # Extract text
peth tabs [new|close]      # Manage tabs
peth ss [-o file]          # Screenshot
peth eval <expression>     # Run JavaScript
peth pdf [options]         # Export PDF
peth health                # Server status
peth quick <url>           # Navigate + analyze
```

### Wallet

```bash
peth wallet create <name>                 # Generate new wallet
peth wallet import <name> <key|mnemonic>  # Import from key or seed
peth wallet list                          # List wallets
peth wallet use <name>                    # Set active wallet
```

### Chain

```bash
peth chain list                           # List all chains
peth chain switch <name|id>               # Switch active chain
peth chain add --name Local --rpc http://localhost:8545 --chain-id 31337
```

Built-in: Ethereum, Optimism, Polygon, Arbitrum, Base, Avalanche, BSC, zkSync, Linea, Sepolia, and testnets.

### Transactions

```bash
peth tx send --to 0x... --value 0.1       # Send transaction
peth tx send --dry-run                    # Simulate first
peth tx send --gas-strategy fast          # EIP-1559 gas control
peth token approve --token <addr> --spender <addr> --amount <n>
```

### dApp Automation

```bash
peth dapp connect                         # Auto-detect and connect wallet
peth run script.yaml                      # Run declarative workflow
peth wait --event Transfer --contract 0x... --timeout 30s
```

### Assertions

```bash
peth assert balance 0x... --gte 1.0       # Check balance (exit 0/1)
peth assert tx <hash> --status success    # Verify transaction
peth assert chain --id 10                 # Verify chain ID
```

### Lifecycle

```bash
peth start --headless                     # Start Pinchtab
peth stop                                 # Stop Pinchtab
```

### Dev Chains

```bash
peth devchain start --tool anvil          # Start Anvil/Hardhat
peth devchain snapshot                    # Save state
peth devchain revert <id>                 # Restore state
peth devchain fund <address> 100          # Fund account
```

### MCP Server

```bash
peth mcp --port 3000                      # AI agent integration
```

### Global Flags

```bash
--host     Pinchtab host (default: localhost)
--port     Pinchtab port (default: 9867)
--token    Auth token
```

## Scripted Workflows

```yaml
# script.yaml
name: swap-on-uniswap
steps:
  - nav: https://app.uniswap.org
  - connect-wallet:
  - chain: optimism
  - fill:
      ref: tokenAmount
      text: "1.0"
  - click: swapButton
  - approve-tx:
      max-gas: "0.01"
```

## Architecture

Browser commands pass through to `pinchtab` via `exec`. Web3 commands are handled natively in Go.

```
cmd/peth/          CLI entry, passthrough dispatch, web3 subcommands
internal/
  client/         Pinchtab HTTP client (interface-based)
  wallet/         EVM + Solana keystores, EIP-1193 provider
  chain/          Chain registry, switching, devchain
  tx/             Transaction builder, signing, ERC-20, gas
  dapp/           Wallet connection, SIWE
  script/         YAML workflow runner
  event/          Contract event listener
  lifecycle/      Process management
  mcp/            MCP tool server
testutil/         Shared test helpers
```

## Development

```bash
make build          # Build (stripped, optimized)
make test           # Run all tests
make test-race      # With race detector
make cover-check    # Fail if coverage < 80%
```

See [TDD.md](TDD.md) for the full workflow and testing conventions.

## License

MIT
