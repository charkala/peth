package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockPassthrough records the command and args it was called with.
type mockPassthrough struct {
	calledCmd  string
	calledArgs []string
	err        error
}

func (m *mockPassthrough) fn(command string, args []string) error {
	m.calledCmd = command
	m.calledArgs = args
	return m.err
}

func noopPassthrough(_ string, _ []string) error { return nil }

func testConfig(t *testing.T) appConfig {
	t.Helper()
	dir := t.TempDir()
	return appConfig{
		walletDir:        filepath.Join(dir, "wallets"),
		customChainsPath: filepath.Join(dir, "chains.json"),
		activeChainPath:  filepath.Join(dir, "active-chain"),
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantOut    string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "version command",
			args:    []string{"version"},
			wantOut: "peth version dev",
			wantErr: false,
		},
		{
			name:       "unknown command",
			args:       []string{"bogus"},
			wantErr:    true,
			wantErrMsg: "unknown command",
		},
		{
			name:    "no args shows usage",
			args:    []string{},
			wantOut: "Usage: peth",
			wantErr: false,
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantOut: "Usage: peth",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(tt.args, &stdout, &stderr, noopPassthrough, testConfig(t))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantOut != "" && !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantOut)
			}
		})
	}
}

func TestGlobalFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--port", "1234", "version"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "peth version") {
		t.Errorf("stdout = %q, want version output", stdout.String())
	}
}

func TestPassthroughCommands(t *testing.T) {
	commands := []struct {
		name string
		args []string
		want string
	}{
		{"nav", []string{"nav", "https://example.com"}, "nav"},
		{"snap", []string{"snap"}, "snap"},
		{"snap with flags", []string{"snap", "-i", "-c"}, "snap"},
		{"click", []string{"click", "e5"}, "click"},
		{"type", []string{"type", "e12", "hello"}, "type"},
		{"press", []string{"press", "Enter"}, "press"},
		{"fill", []string{"fill", "e3", "text"}, "fill"},
		{"hover", []string{"hover", "e8"}, "hover"},
		{"scroll", []string{"scroll", "e20"}, "scroll"},
		{"select", []string{"select", "e10", "option2"}, "select"},
		{"focus", []string{"focus", "e3"}, "focus"},
		{"text", []string{"text"}, "text"},
		{"tabs", []string{"tabs"}, "tabs"},
		{"ss", []string{"ss", "-o", "page.jpg"}, "ss"},
		{"eval", []string{"eval", "document.title"}, "eval"},
		{"pdf", []string{"pdf", "--tab", "TAB1"}, "pdf"},
		{"health", []string{"health"}, "health"},
		{"quick", []string{"quick", "https://example.com"}, "quick"},
	}

	for _, tt := range commands {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPassthrough{}
			var stdout, stderr bytes.Buffer
			err := run(tt.args, &stdout, &stderr, mock.fn, testConfig(t))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mock.calledCmd != tt.want {
				t.Errorf("passthrough command = %q, want %q", mock.calledCmd, tt.want)
			}
		})
	}
}

func TestPassthroughForwardsArgs(t *testing.T) {
	mock := &mockPassthrough{}
	var stdout, stderr bytes.Buffer
	err := run([]string{"nav", "https://example.com"}, &stdout, &stderr, mock.fn, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calledCmd != "nav" {
		t.Errorf("command = %q, want %q", mock.calledCmd, "nav")
	}
	if len(mock.calledArgs) != 1 || mock.calledArgs[0] != "https://example.com" {
		t.Errorf("args = %v, want [https://example.com]", mock.calledArgs)
	}
}

func TestPassthroughError(t *testing.T) {
	mock := &mockPassthrough{err: fmt.Errorf("pinchtab nav exited with code 1")}
	var stdout, stderr bytes.Buffer
	err := run([]string{"nav", "https://example.com"}, &stdout, &stderr, mock.fn, testConfig(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exited with code 1") {
		t.Errorf("error = %q, want substring %q", err.Error(), "exited with code 1")
	}
}

func TestUsageShowsBrowserAndWeb3(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()

	for _, section := range []string{"Browser Commands", "Web3 Commands"} {
		if !strings.Contains(out, section) {
			t.Errorf("usage missing section %q", section)
		}
	}
	for _, cmd := range []string{"nav", "snap", "click", "wallet", "chain", "tx send", "dapp connect"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("usage missing command %q", cmd)
		}
	}
}

// --- Wallet command tests ---

func TestWalletCreate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet", "create", "test-wallet"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created wallet") {
		t.Errorf("stdout = %q, want 'Created wallet'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "0x") {
		t.Errorf("stdout = %q, want address starting with 0x", stdout.String())
	}
}

func TestWalletCreateDuplicate(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	_ = run([]string{"wallet", "create", "dupe"}, &stdout, &stderr, noopPassthrough, cfg)
	err := run([]string{"wallet", "create", "dupe"}, &stdout, &stderr, noopPassthrough, cfg)
	if err == nil {
		t.Fatal("expected error for duplicate wallet")
	}
}

func TestWalletImportHex(t *testing.T) {
	var stdout, stderr bytes.Buffer
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := run([]string{"wallet", "import", "imported", hexKey}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Imported wallet") {
		t.Errorf("stdout = %q, want 'Imported wallet'", stdout.String())
	}
}

func TestWalletImportMnemonic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	args := append([]string{"wallet", "import", "mnemonic-wallet"}, strings.Fields(mnemonic)...)
	err := run(args, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Imported wallet") {
		t.Errorf("stdout = %q, want 'Imported wallet'", stdout.String())
	}
}

