package chain

import (
	"errors"
	"sort"
	"testing"
)

func TestRegistryGetByName(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		name    string
		chainID uint64
	}{
		{"ethereum", 1},
		{"optimism", 10},
		{"polygon", 137},
		{"arbitrum", 42161},
		{"base", 8453},
		{"avalanche", 43114},
		{"bsc", 56},
		{"zksync", 324},
		{"linea", 59144},
		{"sepolia", 11155111},
		{"hardhat", 31337},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := r.Get(tt.name)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.name, err)
			}
			if c.ID != tt.chainID {
				t.Errorf("Get(%q).ID = %d, want %d", tt.name, c.ID, tt.chainID)
			}
		})
	}
}

func TestRegistryGetByNameCaseInsensitive(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		input   string
		chainID uint64
	}{
		{"Ethereum", 1},
		{"OPTIMISM", 10},
		{"Polygon", 137},
		{"BASE", 8453},
		{"AvAlAnChE", 43114},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c, err := r.Get(tt.input)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.input, err)
			}
			if c.ID != tt.chainID {
				t.Errorf("Get(%q).ID = %d, want %d", tt.input, c.ID, tt.chainID)
			}
		})
	}
}

func TestRegistryGetByShortName(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		short   string
		chainID uint64
	}{
		{"eth", 1},
		{"op", 10},
		{"matic", 137},
		{"arb", 42161},
		{"base", 8453},
		{"avax", 43114},
		{"bnb", 56},
		{"hh", 31337},
	}

	for _, tt := range tests {
		t.Run(tt.short, func(t *testing.T) {
			c, err := r.Get(tt.short)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.short, err)
			}
			if c.ID != tt.chainID {
				t.Errorf("Get(%q).ID = %d, want %d", tt.short, c.ID, tt.chainID)
			}
		})
	}
}

func TestRegistryGetByChainID(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		input   string
		chainID uint64
	}{
		{"1", 1},
		{"10", 10},
		{"137", 137},
		{"42161", 42161},
		{"8453", 8453},
		{"56", 56},
		{"31337", 31337},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c, err := r.Get(tt.input)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.input, err)
			}
			if c.ID != tt.chainID {
				t.Errorf("Get(%q).ID = %d, want %d", tt.input, c.ID, tt.chainID)
			}
		})
	}
}

func TestRegistryGetByHexChainID(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		input   string
		chainID uint64
	}{
		{"0x1", 1},
		{"0xa", 10},
		{"0x89", 137},
		{"0xA4B1", 42161},
		{"0x2105", 8453},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c, err := r.Get(tt.input)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.input, err)
			}
			if c.ID != tt.chainID {
				t.Errorf("Get(%q).ID = %d, want %d", tt.input, c.ID, tt.chainID)
			}
		})
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	tests := []string{"nonexistent", "999999", "0xFFFFFF"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := r.Get(input)
			if err == nil {
				t.Fatalf("Get(%q) expected error, got nil", input)
			}
			if !errors.Is(err, ErrChainNotFound) {
				t.Errorf("Get(%q) error = %v, want ErrChainNotFound", input, err)
			}
		})
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	chains := r.List()

	if len(chains) != len(builtinChains) {
		t.Fatalf("List() returned %d chains, want %d", len(chains), len(builtinChains))
	}

	// Verify sorted by ID.
	if !sort.SliceIsSorted(chains, func(i, j int) bool {
		return chains[i].ID < chains[j].ID
	}) {
		t.Error("List() chains are not sorted by ID")
	}
}

func TestRegistryListFilterTestnets(t *testing.T) {
	r := NewRegistry()
	chains := r.List()

	testnets := map[uint64]bool{
		11155111: true, // sepolia
		421614:  true,  // arbitrum-sepolia
		84532:   true,  // base-sepolia
		31337:   true,  // hardhat
	}

	for _, c := range chains {
		if expected, ok := testnets[c.ID]; ok {
			if c.Testnet != expected {
				t.Errorf("chain %s (ID %d): Testnet = %v, want %v", c.Name, c.ID, c.Testnet, expected)
			}
		} else {
			if c.Testnet {
				t.Errorf("chain %s (ID %d): Testnet = true, want false", c.Name, c.ID)
			}
		}
	}
}

func TestRegistryAdd(t *testing.T) {
	r := NewRegistry()

	custom := Chain{
		ID:             999,
		Name:           "custom-chain",
		ShortName:      "cust",
		NativeCurrency: Currency{"Custom", "CUST", 18},
		RPCURLs:        []string{"http://localhost:9999"},
		Testnet:        true,
	}

	if err := r.Add(custom); err != nil {
		t.Fatalf("Add() returned error: %v", err)
	}

	// Retrieve by name.
	c, err := r.Get("custom-chain")
	if err != nil {
		t.Fatalf("Get(custom-chain) returned error: %v", err)
	}
	if c.ID != 999 {
		t.Errorf("Get(custom-chain).ID = %d, want 999", c.ID)
	}

	// Retrieve by ID.
	c, err = r.Get("999")
	if err != nil {
		t.Fatalf("Get(999) returned error: %v", err)
	}
	if c.Name != "custom-chain" {
		t.Errorf("Get(999).Name = %q, want %q", c.Name, "custom-chain")
	}

	// Should appear in List.
	chains := r.List()
	if len(chains) != len(builtinChains)+1 {
		t.Errorf("List() returned %d chains, want %d", len(chains), len(builtinChains)+1)
	}
}

func TestRegistryAddDuplicate(t *testing.T) {
	r := NewRegistry()

	dup := Chain{
		ID:   1, // ethereum already exists
		Name: "duplicate",
	}

	err := r.Add(dup)
	if err == nil {
		t.Fatal("Add() with duplicate ID expected error, got nil")
	}
}

func TestRegistryRemoveCustom(t *testing.T) {
	r := NewRegistry()

	custom := Chain{
		ID:        999,
		Name:      "removable",
		ShortName: "rm",
	}
	if err := r.Add(custom); err != nil {
		t.Fatalf("Add() returned error: %v", err)
	}

	if err := r.Remove(999); err != nil {
		t.Fatalf("Remove(999) returned error: %v", err)
	}

	_, err := r.Get("999")
	if !errors.Is(err, ErrChainNotFound) {
		t.Errorf("Get(999) after Remove: expected ErrChainNotFound, got %v", err)
	}

	_, err = r.Get("removable")
	if !errors.Is(err, ErrChainNotFound) {
		t.Errorf("Get(removable) after Remove: expected ErrChainNotFound, got %v", err)
	}
}

func TestRegistryRemoveBuiltin(t *testing.T) {
	r := NewRegistry()

	err := r.Remove(1) // ethereum is built-in
	if err == nil {
		t.Fatal("Remove(1) built-in chain expected error, got nil")
	}
}

func TestParseChainID(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		input   string
		want    uint64
		wantErr bool
	}{
		// Decimal.
		{"1", 1, false},
		{"137", 137, false},
		{"42161", 42161, false},
		// Hex.
		{"0x1", 1, false},
		{"0xa", 10, false},
		{"0x89", 137, false},
		{"0xA4B1", 42161, false},
		// Name lookup.
		{"ethereum", 1, false},
		{"optimism", 10, false},
		{"polygon", 137, false},
		{"Ethereum", 1, false},
		// Short name.
		{"eth", 1, false},
		{"op", 10, false},
		// Invalid.
		{"", 0, true},
		{"not-a-chain", 0, true},
		{"0xZZZ", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseChainID(tt.input, r)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseChainID(%q) expected error, got %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseChainID(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseChainID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
