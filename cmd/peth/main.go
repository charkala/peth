// Package main is the entry point for the peth CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charkala/peth/internal/chain"
	"github.com/charkala/peth/internal/client"
	"github.com/charkala/peth/internal/dapp"
	"github.com/charkala/peth/internal/event"
	"github.com/charkala/peth/internal/lifecycle"
	"github.com/charkala/peth/internal/script"
	"github.com/charkala/peth/internal/tx"
	"github.com/charkala/peth/internal/update"
	"github.com/charkala/peth/internal/wallet"
)

// version is set at build time via -ldflags.
var version = "dev"

// appConfig holds injectable dependencies for testing.
type appConfig struct {
	walletDir        string
	customChainsPath string
	activeChainPath  string
}

// defaultConfig returns the default appConfig using the user's home directory.
func defaultConfig() (appConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return appConfig{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	return appConfig{
		walletDir:        filepath.Join(home, ".peth", "wallets"),
		customChainsPath: filepath.Join(home, ".peth", "chains.json"),
		activeChainPath:  filepath.Join(home, ".peth", "active-chain"),
	}, nil
}

func main() {
	cfg, err := defaultConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := run(os.Args[1:], os.Stdout, os.Stderr, execPassthrough, cfg); err != nil {
		if exitErr, ok := err.(*exitError); ok {
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer, passthrough passthroughFunc, cfg appConfig) error {
	fs := flag.NewFlagSet("peth", flag.ContinueOnError)
	fs.SetOutput(stderr)

	host := fs.String("host", "localhost", "Pinchtab host")
	port := fs.Int("port", 9867, "Pinchtab port")
	token := fs.String("token", "", "Pinchtab auth token")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return nil
		}
		return err
	}

	remaining := fs.Args()

	if len(remaining) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	// Pinchtab passthrough commands.
	// For "nav": if a wallet is active, inject the EIP-1193 provider preload
	// via CDP before navigating, so wallet-detection SDKs (Privy, Dynamic, etc.)
	// see window.ethereum during their initialization.
	if isPinchtabCommand(cmd) {
		if cmd == "nav" && len(cmdArgs) > 0 {
			if err := runNavWithPreload(cmdArgs, passthrough, cfg); err != nil {
				// Non-fatal: fall through to normal nav if preload fails.
				_ = err
			} else {
				return nil
			}
		}
		return passthrough(cmd, cmdArgs)
	}

	// Lazy builders for shared dependencies.
	makeClient := func() client.Pinchtab {
		baseURL := fmt.Sprintf("http://%s:%d", *host, *port)
		var opts []client.Option
		if *token != "" {
			opts = append(opts, client.WithToken(*token))
		}
		return client.New(baseURL, opts...)
	}

	makeKeystore := func() (*wallet.Keystore, error) {
		return wallet.NewKeystore(cfg.walletDir)
	}

	makeRegistry := func() *chain.Registry {
		reg := chain.NewRegistry()
		_ = reg.LoadCustom(cfg.customChainsPath)
		return reg
	}

	switch cmd {
	case "version":
		fmt.Fprintf(stdout, "peth version %s\n", version)
		return nil

	case "help", "--help":
		printUsage(stdout)
		return nil

	// --- Wallet commands ---
	case "wallet":
		return runWallet(cmdArgs, stdout, makeKeystore)

	// --- Chain commands ---
	case "chain":
		return runChain(cmdArgs, stdout, makeRegistry, cfg.customChainsPath, cfg.activeChainPath)

	// --- Transaction commands ---
	case "tx":
		return runTx(cmdArgs, stdout, makeKeystore, makeRegistry, cfg.activeChainPath)

	// --- Token commands ---
	case "token":
		return runToken(cmdArgs, stdout, makeKeystore, makeRegistry, cfg.activeChainPath)

	// --- Assert commands ---
	case "assert":
		return runAssert(cmdArgs, stdout, makeRegistry, cfg.activeChainPath)

	// --- dApp commands ---
	case "dapp":
		return runDapp(cmdArgs, stdout, makeClient, makeKeystore)

	// --- Script runner ---
	case "run":
		return runScript(cmdArgs, stdout, makeClient, makeKeystore, makeRegistry)

	// --- Event listener ---
	case "wait":
		return runWait(cmdArgs, stdout, makeRegistry, cfg.activeChainPath)

	// --- Devchain ---
	case "devchain":
		return runDevchain(cmdArgs, stdout, makeRegistry, cfg.customChainsPath)

	// --- Lifecycle ---
	case "start":
		return runStart(cmdArgs)

	case "stop":
		return runStop()

	case "update":
		return runUpdate(stdout)

	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// --- Wallet subcommands ---

func runWallet(args []string, stdout io.Writer, makeKeystore func() (*wallet.Keystore, error)) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth wallet <create|import|list|use> [args...]")
	}

	ks, err := makeKeystore()
	if err != nil {
		return err
	}

	switch args[0] {
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: peth wallet create <name>")
		}
		key, err := ks.Create(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created wallet %q: %s\n", key.Name, key.Address)
		return nil

	case "import":
		if len(args) < 3 {
			return fmt.Errorf("usage: peth wallet import <name> <key|mnemonic>")
		}
		input := strings.Join(args[2:], " ")
		key, err := ks.Import(args[1], input)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Imported wallet %q: %s\n", key.Name, key.Address)
		return nil

	case "list":
		keys, err := ks.List()
		if err != nil {
			return err
		}
		active, _ := ks.Active()
		if len(keys) == 0 {
			fmt.Fprintln(stdout, "No wallets found.")
			return nil
		}
		for _, k := range keys {
			marker := "  "
			if active != nil && active.Name == k.Name {
				marker = "* "
			}
			fmt.Fprintf(stdout, "%s%s  %s\n", marker, k.Name, k.Address)
		}
		return nil

	case "use":
		if len(args) < 2 {
			return fmt.Errorf("usage: peth wallet use <name>")
		}
		if err := ks.Use(args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Active wallet set to %q\n", args[1])
		return nil

	case "sign":
		return runWalletSign(args[1:], stdout, makeKeystore)

	default:
		return fmt.Errorf("unknown wallet command: %s", args[0])
	}
}

// --- Chain subcommands ---

// activeChain reads the persisted active chain name, defaulting to "ethereum".
func activeChain(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "ethereum"
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "ethereum"
	}
	return name
}

func runChain(args []string, stdout io.Writer, makeRegistry func() *chain.Registry, customChainsPath string, activeChainPath string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth chain <list|switch|add> [args...]")
	}

	reg := makeRegistry()

	switch args[0] {
	case "list":
		chains := reg.List()
		for _, c := range chains {
			testnet := ""
			if c.Testnet {
				testnet = " (testnet)"
			}
			fmt.Fprintf(stdout, "%-20s  ID: %-8d %s%s\n", c.Name, c.ID, c.NativeCurrency.Symbol, testnet)
		}
		return nil

	case "switch":
		if len(args) < 2 {
			return fmt.Errorf("usage: peth chain switch <name|id>")
		}
		c, err := reg.Get(args[1])
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(activeChainPath), 0700); err != nil {
			return fmt.Errorf("persist active chain: %w", err)
		}
		if err := os.WriteFile(activeChainPath, []byte(c.Name), 0600); err != nil {
			return fmt.Errorf("persist active chain: %w", err)
		}
		fmt.Fprintf(stdout, "Switched to %s (chain ID: %d)\n", c.Name, c.ID)
		return nil

	case "add":
		addFs := flag.NewFlagSet("chain add", flag.ContinueOnError)
		name := addFs.String("name", "", "Chain name")
		rpc := addFs.String("rpc", "", "RPC URL")
		chainID := addFs.Uint64("chain-id", 0, "Chain ID")
		symbol := addFs.String("symbol", "ETH", "Native currency symbol")
		if err := addFs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" || *rpc == "" || *chainID == 0 {
			return fmt.Errorf("usage: peth chain add --name <name> --rpc <url> --chain-id <id>")
		}
		c, err := reg.AddCustom(chain.CustomChainOpts{
			Name:                 *name,
			RPCURL:               *rpc,
			ChainID:              *chainID,
			NativeCurrencySymbol: *symbol,
		})
		if err != nil {
			return err
		}
		if err := reg.SaveCustom(customChainsPath); err != nil {
			return fmt.Errorf("save custom chains: %w", err)
		}
		fmt.Fprintf(stdout, "Added chain %q (ID: %d, RPC: %s)\n", c.Name, c.ID, *rpc)
		return nil

	default:
		return fmt.Errorf("unknown chain command: %s", args[0])
	}
}

