package chain

import "fmt"

// ProviderUpdater is called when the active chain changes.
// It will be implemented by wallet.Provider to update the injected chain ID.
type ProviderUpdater interface {
	SetChainID(id uint64)
}

// Switcher manages the currently active chain and notifies a provider
// updater when the chain changes.
type Switcher struct {
	registry *Registry
	updater  ProviderUpdater
	current  *Chain
}

// NewSwitcher creates a Switcher backed by the given registry and provider updater.
func NewSwitcher(registry *Registry, updater ProviderUpdater) *Switcher {
	return &Switcher{
		registry: registry,
		updater:  updater,
	}
}

// Switch looks up a chain by name, short name, decimal ID, or hex ID,
// updates the current chain, calls the provider updater, and returns the chain.
func (s *Switcher) Switch(nameOrID string) (*Chain, error) {
	c, err := s.registry.Get(nameOrID)
	if err != nil {
		return nil, fmt.Errorf("switch chain: %w", err)
	}
	s.current = c
	if s.updater != nil {
		s.updater.SetChainID(c.ID)
	}
	return c, nil
}

// Current returns the currently active chain, or nil if none has been set.
func (s *Switcher) Current() *Chain {
	return s.current
}

// CurrentID returns the chain ID of the currently active chain, or 0 if none.
func (s *Switcher) CurrentID() uint64 {
	if s.current == nil {
		return 0
	}
	return s.current.ID
}