func TestWalletList(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	_ = run([]string{"wallet", "create", "w1"}, &stdout, &stderr, noopPassthrough, cfg)
	_ = run([]string{"wallet", "create", "w2"}, &stdout, &stderr, noopPassthrough, cfg)

	stdout.Reset()
	err := run([]string{"wallet", "list"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "w1") || !strings.Contains(out, "w2") {
		t.Errorf("wallet list missing wallets: %q", out)
	}
}

func TestWalletListEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet", "list"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No wallets found") {
		t.Errorf("stdout = %q, want 'No wallets found'", stdout.String())
	}
}

func TestWalletUse(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	_ = run([]string{"wallet", "create", "primary"}, &stdout, &stderr, noopPassthrough, cfg)

	stdout.Reset()
	err := run([]string{"wallet", "use", "primary"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Active wallet set to") {
		t.Errorf("stdout = %q, want 'Active wallet set to'", stdout.String())
	}
}

func TestWalletUseNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet", "use", "nonexistent"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for nonexistent wallet")
	}
}

func TestWalletListShowsActive(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	_ = run([]string{"wallet", "create", "w1"}, &stdout, &stderr, noopPassthrough, cfg)
	_ = run([]string{"wallet", "create", "w2"}, &stdout, &stderr, noopPassthrough, cfg)
	_ = run([]string{"wallet", "use", "w1"}, &stdout, &stderr, noopPassthrough, cfg)

	stdout.Reset()
	_ = run([]string{"wallet", "list"}, &stdout, &stderr, noopPassthrough, cfg)
	out := stdout.String()
	// Active wallet should have * marker
	if !strings.Contains(out, "* w1") {
		t.Errorf("wallet list should show active marker: %q", out)
	}
}

func TestWalletMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing wallet subcommand")
	}
}

func TestWalletCreateMissingName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet", "create"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing wallet name")
	}
}

func TestWalletImportMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wallet", "import"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing import args")
	}
	err = run([]string{"wallet", "import", "name"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing import key")
	}
}

// --- Chain command tests ---

func TestChainList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "list"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "ethereum") {
		t.Errorf("chain list missing ethereum: %q", out)
	}
}

func TestChainSwitch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "switch", "optimism"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Switched to") {
		t.Errorf("stdout = %q, want 'Switched to'", stdout.String())
	}
}

func TestChainSwitchPersists(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "switch", "optimism"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify active chain was persisted
	data, err := os.ReadFile(cfg.activeChainPath)
	if err != nil {
		t.Fatalf("failed to read active chain file: %v", err)
	}
	if string(data) != "optimism" {
		t.Errorf("active chain = %q, want %q", string(data), "optimism")
	}
}

func TestActiveChainDefault(t *testing.T) {
	// Non-existent path should return "ethereum"
	got := activeChain("/nonexistent/path")
	if got != "ethereum" {
		t.Errorf("activeChain(nonexistent) = %q, want %q", got, "ethereum")
	}

	// Written value should be returned
	dir := t.TempDir()
	path := filepath.Join(dir, "active-chain")
	if err := os.WriteFile(path, []byte("polygon"), 0600); err != nil {
		t.Fatal(err)
	}
	got = activeChain(path)
	if got != "polygon" {
		t.Errorf("activeChain(polygon) = %q, want %q", got, "polygon")
	}

	// Empty file should return "ethereum"
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	got = activeChain(path)
	if got != "ethereum" {
		t.Errorf("activeChain(empty) = %q, want %q", got, "ethereum")
	}
}

func TestChainSwitchNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "switch", "nonexistent"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for nonexistent chain")
	}
}

func TestChainAdd(t *testing.T) {
	cfg := testConfig(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "add", "--name", "TestNet", "--rpc", "http://localhost:8545", "--chain-id", "99999"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Added chain") {
		t.Errorf("stdout = %q, want 'Added chain'", stdout.String())
	}
	// Verify chains.json was created
	if _, err := os.Stat(cfg.customChainsPath); os.IsNotExist(err) {
		t.Error("custom chains file was not created")
	}
}

func TestChainAddMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain", "add", "--name", "X"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing chain add flags")
	}
}

func TestChainMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"chain"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing chain subcommand")
	}
}

// --- Tx command tests ---

func TestTxSendMissingTo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"tx", "send"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing --to")
	}
}

func TestTxMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"tx"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing tx subcommand")
	}
}

// --- Token command tests ---

func TestTokenApproveMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"token", "approve"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing token approve flags")
	}
}

func TestTokenMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"token"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing token subcommand")
	}
}

// --- Assert command tests ---

func TestAssertMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"assert"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing assert subcommand")
	}
}

func TestAssertBalanceMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"assert", "balance"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing assert balance args")
	}
}

func TestAssertTxMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"assert", "tx"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing assert tx args")
	}
}

func TestAssertChainMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"assert", "chain"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing assert chain --id")
	}
}

// --- dApp command tests ---

func TestDappMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"dapp"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing dapp subcommand")
	}
}

// --- Script runner tests ---

func TestRunScriptMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"run"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing script file")
	}
}

// --- Wait command tests ---

func TestWaitMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"wait"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing wait args")
	}
}

// --- Devchain command tests ---

func TestDevchainMissingSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"devchain"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing devchain subcommand")
	}
}

func TestDevchainRevertMissingID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"devchain", "revert"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing snapshot ID")
	}
}

func TestDevchainFundMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"devchain", "fund"}, &stdout, &stderr, noopPassthrough, testConfig(t))
	if err == nil {
		t.Fatal("expected error for missing fund args")
	}
}