// --- Transaction subcommands ---

func runTx(args []string, stdout io.Writer, makeKeystore func() (*wallet.Keystore, error), makeRegistry func() *chain.Registry, activeChainPath string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth tx <send> [flags]")
	}

	switch args[0] {
	case "send":
		sendFs := flag.NewFlagSet("tx send", flag.ContinueOnError)
		to := sendFs.String("to", "", "Recipient address")
		value := sendFs.String("value", "0", "Value in ETH")
		chainName := sendFs.String("chain", activeChain(activeChainPath), "Chain name or ID")
		gasStrategy := sendFs.String("gas-strategy", "normal", "Gas strategy: fast|normal|slow")
		dryRun := sendFs.Bool("dry-run", false, "Simulate without sending")
		if err := sendFs.Parse(args[1:]); err != nil {
			return err
		}
		if *to == "" {
			return fmt.Errorf("usage: peth tx send --to <address> [--value <amount>] [--chain <name>]")
		}
		if err := tx.ValidateAddress(*to); err != nil {
			return err
		}

		ks, err := makeKeystore()
		if err != nil {
			return err
		}
		activeKey, err := ks.Active()
		if err != nil {
			return err
		}

		reg := makeRegistry()
		c, err := reg.Get(*chainName)
		if err != nil {
			return err
		}
		if len(c.RPCURLs) == 0 {
			return fmt.Errorf("no RPC URL configured for chain %s", c.Name)
		}
		rpcURL := c.RPCURLs[0]

		builder := tx.NewBuilder().
			From(activeKey.Address).
			To(*to).
			Value(*value).
			ChainID(c.ID)

		transaction, err := builder.Build()
		if err != nil {
			return err
		}

		if *dryRun {
			sim := tx.NewRPCSimulator()
			result, err := sim.Simulate(rpcURL, transaction)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Simulation: success=%v, gas=%d\n", result.Success, result.GasUsed)
			if result.RevertReason != "" {
				fmt.Fprintf(stdout, "Revert: %s\n", result.RevertReason)
			}
			return nil
		}

		// Estimate gas
		estimator := tx.NewRPCGasEstimator()
		gasEst, err := estimator.Estimate(rpcURL, tx.GasStrategy(*gasStrategy))
		if err != nil {
			return fmt.Errorf("gas estimation: %w", err)
		}
		if gasEst.IsEIP1559 {
			transaction.MaxFeePerGas = gasEst.MaxFeePerGas
			transaction.MaxPriorityFeePerGas = gasEst.MaxPriorityFeePerGas
		} else {
			transaction.GasPrice = gasEst.GasPrice
		}

		signer := tx.NewLocalSigner()
		signed, err := signer.Sign(transaction, activeKey.PrivateKey)
		if err != nil {
			return err
		}

		sender := tx.NewRPCSender()
		result, err := sender.SendRaw(rpcURL, signed.RawTx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Transaction sent: %s\n", result.Hash)
		return nil

	default:
		return fmt.Errorf("unknown tx command: %s", args[0])
	}
}

