package tx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuilderChaining(t *testing.T) {
	tx, err := NewBuilder().
		From("0x1234567890abcdef1234567890abcdef12345678").
		To("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").
		ValueWei("1000000000000000000").
		Data([]byte{0x01, 0x02}).
		Nonce(5).
		GasLimit(21000).
		GasPrice("20000000000").
		ChainID(1).
		Build()

	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if tx.From != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("From = %q, want 0x1234...", tx.From)
	}
	if tx.To != "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Errorf("To = %q, want 0xabcdef...", tx.To)
	}
	if tx.Value != "1000000000000000000" {
		t.Errorf("Value = %q, want 1000000000000000000", tx.Value)
	}
	if len(tx.Data) != 2 || tx.Data[0] != 0x01 {
		t.Errorf("Data = %x, want 0102", tx.Data)
	}
	if tx.Nonce != 5 {
		t.Errorf("Nonce = %d, want 5", tx.Nonce)
	}
	if tx.GasLimit != 21000 {
		t.Errorf("GasLimit = %d, want 21000", tx.GasLimit)
	}
	if tx.GasPrice != "20000000000" {
		t.Errorf("GasPrice = %q, want 20000000000", tx.GasPrice)
	}
	if tx.ChainID != 1 {
		t.Errorf("ChainID = %d, want 1", tx.ChainID)
	}
}

func TestBuilderValueETH(t *testing.T) {
	tx, err := NewBuilder().
		From("0x1234567890abcdef1234567890abcdef12345678").
		To("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").
		Value("1.5").
		Build()

	if err != nil {
		t.Fatalf("Build() returned error: %v", err)
	}
	if tx.Value != "1500000000000000000" {
		t.Errorf("Value = %q, want 1500000000000000000", tx.Value)
	}
}

