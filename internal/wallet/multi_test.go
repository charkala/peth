package wallet

import (
	"testing"
)

func TestMultiWalletAssign(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	if _, err := ks.Create("alice"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mw := NewMultiWallet(ks)
	if err := mw.Assign("tab-1", "alice"); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	key, err := mw.GetForTab("tab-1")
	if err != nil {
		t.Fatalf("GetForTab: %v", err)
	}
	if key.Name != "alice" {
		t.Errorf("Name = %q, want %q", key.Name, "alice")
	}
}

func TestMultiWalletAssignInvalidWallet(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	mw := NewMultiWallet(ks)
	err = mw.Assign("tab-1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent wallet")
	}
}

func TestMultiWalletGetForTab(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	created, err := ks.Create("bob")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mw := NewMultiWallet(ks)
	if err := mw.Assign("tab-2", "bob"); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	key, err := mw.GetForTab("tab-2")
	if err != nil {
		t.Fatalf("GetForTab: %v", err)
	}
	if key.Address != created.Address {
		t.Errorf("Address = %q, want %q", key.Address, created.Address)
	}
}

func TestMultiWalletGetForTabUnassigned(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}

	mw := NewMultiWallet(ks)
	_, err = mw.GetForTab("tab-unknown")
	if err == nil {
		t.Fatal("expected error for unassigned tab")
	}
}

func TestMultiWalletUnassign(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	if _, err := ks.Create("charlie"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mw := NewMultiWallet(ks)
	if err := mw.Assign("tab-3", "charlie"); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	mw.Unassign("tab-3")

	_, err = mw.GetForTab("tab-3")
	if err == nil {
		t.Fatal("expected error after Unassign")
	}
}

func TestMultiWalletListAssignments(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	if _, err := ks.Create("w1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := ks.Create("w2"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := ks.Create("w3"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	mw := NewMultiWallet(ks)
	mw.Assign("tab-a", "w1")
	mw.Assign("tab-b", "w2")
	mw.Assign("tab-c", "w3")

	assignments := mw.ListAssignments()
	if len(assignments) != 3 {
		t.Fatalf("ListAssignments returned %d, want 3", len(assignments))
	}
	if assignments["tab-a"] != "w1" {
		t.Errorf("tab-a = %q, want %q", assignments["tab-a"], "w1")
	}
	if assignments["tab-b"] != "w2" {
		t.Errorf("tab-b = %q, want %q", assignments["tab-b"], "w2")
	}
	if assignments["tab-c"] != "w3" {
		t.Errorf("tab-c = %q, want %q", assignments["tab-c"], "w3")
	}
}

func TestMultiWalletReassign(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewKeystore(dir)
	if err != nil {
		t.Fatalf("NewKeystore: %v", err)
	}
	if _, err := ks.Create("first"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	second, err := ks.Create("second")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mw := NewMultiWallet(ks)
	if err := mw.Assign("tab-1", "first"); err != nil {
		t.Fatalf("Assign first: %v", err)
	}
	if err := mw.Assign("tab-1", "second"); err != nil {
		t.Fatalf("Assign second: %v", err)
	}

	key, err := mw.GetForTab("tab-1")
	if err != nil {
		t.Fatalf("GetForTab: %v", err)
	}
	if key.Name != "second" {
		t.Errorf("Name = %q, want %q", key.Name, "second")
	}
	if key.Address != second.Address {
		t.Errorf("Address = %q, want %q", key.Address, second.Address)
	}
}