// --- Token subcommands ---

func runToken(args []string, stdout io.Writer, makeKeystore func() (*wallet.Keystore, error), makeRegistry func() *chain.Registry, activeChainPath string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth token <approve> [flags]")
	}

	switch args[0] {
	case "approve":
		approveFs := flag.NewFlagSet("token approve", flag.ContinueOnError)
		tokenAddr := approveFs.String("token", "", "Token contract address")
		spender := approveFs.String("spender", "", "Spender address")
		amount := approveFs.String("amount", "", "Amount to approve")
		chainName := approveFs.String("chain", activeChain(activeChainPath), "Chain name or ID")
		if err := approveFs.Parse(args[1:]); err != nil {
			return err
		}
		if *tokenAddr == "" || *spender == "" || *amount == "" {
			return fmt.Errorf("usage: peth token approve --token <addr> --spender <addr> --amount <n>")
		}

		ks, err := makeKeystore()
		if err != nil {
			return err
		}
		activeKey, err := ks.Active()
		if err != nil {
			return err
		}

		reg := makeRegistry()
		c, err := reg.Get(*chainName)
		if err != nil {
			return err
		}
		if len(c.RPCURLs) == 0 {
			return fmt.Errorf("no RPC URL configured for chain %s", c.Name)
		}

		erc20 := tx.NewERC20(*tokenAddr, c.ID)
		data, err := erc20.ApproveData(*spender, *amount)
		if err != nil {
			return err
		}

		transaction, err := tx.NewBuilder().
			From(activeKey.Address).
			To(*tokenAddr).
			Data(data).
			ChainID(c.ID).
			Build()
		if err != nil {
			return err
		}

		signer := tx.NewLocalSigner()
		signed, err := signer.Sign(transaction, activeKey.PrivateKey)
		if err != nil {
			return err
		}

		sender := tx.NewRPCSender()
		result, err := sender.SendRaw(c.RPCURLs[0], signed.RawTx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Approval tx sent: %s\n", result.Hash)
		return nil

	default:
		return fmt.Errorf("unknown token command: %s", args[0])
	}
}

