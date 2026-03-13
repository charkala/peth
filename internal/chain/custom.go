package chain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
)

// CustomChainOpts holds the parameters for adding a custom chain.
type CustomChainOpts struct {
	Name                   string `json:"name"`
	ShortName              string `json:"shortName,omitempty"`
	RPCURL                 string `json:"rpcURL"`
	ChainID                uint64 `json:"chainID"`
	NativeCurrencyName     string `json:"nativeCurrencyName,omitempty"`
	NativeCurrencySymbol   string `json:"nativeCurrencySymbol,omitempty"`
	NativeCurrencyDecimals int    `json:"nativeCurrencyDecimals,omitempty"`
	ExplorerURL            string `json:"explorerURL,omitempty"`
}

// AddCustom validates opts, builds a Chain, and registers it in the registry.
func (r *Registry) AddCustom(opts CustomChainOpts) (*Chain, error) {
	if err := validateCustomOpts(opts); err != nil {
		return nil, fmt.Errorf("add custom chain: %w", err)
	}

	c := Chain{
		ID:        opts.ChainID,
		Name:      opts.Name,
		ShortName: opts.ShortName,
		NativeCurrency: Currency{
			Name:     opts.NativeCurrencyName,
			Symbol:   opts.NativeCurrencySymbol,
			Decimals: opts.NativeCurrencyDecimals,
		},
		RPCURLs: []string{opts.RPCURL},
	}
	if opts.ExplorerURL != "" {
		c.ExplorerURLs = []string{opts.ExplorerURL}
	}

	if err := r.Add(c); err != nil {
		return nil, err
	}

	// Retrieve the pointer that Add stored.
	stored, _ := r.Get(fmt.Sprintf("%d", opts.ChainID))
	return stored, nil
}

func validateCustomOpts(opts CustomChainOpts) error {
	if opts.Name == "" {
		return errors.New("name is required")
	}
	if opts.ChainID == 0 {
		return errors.New("chain ID is required and must be > 0")
	}
	if opts.RPCURL == "" {
		return errors.New("RPC URL is required")
	}
	u, err := url.Parse(opts.RPCURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid RPC URL: %s", opts.RPCURL)
	}
	return nil
}

// SaveCustom writes all non-builtin chains to a JSON file at path.
func (r *Registry) SaveCustom(path string) error {
	var customs []CustomChainOpts
	for id, c := range r.byID {
		if r.builtinIDs[id] {
			continue
		}
		opts := CustomChainOpts{
			Name:                   c.Name,
			ShortName:              c.ShortName,
			ChainID:                c.ID,
			NativeCurrencyName:     c.NativeCurrency.Name,
			NativeCurrencySymbol:   c.NativeCurrency.Symbol,
			NativeCurrencyDecimals: c.NativeCurrency.Decimals,
		}
		if len(c.RPCURLs) > 0 {
			opts.RPCURL = c.RPCURLs[0]
		}
		if len(c.ExplorerURLs) > 0 {
			opts.ExplorerURL = c.ExplorerURLs[0]
		}
		customs = append(customs, opts)
	}

	data, err := json.MarshalIndent(customs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal custom chains: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadCustom reads custom chains from a JSON file and adds them to the registry.
// If the file does not exist, it is a no-op.
func (r *Registry) LoadCustom(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read custom chains: %w", err)
	}

	var customs []CustomChainOpts
	if err := json.Unmarshal(data, &customs); err != nil {
		return fmt.Errorf("parse custom chains: %w", err)
	}

	for _, opts := range customs {
		if _, err := r.AddCustom(opts); err != nil {
			return fmt.Errorf("load custom chain %q: %w", opts.Name, err)
		}
	}
	return nil
}
