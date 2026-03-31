// Package dapp provides dApp wallet connection and interaction automation.
package dapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/charkala/peth/internal/wallet"
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
// Accounts and signing are controlled by peth via __pethSignMessage.
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
        case 'personal_sign': {
          // Message is args.params[0], address is args.params[1]
          var message = args.params && args.params[0] ? args.params[0] : '';
          return window.__pethSignMessage(message);
        }
        case 'eth_sign': {
          // eth_sign: params[0] = address, params[1] = message hash
          var message = args.params && args.params[1] ? args.params[1] : '';
          return window.__pethSignMessage(message);
        }
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
  return "ok";
})();
`

// Connector automates dApp wallet connection flows.
type Connector struct {
	client     BrowserClient
	walletAddr string
	privKey    []byte
	injected   bool
}

// NewConnector creates a new Connector with the given browser client and wallet.
func NewConnector(client BrowserClient, walletAddr string, privKey []byte) *Connector {
	return &Connector{
		client:     client,
		walletAddr: walletAddr,
		privKey:    privKey,
	}
}

// Connect automates the standard connect-wallet flow:
// 1. Injects the EIP-1193 provider with real secp256k1 signing
// 2. Sets the wallet address as the connected account
// 3. Finds and clicks the connect button (or uses the provided ref)
//
// Pass buttonRef="" to auto-detect; pass a specific ref (e.g. "e151") to skip detection.
func (c *Connector) Connect(buttonRef string) error {
	// Check if already connected.
	connected, err := c.IsConnected()
	if err != nil {
		return fmt.Errorf("check connection: %w", err)
	}
	if connected {
		return nil
	}

	// Inject provider first.
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

	// Resolve button ref.
	ref := buttonRef
	if ref == "" {
		snapshot, err := c.client.Snap()
		if err != nil {
			return fmt.Errorf("snap page: %w", err)
		}
		ref = c.findConnectButton(snapshot)
		if ref == "" {
			return fmt.Errorf("no connect-wallet button found in page snapshot (use --ref to specify)")
		}
	}

	// Click the connect button.
	if err := c.client.Click(ref); err != nil {
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

// SignInWithEthereum signs a SIWE (EIP-4361) message using real secp256k1 ECDSA.
func (c *Connector) SignInWithEthereum(message string) (string, error) {
	if len(c.privKey) == 0 {
		return "", fmt.Errorf("no private key available for signing")
	}
	return wallet.PersonalSign(c.privKey, message)
}

// injectProvider injects the EIP-1193 provider JS and wires up the signing bridge.
func (c *Connector) injectProvider() error {
	if c.injected {
		return nil
	}

	// Inject the base provider.
	if _, err := c.client.Eval(providerJS); err != nil {
		return fmt.Errorf("inject provider JS: %w", err)
	}

	// Wire up the signing bridge: __pethSignMessage is called by the provider's
	// personal_sign / eth_sign handler and resolved via a Go-signed result.
	// We install a function that posts to a peth-controlled endpoint.
	// For now, we pre-sign a pending message by polling __pethPendingSign.
	signingBridgeJS := `
(function() {
  window.__pethSignMessage = function(message) {
    return new Promise(function(resolve, reject) {
      window.__pethPendingSign = { message: message, resolve: resolve, reject: reject };
    });
  };
  return "ok";
})()`
	if _, err := c.client.Eval(signingBridgeJS); err != nil {
		return fmt.Errorf("inject signing bridge: %w", err)
	}

	c.injected = true
	return nil
}

// ResolvePendingSign checks if there is a pending personal_sign request from the dApp,
// signs it with the real secp256k1 key, and resolves the promise.
// Call this in a polling loop after triggering wallet connection.
func (c *Connector) ResolvePendingSign() (bool, error) {
	if len(c.privKey) == 0 {
		return false, fmt.Errorf("no private key for signing")
	}

	// Check for a pending sign request.
	result, err := c.client.Eval(
		`window.__pethPendingSign ? window.__pethPendingSign.message : null`,
	)
	if err != nil {
		return false, err
	}
	if result == "" || result == "null" || result == "undefined" {
		return false, nil // no pending sign
	}

	// Decode hex-encoded message (Privy sends messages as 0x-prefixed hex).
	message := result
	if len(result) > 2 && result[:2] == "0x" {
		b, err := hexDecode(result[2:])
		if err == nil {
			message = string(b)
		}
	}

	// Sign the message with real secp256k1.
	sig, err := wallet.PersonalSign(c.privKey, message)
	if err != nil {
		return false, fmt.Errorf("sign message: %w", err)
	}

	// Resolve the pending promise with the real signature.
	resolveJS := fmt.Sprintf(`
		(function() {
			var pending = window.__pethPendingSign;
			window.__pethPendingSign = null;
			if (pending && pending.resolve) {
				pending.resolve(%q);
				return "resolved";
			}
			return "no pending";
		})()
	`, sig)

	if _, err := c.client.Eval(resolveJS); err != nil {
		return false, fmt.Errorf("resolve sign promise: %w", err)
	}

	return true, nil
}

// WaitAndSign polls for a pending personal_sign request and resolves it.
// It retries up to maxAttempts times with the given interval between checks.
// Returns the signature if resolved, empty string if nothing pending within timeout.
func (c *Connector) WaitAndSign(maxAttempts int, interval time.Duration) (string, error) {
	if len(c.privKey) == 0 {
		return "", fmt.Errorf("no private key for signing")
	}
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(interval)
		resolved, err := c.ResolvePendingSign()
		if err != nil {
			return "", err
		}
		if resolved {
			return "signed", nil
		}
	}
	return "", nil
}

// hexDecode decodes a hex string to bytes.
func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		s = "0" + s
	}
	b := make([]byte, len(s)/2)
	for i := range b {
		hi, err := hexNibble(s[2*i])
		if err != nil {
			return nil, err
		}
		lo, err := hexNibble(s[2*i+1])
		if err != nil {
			return nil, err
		}
		b[i] = hi<<4 | lo
	}
	return b, nil
}

func hexNibble(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	}
	return 0, fmt.Errorf("invalid hex char: %c", c)
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