// --- Assert subcommands ---

func runAssert(args []string, stdout io.Writer, makeRegistry func() *chain.Registry, activeChainPath string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth assert <balance|tx|chain> [flags]")
	}

	reg := makeRegistry()

	switch args[0] {
	case "balance":
		assertFs := flag.NewFlagSet("assert balance", flag.ContinueOnError)
		gte := assertFs.String("gte", "", "Minimum balance in ETH")
		chainName := assertFs.String("chain", activeChain(activeChainPath), "Chain name or ID")
		if err := assertFs.Parse(args[1:]); err != nil {
			return err
		}
		addr := assertFs.Arg(0)
		if addr == "" || *gte == "" {
			return fmt.Errorf("usage: peth assert balance <address> --gte <amount>")
		}
		c, err := reg.Get(*chainName)
		if err != nil {
			return err
		}
		if len(c.RPCURLs) == 0 {
			return fmt.Errorf("no RPC URL for chain %s", c.Name)
		}
		result, err := tx.AssertBalance(c.RPCURLs[0], addr, "gte", *gte)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", result.Message)
		if !result.Passed {
			return fmt.Errorf("assertion failed")
		}
		return nil

	case "tx":
		assertFs := flag.NewFlagSet("assert tx", flag.ContinueOnError)
		status := assertFs.String("status", "success", "Expected status")
		chainName := assertFs.String("chain", activeChain(activeChainPath), "Chain name or ID")
		if err := assertFs.Parse(args[1:]); err != nil {
			return err
		}
		txHash := assertFs.Arg(0)
		if txHash == "" {
			return fmt.Errorf("usage: peth assert tx <hash> --status <success|failure>")
		}
		c, err := reg.Get(*chainName)
		if err != nil {
			return err
		}
		if len(c.RPCURLs) == 0 {
			return fmt.Errorf("no RPC URL for chain %s", c.Name)
		}
		result, err := tx.AssertTxStatus(c.RPCURLs[0], txHash, *status)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", result.Message)
		if !result.Passed {
			return fmt.Errorf("assertion failed")
		}
		return nil

	case "chain":
		assertFs := flag.NewFlagSet("assert chain", flag.ContinueOnError)
		id := assertFs.Uint64("id", 0, "Expected chain ID")
		chainName := assertFs.String("chain", activeChain(activeChainPath), "Chain to check")
		if err := assertFs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == 0 {
			return fmt.Errorf("usage: peth assert chain --id <chain-id>")
		}
		c, err := reg.Get(*chainName)
		if err != nil {
			return err
		}
		if len(c.RPCURLs) == 0 {
			return fmt.Errorf("no RPC URL for chain %s", c.Name)
		}
		result, err := tx.AssertChainID(c.RPCURLs[0], *id)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", result.Message)
		if !result.Passed {
			return fmt.Errorf("assertion failed")
		}
		return nil

	default:
		return fmt.Errorf("unknown assert command: %s", args[0])
	}
}

