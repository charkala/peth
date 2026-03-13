// Package dapp provides dApp wallet connection and interaction automation.
package dapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// BrowserClient defines the browser automation methods needed by the connector.
type BrowserClient interface {
	Nav(url string) error
	Snap() (string, error)
	Click(ref string) error
	Fill(ref, text string) error
	Eval(js string) (string, error)
}

// connectButtonRefs lists common connect-wallet button references to search for.
var connectButtonRefs = []string{
	"connectWallet",
	"connect-wallet",
	"btnConnect",
	"connect_wallet",
	"walletConnect",
}

// providerJS is the JavaScript injected to create a mock EIP-1193 provider.
const providerJS = `
(function() {
  if (window.__pethProvider) return;
  const accounts = [];
  const provider = {
    isMetaMask: true,
    selectedAddress: null,
    chainId: '0x1',
    networkVersion: '1',
    _events: {},
    request: function(args) {
      switch(args.method) {
        case 'eth_requestAccounts':
        case 'eth_accounts':
          return Promise.resolve(accounts);
        case 'eth_chainId':
          return Promise.resolve(provider.chainId);
        case 'net_version':
          return Promise.resolve(provider.networkVersion);
        case 'personal_sign':
          if (window.__pethSignResult) {
            var result = window.__pethSignResult;
            window.__pethSignResult = null;
            return Promise.resolve(result);
          }
          return Promise.reject(new Error('signing not available'));
        default:
          return Promise.reject(new Error('unsupported method: ' + args.method));
      }
    },
    on: function(event, cb) {
      if (!provider._events[event]) provider._events[event] = [];
      provider._events[event].push(cb);
      return provider;
    },
    removeListener: function(event, cb) {
      if (provider._events[event]) {
        provider._events[event] = provider._events[event].filter(function(f) { return f !== cb; });
      }
      return provider;
    },
    emit: function(event, data) {
      if (provider._events[event]) {
        provider._events[event].forEach(function(cb) { cb(data); });
      }
    },
    _setAccounts: function(addrs) {
      accounts.length = 0;
      addrs.forEach(function(a) { accounts.push(a); });
      provider.selectedAddress = accounts[0] || null;
      provider.emit('accountsChanged', accounts);
    },
    _setChainId: function(id) {
      provider.chainId = id;
      provider.networkVersion = String(parseInt(id, 16));
      provider.emit('chainChanged', id);
    }
  };
  window.ethereum = provider;
  window.__pethProvider = provider;
})();
`

// Connector automates dApp wallet connection flows.
type Connector struct {
	client     BrowserClient
	walletAddr string
	injected   bool
}

// NewConnector creates a new Connector with the given browser client and wallet address.
func NewConnector(client BrowserClient, walletAddr string) *Connector {
	return &Connector{
		client:     client,
		walletAddr: walletAddr,
	}
}

// Connect automates the standard connect-wallet flow:
// 1. Checks if already connected (no-op if so)
// 2. Snaps the page to find a connect button
// 3. Injects the provider JS
// 4. Sets the wallet address as the connected account
// 5. Clicks the connect button
func (c *Connector) Connect() error {
	// Check if already connected.
	connected, err := c.IsConnected()
	if err != nil {
		return fmt.Errorf("check connection: %w", err)
	}
	if connected {
		return nil
	}

	// Snap the page to find a connect button.
	snapshot, err := c.client.Snap()
	if err != nil {
		return fmt.Errorf("snap page: %w", err)
	}

	// Find the connect button ref in the snapshot.
	buttonRef := c.findConnectButton(snapshot)
	if buttonRef == "" {
		return fmt.Errorf("no connect-wallet button found in page snapshot")
	}

	// Inject provider if not already done.
	if err := c.injectProvider(); err != nil {
		return fmt.Errorf("inject provider: %w", err)
	}

	// Set accounts so eth_requestAccounts returns the wallet address.
	setAccountsJS := fmt.Sprintf(
		`window.__pethProvider._setAccounts([%q])`,
		c.walletAddr,
	)
	if _, err := c.client.Eval(setAccountsJS); err != nil {
		return fmt.Errorf("set accounts: %w", err)
	}

	// Click the connect button.
	if err := c.client.Click(buttonRef); err != nil {
		return fmt.Errorf("click connect: %w", err)
	}

	return nil
}

// IsConnected checks if the wallet is connected by evaluating JS.
func (c *Connector) IsConnected() (bool, error) {
	result, err := c.client.Eval(
		`(window.ethereum && window.ethereum.selectedAddress) ? window.ethereum.selectedAddress : ''`,
	)
	if err != nil {
		return false, err
	}
	return result != "" && result != "null" && result != "undefined", nil
}

// Disconnect clears the provider state.
func (c *Connector) Disconnect() error {
	_, err := c.client.Eval(`
		if (window.__pethProvider) {
			window.__pethProvider._setAccounts([]);
			window.__pethProvider.selectedAddress = null;
		}
	`)
	return err
}

// SignInWithEthereum signs a SIWE (EIP-4361) message using the wallet's private key.
// It uses SHA-256 as a placeholder for keccak256 (matching the wallet package convention).
func (c *Connector) SignInWithEthereum(message string) (string, error) {
	if c.walletAddr == "" {
		return "", fmt.Errorf("no wallet address configured")
	}

	// Deterministic placeholder signature using HMAC-SHA256.
	// TODO: Replace with proper secp256k1 ECDSA signature when go-ethereum is available.
	addrBytes, err := hex.DecodeString(strings.TrimPrefix(c.walletAddr, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode wallet address: %w", err)
	}

	// HMAC(key=address, message) produces a deterministic 32-byte value.
	// We use two rounds to fill r (32 bytes) and s (32 bytes).
	mac := hmac.New(sha256.New, addrBytes)
	mac.Write([]byte(message))
	rBytes := mac.Sum(nil)

	mac.Reset()
	mac.Write(rBytes)
	sBytes := mac.Sum(nil)

	// Encode as 0x-prefixed hex: r (32 bytes) || s (32 bytes) || v (1 byte)
	sig := make([]byte, 65)
	copy(sig[0:32], rBytes)
	copy(sig[32:64], sBytes)
	sig[64] = 27 // v = 27 (uncompressed, no chain replay protection)

	return "0x" + hex.EncodeToString(sig), nil
}

// injectProvider injects the EIP-1193 mock provider JS into the page.
func (c *Connector) injectProvider() error {
	if c.injected {
		return nil
	}
	_, err := c.client.Eval(providerJS)
	if err != nil {
		return err
	}
	c.injected = true
	return nil
}


// findConnectButton looks for known connect-wallet button refs in the snapshot text.
func (c *Connector) findConnectButton(snapshot string) string {
	lower := strings.ToLower(snapshot)
	for _, ref := range connectButtonRefs {
		if strings.Contains(lower, strings.ToLower(ref)) {
			return ref
		}
	}
	return ""
}
