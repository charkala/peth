package wallet

import "fmt"

// SolanaProvider generates Phantom-compatible window.solana JavaScript
// for injecting into web pages.
type SolanaProvider struct {
	ks         *SolanaKeystore
	walletName string
}

// NewSolanaProvider creates a SolanaProvider for the named wallet.
func NewSolanaProvider(ks *SolanaKeystore, walletName string) *SolanaProvider {
	return &SolanaProvider{
		ks:         ks,
		walletName: walletName,
	}
}

// GenerateJS produces JavaScript that creates a window.solana object
// implementing a Phantom-compatible API.
func (sp *SolanaProvider) GenerateJS() string {
	key, err := sp.ks.Get(sp.walletName)
	if err != nil {
		// Return a minimal stub if wallet not found
		return `window.solana = { isPhantom: true, connect: function() { return Promise.reject('wallet not found'); } };`
	}

	return fmt.Sprintf(`window.solana = {
  isPhantom: true,
  isConnected: false,
  publicKey: {
    _base58: '%s',
    toBase58: function() { return this._base58; },
    toString: function() { return this._base58; },
    toBytes: function() { return new Uint8Array([]); }
  },
  connect: function(opts) {
    this.isConnected = true;
    return Promise.resolve({ publicKey: this.publicKey });
  },
  disconnect: function() {
    this.isConnected = false;
    return Promise.resolve();
  },
  signTransaction: function(tx) {
    return Promise.resolve(tx);
  },
  signAllTransactions: function(txs) {
    return Promise.resolve(txs);
  },
  signMessage: function(message, display) {
    var sig = new Uint8Array(64);
    return Promise.resolve({ signature: sig, publicKey: this.publicKey });
  },
  on: function(event, cb) {},
  off: function(event, cb) {},
  request: function(args) {
    if (args.method === 'connect') return this.connect();
    if (args.method === 'disconnect') return this.disconnect();
    return Promise.reject({ code: 4200, message: 'Unsupported method: ' + args.method });
  }
};
window.dispatchEvent(new Event('solana#initialized'));`, key.Address)
}