// --- dApp subcommands ---

func runDapp(args []string, stdout io.Writer, makeClient func() client.Pinchtab, makeKeystore func() (*wallet.Keystore, error)) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth dapp <connect>")
	}

	switch args[0] {
	case "connect":
		// Parse optional --ref flag.
		fs := flag.NewFlagSet("dapp connect", flag.ContinueOnError)
		buttonRef := fs.String("ref", "", "explicit connect button ref (e.g. --ref e151); skips auto-detection")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		ks, err := makeKeystore()
		if err != nil {
			return err
		}
		activeKey, err := ks.Active()
		if err != nil {
			return err
		}
		connector := dapp.NewConnector(makeClient(), activeKey.Address, activeKey.PrivateKey)
		if err := connector.Connect(*buttonRef); err != nil {
			return err
		}

		// Poll for a pending personal_sign request.
		// SDKs like Privy issue a SIWE challenge immediately after wallet connection.
		// Poll for up to 10 seconds (20 × 500ms).
		sig, err := connector.WaitAndSign(20, 500*time.Millisecond)
		if err != nil {
			return fmt.Errorf("sign challenge: %w", err)
		}
		if sig != "" {
			fmt.Fprintf(stdout, "Signed SIWE challenge\n")
		}

		fmt.Fprintf(stdout, "Connected wallet %s to dApp\n", activeKey.Address)
		return nil

	default:
		return fmt.Errorf("unknown dapp command: %s", args[0])
	}
}

// --- Script runner ---

// scriptWalletAdapter adapts wallet.Keystore to script.ScriptWallet.
type scriptWalletAdapter struct {
	ks *wallet.Keystore
}

func (a *scriptWalletAdapter) Active() (string, error) {
	key, err := a.ks.Active()
	if err != nil {
		return "", err
	}
	return key.Address, nil
}

// scriptChainAdapter adapts chain.Switcher to script.ScriptChain.
type scriptChainAdapter struct {
	switcher *chain.Switcher
}

func (a *scriptChainAdapter) Switch(nameOrID string) error {
	_, err := a.switcher.Switch(nameOrID)
	return err
}

func runScript(args []string, stdout io.Writer, makeClient func() client.Pinchtab, makeKeystore func() (*wallet.Keystore, error), makeRegistry func() *chain.Registry) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth run <script.yaml>")
	}

	browserClient := makeClient()

	ks, err := makeKeystore()
	if err != nil {
		return err
	}

	reg := makeRegistry()
	switcher := chain.NewSwitcher(reg, nil)

	runner := script.NewRunner(browserClient, &scriptWalletAdapter{ks}, &scriptChainAdapter{switcher})
	s, err := runner.LoadFile(args[0])
	if err != nil {
		return err
	}
	if err := runner.Run(s); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Script %q completed successfully\n", s.Name)
	return nil
}

// --- Event listener ---

