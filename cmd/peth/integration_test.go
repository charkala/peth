//go:build integration

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationPassthroughNav(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"nav", "https://example.com"}, &stdout, &stderr, execPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("nav failed: %v", err)
	}
}

func TestIntegrationPassthroughSnap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Navigate first
	_ = run([]string{"nav", "https://example.com"}, &stdout, &stderr, execPassthrough, testConfig(t))
	// Then snap — output goes to pinchtab's stdout, not ours
	err := run([]string{"snap"}, &stdout, &stderr, execPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("snap failed: %v", err)
	}
}

func TestIntegrationPassthroughHealth(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"health"}, &stdout, &stderr, execPassthrough, testConfig(t))
	if err != nil {
		t.Fatalf("health failed: %v", err)
	}
}

func TestIntegrationWalletFlow(t *testing.T) {
	dir := t.TempDir()
	cfg := appConfig{
		walletDir:        filepath.Join(dir, "wallets"),
		customChainsPath: filepath.Join(dir, "chains.json"),
		activeChainPath:  filepath.Join(dir, "active-chain"),
	}
	var stdout, stderr bytes.Buffer

	// Create wallet
	err := run([]string{"wallet", "create", "test-int"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("wallet create: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created wallet") {
		t.Fatalf("expected created wallet output, got %q", stdout.String())
	}

	// Use wallet
	stdout.Reset()
	err = run([]string{"wallet", "use", "test-int"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("wallet use: %v", err)
	}

	// List wallets
	stdout.Reset()
	err = run([]string{"wallet", "list"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("wallet list: %v", err)
	}
	if !strings.Contains(stdout.String(), "* test-int") {
		t.Fatalf("expected active marker on test-int, got %q", stdout.String())
	}

	// Chain list
	stdout.Reset()
	err = run([]string{"chain", "list"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("chain list: %v", err)
	}
	if !strings.Contains(stdout.String(), "ethereum") {
		t.Fatalf("expected ethereum in chain list, got %q", stdout.String())
	}

	// Chain add
	stdout.Reset()
	err = run([]string{"chain", "add", "--name", "localnet", "--rpc", "http://localhost:8545", "--chain-id", "31338"}, &stdout, &stderr, noopPassthrough, cfg)
	if err != nil {
		t.Fatalf("chain add: %v", err)
	}
	if !strings.Contains(stdout.String(), "Added chain") {
		t.Fatalf("expected 'Added chain', got %q", stdout.String())
	}
}
