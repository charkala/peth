package chain

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrChainNotFound is returned when a chain lookup finds no match.
var ErrChainNotFound = errors.New("chain not found")

// Currency describes a chain's native currency.
type Currency struct {
	Name     string
	Symbol   string
	Decimals int
}

// Chain represents an EVM-compatible blockchain network.
type Chain struct {
	ID             uint64
	Name           string
	ShortName      string
	NativeCurrency Currency
	RPCURLs        []string
	ExplorerURLs   []string
	Testnet        bool
}

// Registry holds a set of known chains indexed for fast lookup.
type Registry struct {
	byID       map[uint64]*Chain
	byName     map[string]*Chain // lower-cased name and short name
	builtinIDs map[uint64]bool
}

var builtinChains = []Chain{
	{ID: 1, Name: "ethereum", ShortName: "eth", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://eth.llamarpc.com"}, ExplorerURLs: []string{"https://etherscan.io"}, Testnet: false},
	{ID: 10, Name: "optimism", ShortName: "op", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://mainnet.optimism.io"}, ExplorerURLs: []string{"https://optimistic.etherscan.io"}, Testnet: false},
	{ID: 137, Name: "polygon", ShortName: "matic", NativeCurrency: Currency{"POL", "POL", 18}, RPCURLs: []string{"https://polygon-rpc.com"}, ExplorerURLs: []string{"https://polygonscan.com"}, Testnet: false},
	{ID: 42161, Name: "arbitrum", ShortName: "arb", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://arb1.arbitrum.io/rpc"}, ExplorerURLs: []string{"https://arbiscan.io"}, Testnet: false},
	{ID: 8453, Name: "base", ShortName: "base", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://mainnet.base.org"}, ExplorerURLs: []string{"https://basescan.org"}, Testnet: false},
	{ID: 43114, Name: "avalanche", ShortName: "avax", NativeCurrency: Currency{"Avalanche", "AVAX", 18}, RPCURLs: []string{"https://api.avax.network/ext/bc/C/rpc"}, ExplorerURLs: []string{"https://snowtrace.io"}, Testnet: false},
	{ID: 56, Name: "bsc", ShortName: "bnb", NativeCurrency: Currency{"BNB", "BNB", 18}, RPCURLs: []string{"https://bsc-dataseed.binance.org"}, ExplorerURLs: []string{"https://bscscan.com"}, Testnet: false},
	{ID: 324, Name: "zksync", ShortName: "zksync", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://mainnet.era.zksync.io"}, ExplorerURLs: []string{"https://explorer.zksync.io"}, Testnet: false},
	{ID: 59144, Name: "linea", ShortName: "linea", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://rpc.linea.build"}, ExplorerURLs: []string{"https://lineascan.build"}, Testnet: false},
	// Testnets
	{ID: 11155111, Name: "sepolia", ShortName: "sep", NativeCurrency: Currency{"Sepolia Ether", "SEP", 18}, RPCURLs: []string{"https://rpc.sepolia.org"}, ExplorerURLs: []string{"https://sepolia.etherscan.io"}, Testnet: true},
	{ID: 421614, Name: "arbitrum-sepolia", ShortName: "arb-sep", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://sepolia-rollup.arbitrum.io/rpc"}, ExplorerURLs: []string{"https://sepolia.arbiscan.io"}, Testnet: true},
	{ID: 84532, Name: "base-sepolia", ShortName: "base-sep", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"https://sepolia.base.org"}, ExplorerURLs: []string{"https://sepolia.basescan.org"}, Testnet: true},
	{ID: 31337, Name: "hardhat", ShortName: "hh", NativeCurrency: Currency{"Ether", "ETH", 18}, RPCURLs: []string{"http://localhost:8545"}, ExplorerURLs: nil, Testnet: true},
}

// NewRegistry creates a Registry pre-loaded with built-in EVM chains.
func NewRegistry() *Registry {
	r := &Registry{
		byID:       make(map[uint64]*Chain),
		byName:     make(map[string]*Chain),
		builtinIDs: make(map[uint64]bool),
	}
	for i := range builtinChains {
		c := &builtinChains[i]
		r.byID[c.ID] = c
		r.byName[strings.ToLower(c.Name)] = c
		if c.ShortName != "" && strings.ToLower(c.ShortName) != strings.ToLower(c.Name) {
			r.byName[strings.ToLower(c.ShortName)] = c
		}
		r.builtinIDs[c.ID] = true
	}
	return r
}

// Get looks up a chain by name (case-insensitive), short name, decimal chain ID,
// or hex chain ID (0x prefix).
func (r *Registry) Get(nameOrID string) (*Chain, error) {
	if nameOrID == "" {
		return nil, ErrChainNotFound
	}

	// Try numeric ID (hex or decimal).
	if id, err := parseNumericID(nameOrID); err == nil {
		if c, ok := r.byID[id]; ok {
			return c, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrChainNotFound, nameOrID)
	}

	// Try name / short name lookup.
	if c, ok := r.byName[strings.ToLower(nameOrID)]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrChainNotFound, nameOrID)
}

// List returns all registered chains sorted by chain ID.
func (r *Registry) List() []*Chain {
	chains := make([]*Chain, 0, len(r.byID))
	for _, c := range r.byID {
		chains = append(chains, c)
	}
	sort.Slice(chains, func(i, j int) bool {
		return chains[i].ID < chains[j].ID
	})
	return chains
}

// Add registers a custom chain. Returns an error if the chain ID already exists.
func (r *Registry) Add(chain Chain) error {
	if _, exists := r.byID[chain.ID]; exists {
		return fmt.Errorf("chain ID %d already exists", chain.ID)
	}
	c := new(Chain)
	*c = chain
	r.byID[c.ID] = c
	r.byName[strings.ToLower(c.Name)] = c
	if c.ShortName != "" && strings.ToLower(c.ShortName) != strings.ToLower(c.Name) {
		r.byName[strings.ToLower(c.ShortName)] = c
	}
	return nil
}

// Remove deletes a custom chain by ID. Built-in chains cannot be removed.
func (r *Registry) Remove(id uint64) error {
	if r.builtinIDs[id] {
		return fmt.Errorf("cannot remove built-in chain %d", id)
	}
	c, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("%w: %d", ErrChainNotFound, id)
	}
	delete(r.byID, id)
	delete(r.byName, strings.ToLower(c.Name))
	if c.ShortName != "" {
		delete(r.byName, strings.ToLower(c.ShortName))
	}
	return nil
}

// ParseChainID resolves a user-provided string to a numeric chain ID.
// It accepts decimal numbers, hex strings (0x prefix), or chain names/short names
// (looked up via the provided registry).
func ParseChainID(input string, r *Registry) (uint64, error) {
	if input == "" {
		return 0, fmt.Errorf("empty chain ID")
	}

	// Try numeric first.
	if id, err := parseNumericID(input); err == nil {
		return id, nil
	}

	// Try name lookup via registry.
	c, err := r.Get(input)
	if err != nil {
		return 0, fmt.Errorf("unknown chain: %s", input)
	}
	return c.ID, nil
}

// parseNumericID parses a decimal or 0x-prefixed hex string to uint64.
func parseNumericID(s string) (uint64, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}
