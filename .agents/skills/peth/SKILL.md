---
name: peth
description: Web3 browser automation CLI - wallet management, chain switching, transaction signing, and dApp interactions via Pinchtab
version: 0.2.0
tools:
  - peth
---

# peth — Web3 Browser Automation

## What peth does

peth is a Go CLI that wraps Pinchtab to add web3 browser automation capabilities. It solves the problem that wallet extensions (MetaMask, Phantom) cannot be automated programmatically — transaction signing, chain switching, and dApp wallet connections all require manual UI interaction. peth injects web3 providers directly, making these operations scriptable CLI commands.

Supported ecosystems:
- **EVM chains** (Ethereum, Optimism, Polygon, Arbitrum, Base, Avalanche, BSC, zkSync, Linea, testnets)
- **Solana** (Phantom-compatible provider injection)

## Available Commands

### Browser Commands (Pinchtab passthrough)
- `peth nav <url>` — Navigate the browser to a URL
- `peth snap` — Take an accessibility snapshot of the current page
- `peth click <ref>` — Click an element by accessibility reference
- `peth fill <ref> <text>` — Fill an input element with text
- `peth eval <expression>` — Evaluate a JavaScript expression in the page
- `peth press <key>` — Press a keyboard key

### Wallet Commands
- `peth wallet create <name>` — Generate a new wallet (EVM or Solana)
- `peth wallet import <name> <key|mnemonic>` — Import from private key or seed phrase
- `peth wallet list` — List all wallets in the keystore
- `peth wallet use <name>` — Set the active wallet
- `peth wallet delete <name>` — Remove a wallet

### Chain Commands
- `peth chain list` — List all registered EVM chains
- `peth chain switch <name|id>` — Switch the active chain (by name, short name, or chain ID)
- `peth chain add <json>` — Register a custom chain

### Transaction Commands
- `peth tx send --to <addr> --value <amount>` — Build, sign, and send a transaction
- `peth token approve <token> <spender> <amount>` — Approve ERC-20 token spending

### Assertion Commands (CI-friendly)
- `peth assert balance <addr> <operator> <amount>` — Assert on-chain balance (gte, lte, eq, gt, lt)
- `peth assert tx <hash> --status <success|failed>` — Assert transaction receipt status
- `peth assert chain <id>` — Assert the connected chain ID

### dApp Workflow Commands
- `peth dapp connect` — Inject provider and trigger wallet connection
- `peth run <script.yaml>` — Execute a declarative automation script

### Lifecycle Commands
- `peth start [--headless] [--port N]` — Start a Pinchtab instance
- `peth stop` — Stop the running Pinchtab instance
- `peth update` — Self-update to the latest release

### Development Chain Commands
- `peth devchain start [--tool anvil|hardhat] [--port N]` — Start a local dev chain
- `peth devchain stop` — Stop the local dev chain
- `peth devchain snapshot` — Create an EVM state snapshot
- `peth devchain revert <id>` — Revert to a snapshot
- `peth devchain fund <addr> <amount>` — Set an account balance

### MCP Server Mode
- `peth mcp serve [--port N]` — Start an MCP server exposing peth tools over HTTP

## Common Workflows

### Automated DEX swap
```bash
peth start --headless
peth wallet import trader "seed phrase here..."
peth nav https://app.uniswap.org
peth dapp connect
peth chain switch optimism
peth fill ref:tokenAmount "1.0"
peth click ref:swapButton
peth assert tx $TX_HASH --status success
peth stop
```

### CI balance verification
```bash
peth assert balance 0xMyAddr gte 0.1
peth assert chain 1
```

### Local development with Anvil
```bash
peth devchain start --tool anvil --port 8545
peth devchain fund 0xMyAddr 100
peth wallet create dev-wallet
peth chain switch 31337
# ... run tests ...
peth devchain stop
```

### Multi-wallet testing
```bash
peth wallet create alice
peth wallet create bob
# Assign different wallets to different browser tabs
# for multi-account dApp interaction testing
```

## Requirements

- Pinchtab running (default http://localhost:9867)
- Go binary at `bin/peth` (build with `make build`)
- For devchain: `anvil` (Foundry) or `npx hardhat` installed

## Example Usage

```bash
# Build the binary
make build

# Quick health check
peth health

# Create a wallet and navigate to a dApp
peth wallet create my-wallet
peth start --headless
peth nav https://app.aave.com
peth dapp connect
peth snap

# Send a transaction on a local chain
peth devchain start --tool anvil
peth devchain fund 0x1234...5678 10
peth tx send --to 0xdead...beef --value 0.5
peth assert tx $HASH --status success
```