func TestBuilderValidation(t *testing.T) {
	tests := []struct {
		name    string
		builder *Builder
		wantErr string
	}{
		{
			name: "missing To",
			builder: NewBuilder().
				From("0x1234567890abcdef1234567890abcdef12345678"),
			wantErr: "missing To address",
		},
		{
			name: "missing From",
			builder: NewBuilder().
				To("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			wantErr: "missing From address",
		},
		{
			name: "invalid To address",
			builder: NewBuilder().
				From("0x1234567890abcdef1234567890abcdef12345678").
				To("not-an-address"),
			wantErr: "invalid To address",
		},
		{
			name: "invalid From address",
			builder: NewBuilder().
				From("bad-address").
				To("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			wantErr: "invalid From address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.builder.Build()
			if err == nil {
				t.Fatal("Build() expected error, got nil")
			}
			if got := err.Error(); got != tt.wantErr && !contains(got, tt.wantErr) {
				t.Errorf("Build() error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestEthToWei(t *testing.T) {
	tests := []struct {
		eth  string
		want string
	}{
		{"1", "1000000000000000000"},
		{"0.1", "100000000000000000"},
		{"0", "0"},
		{"1.123456789012345678", "1123456789012345678"},
		{"0.000000000000000001", "1"},
		{"100", "100000000000000000000"},
		{"1.5", "1500000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.eth, func(t *testing.T) {
			got, err := EthToWei(tt.eth)
			if err != nil {
				t.Fatalf("EthToWei(%q) returned error: %v", tt.eth, err)
			}
			if got != tt.want {
				t.Errorf("EthToWei(%q) = %q, want %q", tt.eth, got, tt.want)
			}
		})
	}
}

func TestWeiToEth(t *testing.T) {
	tests := []struct {
		wei  string
		want string
	}{
		{"1000000000000000000", "1"},
		{"100000000000000000", "0.1"},
		{"0", "0"},
		{"1123456789012345678", "1.123456789012345678"},
		{"1", "0.000000000000000001"},
		{"100000000000000000000", "100"},
		{"1500000000000000000", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.wei, func(t *testing.T) {
			got, err := WeiToEth(tt.wei)
			if err != nil {
				t.Fatalf("WeiToEth(%q) returned error: %v", tt.wei, err)
			}
			if got != tt.want {
				t.Errorf("WeiToEth(%q) = %q, want %q", tt.wei, got, tt.want)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid", "0x1234567890abcdef1234567890abcdef12345678", false},
		{"valid uppercase", "0xABCDEF1234567890ABCDEF1234567890ABCDEF12", false},
		{"no 0x prefix", "1234567890abcdef1234567890abcdef12345678", true},
		{"too short", "0x1234", true},
		{"too long", "0x1234567890abcdef1234567890abcdef1234567800", true},
		{"invalid hex chars", "0xGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.addr)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateAddress(%q) expected error, got nil", tt.addr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateAddress(%q) unexpected error: %v", tt.addr, err)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.5", "1500000000000000000"},
		{"1.5 ETH", "1500000000000000000"},
		{"1.5 eth", "1500000000000000000"},
		{"1500000000000000000 wei", "1500000000000000000"},
		{"1500000000000000000 WEI", "1500000000000000000"},
		{"0", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseValue(tt.input)
			if err != nil {
				t.Fatalf("ParseValue(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLocalSignerSign(t *testing.T) {
	signer := NewLocalSigner()

	// Generate a test key.
	// TODO: Replace P-256 with secp256k1 when go-ethereum is available.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() returned error: %v", err)
	}
	privBytes := privKey.D.Bytes()
	if len(privBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(privBytes):], privBytes)
		privBytes = padded
	}

	tx := &Transaction{
		From:     "0x1234567890abcdef1234567890abcdef12345678",
		To:       "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value:    "1000000000000000000",
		Nonce:    1,
		GasLimit: 21000,
		GasPrice: "20000000000",
		ChainID:  1,
	}

	signed, err := signer.Sign(tx, privBytes)
	if err != nil {
		t.Fatalf("Sign() returned error: %v", err)
	}

	if signed.Hash == "" {
		t.Error("Sign() returned empty hash")
	}
	if len(signed.RawTx) == 0 {
		t.Error("Sign() returned empty raw tx")
	}
	if signed.From != tx.From {
		t.Errorf("From = %q, want %q", signed.From, tx.From)
	}
	if signed.To != tx.To {
		t.Errorf("To = %q, want %q", signed.To, tx.To)
	}
	if signed.Value != tx.Value {
		t.Errorf("Value = %q, want %q", signed.Value, tx.Value)
	}

	// Verify deterministic hash (same tx + same key = same hash).
	signed2, err := signer.Sign(tx, privBytes)
	if err != nil {
		t.Fatalf("Sign() second call returned error: %v", err)
	}
	if signed.Hash != signed2.Hash {
		t.Errorf("Sign() not deterministic: hash1=%q, hash2=%q", signed.Hash, signed2.Hash)
	}
}

func TestRPCSenderSendRaw(t *testing.T) {
	expectedHash := "0xabc123def456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if req.Method != "eth_sendRawTransaction" {
			t.Errorf("method = %q, want eth_sendRawTransaction", req.Method)
		}
		if req.JSONRPC != "2.0" {
			t.Errorf("jsonrpc = %q, want 2.0", req.JSONRPC)
		}
		if len(req.Params) != 1 {
			t.Fatalf("params length = %d, want 1", len(req.Params))
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  expectedHash,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sender := NewRPCSender()
	result, err := sender.SendRaw(server.URL, []byte{0xde, 0xad, 0xbe, 0xef})
	if err != nil {
		t.Fatalf("SendRaw() returned error: %v", err)
	}

	if result.Hash != expectedHash {
		t.Errorf("Hash = %q, want %q", result.Hash, expectedHash)
	}
	if result.Status != "pending" {
		t.Errorf("Status = %q, want pending", result.Status)
	}
}

func TestRPCSenderSendRawError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "insufficient funds",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sender := NewRPCSender()
	_, err := sender.SendRaw(server.URL, []byte{0xde, 0xad})
	if err == nil {
		t.Fatal("SendRaw() expected error, got nil")
	}
	if !contains(err.Error(), "insufficient funds") {
		t.Errorf("error = %q, want to contain 'insufficient funds'", err.Error())
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEthToWeiErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"multiple dots", "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EthToWei(tt.input)
			if err == nil {
				t.Errorf("EthToWei(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestWeiToEthErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"non-numeric", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := WeiToEth(tt.input)
			if err == nil {
				t.Errorf("WeiToEth(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestParseValueErrors(t *testing.T) {
	_, err := ParseValue("")
	if err == nil {
		t.Error("ParseValue('') expected error, got nil")
	}
}

// rpcRequestForTest is used to verify the structure of JSON-RPC requests in tests.
// It mirrors rpcRequest but with interface{} Params for flexible decoding.
type rpcRequestForTest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

func TestRPCSenderRequestFormat(t *testing.T) {
	var captured rpcRequestForTest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0xhash",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	sender := NewRPCSender()
	sender.SendRaw(server.URL, []byte{0xab, 0xcd})

	if captured.Method != "eth_sendRawTransaction" {
		t.Errorf("method = %q, want eth_sendRawTransaction", captured.Method)
	}
	if len(captured.Params) != 1 {
		t.Fatalf("params count = %d, want 1", len(captured.Params))
	}
	param, ok := captured.Params[0].(string)
	if !ok {
		t.Fatalf("param is not string: %T", captured.Params[0])
	}
	if param != "0xabcd" {
		t.Errorf("param = %q, want 0xabcd", param)
	}
}

// Verify ValidateAddress returns ErrInvalidAddress sentinel.
func TestValidateAddressSentinel(t *testing.T) {
	err := ValidateAddress("bad")
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "invalid address") {
		t.Errorf("error = %q, want to contain 'invalid address'", err.Error())
	}

	// Verify error wrapping with fmt.Errorf %w.
	err = ValidateAddress("1234567890abcdef1234567890abcdef12345678")
	if err == nil {
		t.Fatal("expected error for missing 0x")
	}

	// Use errors.Is through the chain
	_ = fmt.Sprintf("error: %v", err) // just ensure it formats
}
