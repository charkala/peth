package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charkala/peth/internal/dapp"
	"github.com/charkala/peth/internal/wallet"
)

// cdpPort is the default Chrome DevTools Protocol port used by Pinchtab.
// Pinchtab starts Chrome on port 9868, which forwards CDP on 9869.
const defaultCDPPort = 9869

// runNavWithPreload intercepts "peth nav <url>" when a wallet is active.
// It:
//  1. Resolves the active wallet address.
//  2. Registers Page.addScriptToEvaluateOnNewDocument on the active CDP target
//     to inject the EIP-1193 provider before any page JS runs.
//  3. Delegates the actual navigation to pinchtab via passthrough.
//
// This ensures wallet-detection SDKs (Privy, Dynamic, Web3Auth, etc.) see
// window.ethereum during their initialization phase.
func runNavWithPreload(args []string, passthrough passthroughFunc, cfg appConfig) error {
	if len(args) == 0 {
		return fmt.Errorf("nav requires a URL")
	}

	// Parse nav flags to extract the URL (first non-flag arg).
	fs := flag.NewFlagSet("nav", flag.ContinueOnError)
	newTab := fs.Bool("new-tab", false, "")
	blockAds := fs.Bool("block-ads", false, "")
	blockImages := fs.Bool("block-images", false, "")
	tabFlag := fs.String("tab", "", "")
	fs.Parse(args)

	url := ""
	if remaining := fs.Args(); len(remaining) > 0 {
		url = remaining[0]
	}
	if url == "" {
		return fmt.Errorf("nav: no URL provided")
	}
	_ = newTab
	_ = blockAds
	_ = blockImages
	_ = tabFlag

	// Load active wallet — if none set, skip preload (passthrough handles it).
	ks, err := wallet.NewKeystore(cfg.walletDir)
	if err != nil {
		return fmt.Errorf("open keystore: %w", err)
	}
	activeKey, err := ks.Active()
	if err != nil {
		// No active wallet — skip preload injection.
		return fmt.Errorf("no active wallet")
	}

	// Find the CDP port. Pinchtab uses 9869 by default.
	cdpPort := defaultCDPPort
	if !isCDPAvailable(cdpPort) {
		// Try fallback ports.
		for _, p := range []int{9870, 9871, 9872} {
			if isCDPAvailable(p) {
				cdpPort = p
				break
			}
		}
		if !isCDPAvailable(cdpPort) {
			return fmt.Errorf("CDP not available on port %d", cdpPort)
		}
	}

	injector := dapp.NewPreloadInjector(cdpPort, activeKey.Address)

	// Find the active target (most recently loaded xmarket/page target).
	// We register the preload on all existing page targets so the next
	// navigation (triggered by passthrough) picks it up.
	targets, err := injector.ListPageTargets()
	if err != nil {
		return fmt.Errorf("list targets: %w", err)
	}

	for _, targetID := range targets {
		// Register preload — non-fatal if individual target fails.
		injector.RegisterPreload(targetID)
	}

	// Now run the actual nav via pinchtab.
	if err := passthrough("nav", args); err != nil {
		return err
	}

	// After nav, the new page has loaded with the preload script.
	// Wait briefly for the page to settle, then verify injection.
	time.Sleep(500 * time.Millisecond)
	return nil
}

// isCDPAvailable checks if CDP is listening on the given port.
func isCDPAvailable(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// targetIDFromWS extracts the target ID from a WebSocket debugger URL.
func targetIDFromWS(wsURL string) string {
	parts := strings.Split(wsURL, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
