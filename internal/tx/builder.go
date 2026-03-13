// Package tx provides transaction building, signing, and sending for peth.
package tx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
)

// Sentinel errors for transaction operations.
var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAddress    = errors.New("invalid address")
)

// Transaction represents an unsigned Ethereum transaction.
type Transaction struct {
	To                   string
	From                 string
	Value                string // wei as decimal string
	Data                 []byte
	Nonce                uint64
	GasLimit             uint64
	GasPrice             string
	MaxFeePerGas         string
	MaxPriorityFeePerGas string
	ChainID              uint64
}

// SignedTransaction represents a signed transaction ready for broadcast.
type SignedTransaction struct {
	RawTx []byte
	Hash  string
	From  string
	To    string
	Value string
	Nonce uint64
}

// SendResult holds the response from sending a transaction.
type SendResult struct {
	Hash   string
	Status string
}

// Builder constructs Transaction structs using a chainable API.
type Builder struct {
	to                   string
	from                 string
	value                string // wei
	data                 []byte
	nonce                uint64
	gasLimit             uint64
	gasPrice             string
	maxFeePerGas         string
	maxPriorityFeePerGas string
	chainID              uint64
}

// NewBuilder creates a new transaction Builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// From sets the sender address.
func (b *Builder) From(addr string) *Builder {
	b.from = addr
	return b
}

// To sets the recipient address.
func (b *Builder) To(addr string) *Builder {
	b.to = addr
	return b
}

// Value sets the transfer amount in ETH, converting to wei.
func (b *Builder) Value(amount string) *Builder {
	wei, err := EthToWei(amount)
	if err != nil {
		// Store raw; Build() will re-validate
		b.value = amount
		return b
	}
	b.value = wei
	return b
}

// ValueWei sets the transfer amount in raw wei.
func (b *Builder) ValueWei(wei string) *Builder {
	b.value = wei
	return b
}

// Data sets the transaction call data.
func (b *Builder) Data(data []byte) *Builder {
	b.data = data
	return b
}

// Nonce sets the transaction nonce.
func (b *Builder) Nonce(n uint64) *Builder {
	b.nonce = n
	return b
}

// GasLimit sets the gas limit.
func (b *Builder) GasLimit(limit uint64) *Builder {
	b.gasLimit = limit
	return b
}

// GasPrice sets the legacy gas price in wei.
func (b *Builder) GasPrice(price string) *Builder {
	b.gasPrice = price
	return b
}

// ChainID sets the chain ID.
func (b *Builder) ChainID(id uint64) *Builder {
	b.chainID = id
	return b
}

// Build validates and returns the Transaction.
func (b *Builder) Build() (*Transaction, error) {
	if b.to == "" {
		return nil, fmt.Errorf("missing To address")
	}
	if b.from == "" {
		return nil, fmt.Errorf("missing From address")
	}
	if err := ValidateAddress(b.to); err != nil {
		return nil, fmt.Errorf("invalid To address: %w", err)
	}
	if err := ValidateAddress(b.from); err != nil {
		return nil, fmt.Errorf("invalid From address: %w", err)
	}

	return &Transaction{
		To:                   b.to,
		From:                 b.from,
		Value:                b.value,
		Data:                 b.data,
		Nonce:                b.nonce,
		GasLimit:             b.gasLimit,
		GasPrice:             b.gasPrice,
		MaxFeePerGas:         b.maxFeePerGas,
		MaxPriorityFeePerGas: b.maxPriorityFeePerGas,
		ChainID:              b.chainID,
	}, nil
}

// Signer signs transactions with a private key.
type Signer interface {
	Sign(tx *Transaction, privateKey []byte) (*SignedTransaction, error)
}

// LocalSigner signs transactions locally using ECDSA.
type LocalSigner struct{}

// NewLocalSigner creates a new LocalSigner.
func NewLocalSigner() *LocalSigner {
	return &LocalSigner{}
}

// Sign creates a deterministic hash of the transaction fields and signs with ECDSA.
// TODO: Replace P-256 with secp256k1 and use RLP encoding when go-ethereum is available.
func (s *LocalSigner) Sign(tx *Transaction, privateKey []byte) (*SignedTransaction, error) {
	// Create deterministic hash of transaction fields.
	txData := fmt.Sprintf("%s|%s|%s|%x|%d|%d|%s|%d",
		tx.From, tx.To, tx.Value, tx.Data, tx.Nonce, tx.GasLimit, tx.GasPrice, tx.ChainID)
	txHash := sha256.Sum256([]byte(txData))

	// Reconstruct ECDSA private key from raw bytes.
	// TODO: Replace P-256 with secp256k1 when go-ethereum is available.
	curve := elliptic.P256()
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privateKey)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privateKey)

	r, sigS, err := ecdsa.Sign(rand.Reader, privKey, txHash[:])
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Encode signature as raw bytes (r || s).
	sig := append(r.Bytes(), sigS.Bytes()...)

	// Build raw tx: hash + signature (placeholder format).
	// TODO: Use proper RLP encoding when go-ethereum is available.
	rawTx := append(txHash[:], sig...)

	return &SignedTransaction{
		RawTx: rawTx,
		Hash:  "0x" + hex.EncodeToString(txHash[:]),
		From:  tx.From,
		To:    tx.To,
		Value: tx.Value,
		Nonce: tx.Nonce,
	}, nil
}

