package chain

import (
	"errors"
	"testing"
)

// mockUpdater records SetChainID calls for test verification.
type mockUpdater struct {
	calls []uint64
}

func (m *mockUpdater) SetChainID(id uint64) {
	m.calls = append(m.calls, id)
}

func TestSwitcherSwitchByName(t *testing.T) {
	r := NewRegistry()
	s := NewSwitcher(r, &mockUpdater{})

	c, err := s.Switch("optimism")
	if err != nil {
		t.Fatalf("Switch(optimism) returned error: %v", err)
	}
	if c.ID != 10 {
		t.Errorf("Switch(optimism).ID = %d, want 10", c.ID)
	}
	if s.CurrentID() != 10 {
		t.Errorf("CurrentID() = %d, want 10", s.CurrentID())
	}
}

func TestSwitcherSwitchByID(t *testing.T) {
	r := NewRegistry()
	s := NewSwitcher(r, &mockUpdater{})

	c, err := s.Switch("137")
	if err != nil {
		t.Fatalf("Switch(137) returned error: %v", err)
	}
	if c.Name != "polygon" {
		t.Errorf("Switch(137).Name = %q, want polygon", c.Name)
	}
	if s.CurrentID() != 137 {
		t.Errorf("CurrentID() = %d, want 137", s.CurrentID())
	}
}

func TestSwitcherSwitchByHex(t *testing.T) {
	r := NewRegistry()
	s := NewSwitcher(r, &mockUpdater{})

	c, err := s.Switch("0xa")
	if err != nil {
		t.Fatalf("Switch(0xa) returned error: %v", err)
	}
	if c.Name != "optimism" {
		t.Errorf("Switch(0xa).Name = %q, want optimism", c.Name)
	}
	if c.ID != 10 {
		t.Errorf("Switch(0xa).ID = %d, want 10", c.ID)
	}
}

func TestSwitcherSwitchNotFound(t *testing.T) {
	r := NewRegistry()
	s := NewSwitcher(r, &mockUpdater{})

	_, err := s.Switch("unknown-chain")
	if err == nil {
		t.Fatal("Switch(unknown-chain) expected error, got nil")
	}
	if !errors.Is(err, ErrChainNotFound) {
		t.Errorf("Switch(unknown-chain) error = %v, want ErrChainNotFound", err)
	}
}

func TestSwitcherCurrent(t *testing.T) {
	r := NewRegistry()
	s := NewSwitcher(r, &mockUpdater{})

	// Before any switch, current is nil.
	if s.Current() != nil {
		t.Error("Current() before any switch should be nil")
	}
	if s.CurrentID() != 0 {
		t.Errorf("CurrentID() before any switch = %d, want 0", s.CurrentID())
	}

	// Switch to ethereum.
	if _, err := s.Switch("ethereum"); err != nil {
		t.Fatalf("Switch(ethereum) returned error: %v", err)
	}
	if s.Current().ID != 1 {
		t.Errorf("Current().ID = %d, want 1", s.Current().ID)
	}

	// Switch to polygon, verify current updates.
	if _, err := s.Switch("polygon"); err != nil {
		t.Fatalf("Switch(polygon) returned error: %v", err)
	}
	if s.Current().Name != "polygon" {
		t.Errorf("Current().Name = %q, want polygon", s.Current().Name)
	}
	if s.CurrentID() != 137 {
		t.Errorf("CurrentID() = %d, want 137", s.CurrentID())
	}
}

func TestSwitcherCallsUpdater(t *testing.T) {
	r := NewRegistry()
	m := &mockUpdater{}
	s := NewSwitcher(r, m)

	// Switch twice.
	if _, err := s.Switch("ethereum"); err != nil {
		t.Fatalf("Switch(ethereum) returned error: %v", err)
	}
	if _, err := s.Switch("optimism"); err != nil {
		t.Fatalf("Switch(optimism) returned error: %v", err)
	}

	if len(m.calls) != 2 {
		t.Fatalf("updater received %d calls, want 2", len(m.calls))
	}
	if m.calls[0] != 1 {
		t.Errorf("updater call[0] = %d, want 1 (ethereum)", m.calls[0])
	}
	if m.calls[1] != 10 {
		t.Errorf("updater call[1] = %d, want 10 (optimism)", m.calls[1])
	}
}
