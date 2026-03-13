package wallet

import (
	"fmt"
	"strings"
)

// Provider generates EIP-1193 compatible JavaScript for injecting
// a mock window.ethereum object into web pages.
type Provider struct {
	ks       *Keystore
	chainID  uint64
	accounts []string
}

// NewProvider creates a Provider backed by the given keystore and chain ID.
func NewProvider(ks *Keystore, chainID uint64) *Provider {
	return &Provider{
		ks:      ks,
		chainID: chainID,
	}
}

// SetChainID updates the chain ID used in generated JavaScript.
func (p *Provider) SetChainID(id uint64) {
	p.chainID = id
}

// SetAccounts overrides the accounts list used in generated JavaScript.
func (p *Provider) SetAccounts(addrs []string) {
	p.accounts = addrs
}

// GenerateJS produces JavaScript that creates a window.ethereum object
// implementing EIP-1193. It reads the active wallet from the keystore
// if no explicit accounts have been set.
func (p *Provider) GenerateJS() (string, error) {
	accounts := p.accounts
	if len(accounts) == 0 {
		active, err := p.ks.Active()
		if err == nil {
			accounts = []string{active.Address}
		}
		// If no active wallet, use empty accounts — not an error
	}

	chainHex := fmt.Sprintf("0x%x", p.chainID)
	selectedAddr := ""
	if len(accounts) > 0 {
		selectedAddr = accounts[0]
	}

	// Build JS accounts array literal
	quotedAccounts := make([]string, len(accounts))
	for i, a := range accounts {
		quotedAccounts[i] = fmt.Sprintf("'%s'", a)
	}
	accountsJS := "[" + strings.Join(quotedAccounts, ",") + "]"

	js := fmt.Sprintf(`window.ethereum = {
  isMetaMask: true,
  chainId: '%s',
  selectedAddress: %s,
  _accounts: %s,
  _events: {},
  on: function(event, cb) {
    if (!this._events[event]) this._events[event] = [];
    this._events[event].push(cb);
  },
  removeListener: function(event, cb) {
    if (!this._events[event]) return;
    this._events[event] = this._events[event].filter(function(f) { return f !== cb; });
  },
  emit: function(event, data) {
    if (!this._events[event]) return;
    this._events[event].forEach(function(cb) { cb(data); });
  },
  _supportedEvents: ['accountsChanged', 'chainChanged', 'connect', 'disconnect'],
  request: function(args) {
    var method = args.method;
    var params = args.params;
    var self = this;
    switch(method) {
      case 'eth_requestAccounts':
        self.emit('connect', {chainId: self.chainId});
        return Promise.resolve(self._accounts);
      case 'eth_accounts':
        return Promise.resolve(self._accounts);
      case 'eth_chainId':
        return Promise.resolve(self.chainId);
      case 'wallet_switchEthereumChain':
        var newChainId = params[0].chainId;
        self.chainId = newChainId;
        self.emit('chainChanged', newChainId);
        return Promise.resolve(null);
      case 'eth_sendTransaction':
        return Promise.resolve('0x' + '0'.repeat(64));
      case 'personal_sign':
        return Promise.resolve('0x' + '0'.repeat(130));
      case 'eth_signTypedData_v4':
        return Promise.resolve('0x' + '0'.repeat(130));
      default:
        return Promise.reject({code: 4200, message: 'Unsupported method: ' + method});
    }
  }
};
window.dispatchEvent(new Event('ethereum#initialized'));`,
		chainHex,
		formatJSString(selectedAddr),
		accountsJS,
	)

	return js, nil
}

// InjectJS returns the full injection script as a string.
// This is a convenience wrapper around GenerateJS that panics on error.
func (p *Provider) InjectJS() string {
	js, err := p.GenerateJS()
	if err != nil {
		// GenerateJS currently never returns an error, but if it did
		// in the future, InjectJS would surface it.
		return ""
	}

	// Wrap in an IIFE to avoid polluting global scope with intermediates
	return "(function() {\n" + js + "\n})();"
}

// formatJSString returns a JS string literal or null.
func formatJSString(s string) string {
	if s == "" {
		return "null"
	}
	return fmt.Sprintf("'%s'", s)
}