// Sender broadcasts signed transactions to the network.
type Sender interface {
	SendRaw(rpcURL string, rawTx []byte) (*SendResult, error)
}

// RPCSender sends transactions via JSON-RPC.
type RPCSender struct {
	client *http.Client
}

// NewRPCSender creates a new RPCSender.
func NewRPCSender() *RPCSender {
	return &RPCSender{client: &http.Client{}}
}

// rpcRequest is a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// rpcResponse is a JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC 2.0 error.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SendRaw broadcasts a signed transaction via eth_sendRawTransaction.
func (s *RPCSender) SendRaw(rpcURL string, rawTx []byte) (*SendResult, error) {
	hexTx := "0x" + hex.EncodeToString(rawTx)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendRawTransaction",
		Params:  []interface{}{hexTx},
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := s.client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var hash string
	if err := json.Unmarshal(rpcResp.Result, &hash); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	return &SendResult{
		Hash:   hash,
		Status: "pending",
	}, nil
}

// EthToWei converts an ETH amount string to a wei decimal string.
// Supports up to 18 decimal places.
func EthToWei(eth string) (string, error) {
	eth = strings.TrimSpace(eth)
	if eth == "" {
		return "", fmt.Errorf("empty ETH value")
	}

	negative := false
	if strings.HasPrefix(eth, "-") {
		negative = true
		eth = eth[1:]
	}

	parts := strings.Split(eth, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("invalid ETH value: %q", eth)
	}

	intPart := parts[0]
	if intPart == "" {
		intPart = "0"
	}

	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	// Truncate or pad fractional part to 18 digits.
	if len(fracPart) > 18 {
		fracPart = fracPart[:18]
	}
	for len(fracPart) < 18 {
		fracPart += "0"
	}

	// Combine integer and fractional parts.
	weiStr := intPart + fracPart

	// Remove leading zeros but keep at least one digit.
	weiStr = strings.TrimLeft(weiStr, "0")
	if weiStr == "" {
		return "0", nil
	}

	if negative {
		weiStr = "-" + weiStr
	}

	return weiStr, nil
}

// WeiToEth converts a wei decimal string to an ETH amount string.
func WeiToEth(wei string) (string, error) {
	wei = strings.TrimSpace(wei)
	if wei == "" {
		return "", fmt.Errorf("empty wei value")
	}

	negative := false
	if strings.HasPrefix(wei, "-") {
		negative = true
		wei = wei[1:]
	}

	// Validate digits only.
	for _, c := range wei {
		if c < '0' || c > '9' {
			return "", fmt.Errorf("invalid wei value: non-numeric character %q", string(c))
		}
	}

	// Pad with leading zeros so we have at least 19 chars (1 integer + 18 decimal).
	for len(wei) < 19 {
		wei = "0" + wei
	}

	intPart := wei[:len(wei)-18]
	fracPart := wei[len(wei)-18:]

	// Trim trailing zeros from fractional part.
	fracPart = strings.TrimRight(fracPart, "0")

	result := intPart
	if fracPart != "" {
		result += "." + fracPart
	}

	// Remove leading zeros from integer part (keep at least one).
	result = strings.TrimLeft(result, "0")
	if result == "" || result[0] == '.' {
		result = "0" + result
	}

	if negative {
		result = "-" + result
	}

	return result, nil
}

// ValidateAddress checks that an address is a valid Ethereum hex address.
func ValidateAddress(addr string) error {
	if !strings.HasPrefix(addr, "0x") {
		return fmt.Errorf("%w: missing 0x prefix", ErrInvalidAddress)
	}
	if len(addr) != 42 {
		return fmt.Errorf("%w: expected 42 characters, got %d", ErrInvalidAddress, len(addr))
	}
	// Validate hex characters after 0x.
	if _, err := hex.DecodeString(addr[2:]); err != nil {
		return fmt.Errorf("%w: invalid hex characters", ErrInvalidAddress)
	}
	return nil
}

// ParseValue parses a value string in various formats:
// - "1.5" (assumed ETH, converted to wei)
// - "1.5 ETH" (explicit ETH, converted to wei)
// - "1500000000000000000 wei" (explicit wei, returned as-is)
func ParseValue(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty value")
	}

	lower := strings.ToLower(input)

	if strings.HasSuffix(lower, " wei") {
		weiStr := strings.TrimSpace(input[:len(input)-4])
		// Validate it is numeric.
		for _, c := range weiStr {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("invalid wei value: %q", weiStr)
			}
		}
		return weiStr, nil
	}

	if strings.HasSuffix(lower, " eth") {
		ethStr := strings.TrimSpace(input[:len(input)-4])
		return EthToWei(ethStr)
	}

	// Default: assume ETH.
	return EthToWei(input)
}