func runWait(args []string, stdout io.Writer, makeRegistry func() *chain.Registry, activeChainPath string) error {
	waitFs := flag.NewFlagSet("wait", flag.ContinueOnError)
	eventName := waitFs.String("event", "", "Event signature (e.g. Transfer(address,address,uint256))")
	contract := waitFs.String("contract", "", "Contract address")
	timeout := waitFs.Duration("timeout", 30*time.Second, "Timeout duration")
	chainName := waitFs.String("chain", activeChain(activeChainPath), "Chain name or ID")
	if err := waitFs.Parse(args); err != nil {
		return err
	}
	if *eventName == "" || *contract == "" {
		return fmt.Errorf("usage: peth wait --event <signature> --contract <address> [--timeout <duration>]")
	}

	reg := makeRegistry()
	c, err := reg.Get(*chainName)
	if err != nil {
		return err
	}
	if len(c.RPCURLs) == 0 {
		return fmt.Errorf("no RPC URL for chain %s", c.Name)
	}

	listener := event.NewListener(c.RPCURLs[0])
	filter := event.EventFilter{
		ContractAddress: *contract,
		EventSignature:  event.EventSignatureHash(*eventName),
	}
	evt, err := listener.WaitForEvent(filter, *timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Event received in tx %s (block %d)\n", evt.TxHash, evt.BlockNumber)
	return nil
}

// --- Devchain subcommands ---

func runDevchain(args []string, stdout io.Writer, makeRegistry func() *chain.Registry, customChainsPath string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: peth devchain <start|stop|snapshot|revert|fund>")
	}

	switch args[0] {
	case "start":
		startFs := flag.NewFlagSet("devchain start", flag.ContinueOnError)
		tool := startFs.String("tool", "anvil", "Dev chain tool: anvil|hardhat")
		port := startFs.Int("port", 8545, "RPC port")
		chainID := startFs.Uint64("chain-id", 31337, "Chain ID")
		if err := startFs.Parse(args[1:]); err != nil {
			return err
		}
		dc := chain.NewDevChain(chain.DevChainOpts{
			Tool:    *tool,
			Port:    *port,
			ChainID: *chainID,
		})
		if err := dc.Start(); err != nil {
			return err
		}
		// Register in chain registry
		reg := makeRegistry()
		_, _ = reg.AddCustom(chain.CustomChainOpts{
			Name:                 "devchain",
			RPCURL:               dc.RPCURL(),
			ChainID:              *chainID,
			NativeCurrencySymbol: "ETH",
		})
		_ = reg.SaveCustom(customChainsPath)
		fmt.Fprintf(stdout, "Dev chain started (%s) at %s\n", *tool, dc.RPCURL())
		return nil

	case "stop":
		dc := chain.NewDevChain(chain.DevChainOpts{})
		return dc.Stop()

	case "snapshot":
		dc := chain.NewDevChain(chain.DevChainOpts{})
		id, err := dc.Snapshot()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Snapshot: %s\n", id)
		return nil

	case "revert":
		if len(args) < 2 {
			return fmt.Errorf("usage: peth devchain revert <snapshot-id>")
		}
		dc := chain.NewDevChain(chain.DevChainOpts{})
		return dc.Revert(args[1])

	case "fund":
		if len(args) < 3 {
			return fmt.Errorf("usage: peth devchain fund <address> <amount>")
		}
		dc := chain.NewDevChain(chain.DevChainOpts{})
		if err := dc.FundAccount(args[1], args[2]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Funded %s with %s ETH\n", args[1], args[2])
		return nil

	default:
		return fmt.Errorf("unknown devchain command: %s", args[0])
	}
}

// --- Lifecycle commands ---

func pethPIDFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "peth", "pinchtab.pid")
}

func runStart(args []string) error {
	startFs := flag.NewFlagSet("start", flag.ContinueOnError)
	headless := startFs.Bool("headless", false, "Run headless")
	port := startFs.Int("port", 9867, "Pinchtab port")
	profile := startFs.String("profile", "", "Chrome profile path")
	if err := startFs.Parse(args); err != nil {
		return err
	}
	pidFile := pethPIDFile()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}
	mgr := lifecycle.NewManager(&lifecycle.ExecCommander{})
	mgr.PIDFile = pidFile
	return mgr.Start(lifecycle.StartOpts{
		Headless: *headless,
		Port:     *port,
		Profile:  *profile,
		BinPath:  "pinchtab",
	})
}

