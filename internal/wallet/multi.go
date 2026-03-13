package wallet

import "fmt"

// MultiWallet maps browser tab IDs to wallet names, enabling per-tab
// wallet isolation for multi-account dApp workflows.
type MultiWallet struct {
	ks          *Keystore
	assignments map[string]string // tabID -> walletName
}

// NewMultiWallet creates a MultiWallet backed by the given Keystore.
func NewMultiWallet(ks *Keystore) *MultiWallet {
	return &MultiWallet{
		ks:          ks,
		assignments: make(map[string]string),
	}
}

// Assign associates a wallet name with a tab ID. The wallet must exist
// in the keystore. Reassigning a tab to a different wallet is allowed.
func (mw *MultiWallet) Assign(tabID string, walletName string) error {
	if !mw.ks.exists(walletName) {
		return fmt.Errorf("wallet %q not found in keystore", walletName)
	}
	mw.assignments[tabID] = walletName
	return nil
}

// GetForTab returns the wallet key assigned to the given tab ID.
func (mw *MultiWallet) GetForTab(tabID string) (*Key, error) {
	name, ok := mw.assignments[tabID]
	if !ok {
		return nil, fmt.Errorf("no wallet assigned to tab %q", tabID)
	}
	return mw.ks.Get(name)
}

// Unassign removes the wallet assignment for a tab.
func (mw *MultiWallet) Unassign(tabID string) {
	delete(mw.assignments, tabID)
}

// ListAssignments returns a copy of the current tab-to-wallet mappings.
func (mw *MultiWallet) ListAssignments() map[string]string {
	result := make(map[string]string, len(mw.assignments))
	for k, v := range mw.assignments {
		result[k] = v
	}
	return result
}
