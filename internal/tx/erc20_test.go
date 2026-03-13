package tx

import (
	"encoding/hex"
	"math/big"
	"testing"
)

func TestApproveData(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)

	spender := "0x1234567890abcdef1234567890abcdef12345678"
	amount := "1000000000000000000" // 1 token (18 decimals)

	data, err := erc.ApproveData(spender, amount)
	if err != nil {
		t.Fatalf("ApproveData() returned error: %v", err)
	}

	// Verify total length: 4 (selector) + 32 (address) + 32 (amount) = 68.
	if len(data) != 68 {
		t.Fatalf("ApproveData() length = %d, want 68", len(data))
	}

	// Verify function selector (first 4 bytes).
	selector := data[:4]
	expectedSelector := functionSelector("approve(address,uint256)")
	if !bytesEqual(selector, expectedSelector) {
		t.Errorf("selector = 0x%s, want 0x%s", hex.EncodeToString(selector), hex.EncodeToString(expectedSelector))
	}

	// Verify address encoding (bytes 4-36): should be left-padded 20-byte address.
	addrPart := data[4:36]
	// First 12 bytes should be zero (padding).
	for i := 0; i < 12; i++ {
		if addrPart[i] != 0 {
			t.Errorf("address padding byte %d = 0x%02x, want 0x00", i, addrPart[i])
		}
	}

	// Verify amount encoding (bytes 36-68).
	amountPart := data[36:68]
	expectedAmount := new(big.Int)
	expectedAmount.SetString(amount, 10)
	gotAmount := new(big.Int).SetBytes(amountPart)
	if gotAmount.Cmp(expectedAmount) != 0 {
		t.Errorf("amount = %s, want %s", gotAmount.String(), expectedAmount.String())
	}
}

func TestAllowanceData(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)

	owner := "0x1234567890abcdef1234567890abcdef12345678"
	spender := "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"

	data, err := erc.AllowanceData(owner, spender)
	if err != nil {
		t.Fatalf("AllowanceData() returned error: %v", err)
	}

	// Verify total length: 4 (selector) + 32 (owner) + 32 (spender) = 68.
	if len(data) != 68 {
		t.Fatalf("AllowanceData() length = %d, want 68", len(data))
	}

	// Verify function selector.
	selector := data[:4]
	expectedSelector := functionSelector("allowance(address,address)")
	if !bytesEqual(selector, expectedSelector) {
		t.Errorf("selector = 0x%s, want 0x%s", hex.EncodeToString(selector), hex.EncodeToString(expectedSelector))
	}
}

func TestTransferData(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)

	to := "0x1234567890abcdef1234567890abcdef12345678"
	amount := "500000000000000000" // 0.5 tokens

	data, err := erc.TransferData(to, amount)
	if err != nil {
		t.Fatalf("TransferData() returned error: %v", err)
	}

	// Verify total length: 4 + 32 + 32 = 68.
	if len(data) != 68 {
		t.Fatalf("TransferData() length = %d, want 68", len(data))
	}

	// Verify function selector.
	selector := data[:4]
	expectedSelector := functionSelector("transfer(address,uint256)")
	if !bytesEqual(selector, expectedSelector) {
		t.Errorf("selector = 0x%s, want 0x%s", hex.EncodeToString(selector), hex.EncodeToString(expectedSelector))
	}

	// Verify amount.
	amountPart := data[36:68]
	expectedAmount := new(big.Int)
	expectedAmount.SetString(amount, 10)
	gotAmount := new(big.Int).SetBytes(amountPart)
	if gotAmount.Cmp(expectedAmount) != 0 {
		t.Errorf("amount = %s, want %s", gotAmount.String(), expectedAmount.String())
	}
}

func TestApproveDataInvalidAddress(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)

	tests := []struct {
		name    string
		spender string
	}{
		{"no prefix", "1234567890abcdef1234567890abcdef12345678"},
		{"too short", "0x1234"},
		{"invalid hex", "0xGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := erc.ApproveData(tt.spender, "1000")
			if err == nil {
				t.Error("ApproveData() expected error for invalid spender, got nil")
			}
		})
	}
}

func TestApproveDataMaxUint256(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)

	spender := "0x1234567890abcdef1234567890abcdef12345678"
	// Max uint256 = 2^256 - 1
	maxUint256 := new(big.Int)
	maxUint256.Exp(big.NewInt(2), big.NewInt(256), nil)
	maxUint256.Sub(maxUint256, big.NewInt(1))

	data, err := erc.ApproveData(spender, maxUint256.String())
	if err != nil {
		t.Fatalf("ApproveData() with max uint256 returned error: %v", err)
	}

	if len(data) != 68 {
		t.Fatalf("length = %d, want 68", len(data))
	}

	// Verify max uint256 encoding: all 32 bytes should be 0xFF.
	amountPart := data[36:68]
	for i, b := range amountPart {
		if b != 0xFF {
			t.Errorf("byte %d = 0x%02x, want 0xFF", i, b)
		}
	}
}

func TestAllowanceDataInvalidOwner(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)
	_, err := erc.AllowanceData("bad-addr", "0x1234567890abcdef1234567890abcdef12345678")
	if err == nil {
		t.Error("expected error for invalid owner")
	}
}

func TestAllowanceDataInvalidSpender(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)
	_, err := erc.AllowanceData("0x1234567890abcdef1234567890abcdef12345678", "bad-addr")
	if err == nil {
		t.Error("expected error for invalid spender")
	}
}

func TestTransferDataInvalidAddress(t *testing.T) {
	erc := NewERC20("0xdAC17F958D2ee523a2206206994597C13D831ec7", 1)
	_, err := erc.TransferData("bad-addr", "1000")
	if err == nil {
		t.Error("expected error for invalid to address")
	}
}

// bytesEqual compares two byte slices.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