func runStop() error {
	mgr := lifecycle.NewManager(&lifecycle.ExecCommander{})
	mgr.PIDFile = pethPIDFile()
	return mgr.Stop()
}

func runUpdate(w io.Writer) error {
	u, err := update.NewUpdater()
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "Updating peth...")
	if err := u.Run(); err != nil {
		return update.HintSudo(err)
	}
	fmt.Fprintln(w, "Updated successfully. Run 'peth version' to verify.")
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: peth [flags] <command> [args...]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Browser Commands (pinchtab passthrough):")
	fmt.Fprintln(w, "  nav <url>              Navigate to URL")
	fmt.Fprintln(w, "  snap [-i] [-c] [-d]    Snapshot accessibility tree")
	fmt.Fprintln(w, "  click <ref>            Click element by ref")
	fmt.Fprintln(w, "  type <ref> <text>      Type into element")
	fmt.Fprintln(w, "  press <key>            Press key (Enter, Tab, Escape...)")
	fmt.Fprintln(w, "  fill <ref> <text>      Fill input directly")
	fmt.Fprintln(w, "  hover <ref>            Hover element")
	fmt.Fprintln(w, "  scroll <ref|pixels>    Scroll to element or by pixels")
	fmt.Fprintln(w, "  select <ref> <value>   Select dropdown option")
	fmt.Fprintln(w, "  focus <ref>            Focus element")
	fmt.Fprintln(w, "  text [--raw]           Extract readable text")
	fmt.Fprintln(w, "  tabs [new|close]       Manage tabs")
	fmt.Fprintln(w, "  ss [-o file]           Screenshot")
	fmt.Fprintln(w, "  eval <expression>      Evaluate JavaScript")
	fmt.Fprintln(w, "  pdf [options]          Export page as PDF")
	fmt.Fprintln(w, "  health                 Check server status")
	fmt.Fprintln(w, "  quick <url>            Navigate + analyze page")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Web3 Commands:")
	fmt.Fprintln(w, "  wallet create <name>   Generate new wallet")
	fmt.Fprintln(w, "  wallet import <name>   Import from key or mnemonic")
	fmt.Fprintln(w, "  wallet list            List all wallets")
	fmt.Fprintln(w, "  wallet use <name>      Set active wallet")
	fmt.Fprintln(w, "  chain list             List known chains")
	fmt.Fprintln(w, "  chain switch <name>    Switch active chain")
	fmt.Fprintln(w, "  chain add [flags]      Add custom chain")
	fmt.Fprintln(w, "  tx send [flags]        Send transaction")
	fmt.Fprintln(w, "  token approve [flags]  Approve ERC-20 spending")
	fmt.Fprintln(w, "  dapp connect           Connect wallet to dApp")
	fmt.Fprintln(w, "  run <script.yaml>      Run workflow script")
	fmt.Fprintln(w, "  wait [flags]           Wait for contract event")
	fmt.Fprintln(w, "  assert [type] [flags]  Assert on-chain state")
	fmt.Fprintln(w, "  devchain [cmd]         Manage local dev chain")
	fmt.Fprintln(w, "  start [flags]          Start Pinchtab")
	fmt.Fprintln(w, "  stop                   Stop Pinchtab")
	fmt.Fprintln(w, "  update                 Update peth to the latest version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Other:")
	fmt.Fprintln(w, "  version                Print version")
	fmt.Fprintln(w, "  help                   Show this help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --host     Pinchtab host (default: localhost)")
	fmt.Fprintln(w, "  --port     Pinchtab port (default: 9867)")
	fmt.Fprintln(w, "  --token    Auth token")
}
