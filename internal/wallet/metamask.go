package wallet

import "fmt"

// PinchtabClient defines the browser automation methods needed by MetaMaskAutomator.
type PinchtabClient interface {
	Nav(url string) error
	Click(ref string) error
	Fill(ref, text string) error
	Snap() (string, error)
}

// MetaMask extension ID (stable across installations for the Chrome Web Store version).
const metaMaskExtensionID = "nkbihfbeogaeaoehlefnkodbefgpgknn"

// MetaMask onboarding accessibility refs (these map to MetaMask's UI elements).
const (
	refAgreeTerms      = "agree-terms"
	refImportWallet    = "import-wallet"
	refNoThanks        = "no-thanks"
	refSeedPhraseInput = "import-srp__srp-word-0"
	refPasswordInput   = "create-password-new"
	refConfirmPassword = "create-password-confirm"
	refPasswordTerms   = "create-password-terms"
	refCreatePassword  = "create-password-import"
	refDoneButton      = "onboarding-complete-done"
	refNextButton      = "pin-extension-next"
	refPinDone         = "pin-extension-done"

	refConnectApprove = "page-container-footer-next"
	refTxConfirm      = "page-container-footer-next"

	refNetworkButton  = "network-display"
)

// defaultMetaMaskPassword is used during automated onboarding.
// This password protects the local MetaMask keystore only during automation.
const defaultMetaMaskPassword = "peth-automation-12345!"

// MetaMaskAutomator drives MetaMask UI through a Pinchtab browser client.
type MetaMaskAutomator struct {
	client PinchtabClient
}

// NewMetaMaskAutomator creates a MetaMaskAutomator with the given client.
func NewMetaMaskAutomator(client PinchtabClient) *MetaMaskAutomator {
	return &MetaMaskAutomator{client: client}
}

// Setup automates MetaMask onboarding: imports a seed phrase and sets a password.
func (m *MetaMaskAutomator) Setup(seedPhrase string) error {
	// Navigate to MetaMask onboarding page
	onboardingURL := fmt.Sprintf("chrome-extension://%s/home.html#onboarding/welcome", metaMaskExtensionID)
	if err := m.client.Nav(onboardingURL); err != nil {
		return fmt.Errorf("metamask setup: navigate to onboarding: %w", err)
	}

	// Accept terms
	if err := m.client.Click(refAgreeTerms); err != nil {
		return fmt.Errorf("metamask setup: agree terms: %w", err)
	}

	// Choose import wallet
	if err := m.client.Click(refImportWallet); err != nil {
		return fmt.Errorf("metamask setup: click import: %w", err)
	}

	// Decline analytics
	if err := m.client.Click(refNoThanks); err != nil {
		return fmt.Errorf("metamask setup: decline analytics: %w", err)
	}

	// Fill seed phrase
	if err := m.client.Fill(refSeedPhraseInput, seedPhrase); err != nil {
		return fmt.Errorf("metamask setup: fill seed phrase: %w", err)
	}

	// Set password
	if err := m.client.Fill(refPasswordInput, defaultMetaMaskPassword); err != nil {
		return fmt.Errorf("metamask setup: fill password: %w", err)
	}

	// Confirm password
	if err := m.client.Fill(refConfirmPassword, defaultMetaMaskPassword); err != nil {
		return fmt.Errorf("metamask setup: confirm password: %w", err)
	}

	// Accept password terms
	if err := m.client.Click(refPasswordTerms); err != nil {
		return fmt.Errorf("metamask setup: accept password terms: %w", err)
	}

	// Submit import
	if err := m.client.Click(refCreatePassword); err != nil {
		return fmt.Errorf("metamask setup: submit import: %w", err)
	}

	// Complete onboarding
	if err := m.client.Click(refDoneButton); err != nil {
		return fmt.Errorf("metamask setup: click done: %w", err)
	}

	// Handle pin extension prompt
	if err := m.client.Click(refNextButton); err != nil {
		return fmt.Errorf("metamask setup: click next: %w", err)
	}

	if err := m.client.Click(refPinDone); err != nil {
		return fmt.Errorf("metamask setup: click pin done: %w", err)
	}

	return nil
}

// ApproveConnection clicks the approve button on a MetaMask connection popup.
func (m *MetaMaskAutomator) ApproveConnection() error {
	if err := m.client.Click(refConnectApprove); err != nil {
		return fmt.Errorf("metamask approve connection: %w", err)
	}
	return nil
}

// ApproveTransaction clicks the confirm button on a MetaMask transaction popup.
func (m *MetaMaskAutomator) ApproveTransaction() error {
	if err := m.client.Click(refTxConfirm); err != nil {
		return fmt.Errorf("metamask approve transaction: %w", err)
	}
	return nil
}

// SwitchNetwork switches the active network in MetaMask to the given chain ID.
func (m *MetaMaskAutomator) SwitchNetwork(chainID uint64) error {
	// Click network selector
	if err := m.client.Click(refNetworkButton); err != nil {
		return fmt.Errorf("metamask switch network: open network selector: %w", err)
	}

	// Click the target network by chain ID ref
	networkRef := fmt.Sprintf("network-%d", chainID)
	if err := m.client.Click(networkRef); err != nil {
		return fmt.Errorf("metamask switch network: select chain %d: %w", chainID, err)
	}

	return nil
}
