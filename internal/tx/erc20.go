package tx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// ERC20 provides methods to encode ERC-20 token ABI call data.
type ERC20 struct {
	TokenAddress string
	ChainID      uint64
}

// NewERC20 creates a new ERC20 encoder for the given token contract.
func NewERC20(tokenAddr string, chainID uint64) *ERC20 {
	return &ERC20{
		TokenAddress: tokenAddr,
		ChainID:      chainID,
	}
}

// ApproveData encodes an ERC-20 approve(address,uint256) call.
// Returns 68 bytes: 4-byte selector + 32-byte address + 32-byte amount.
//
// TODO: Replace SHA-256 with Keccak-256 for correct function selector.
// Real selector for approve(address,uint256) is 0x095ea7b3.
func (e *ERC20) ApproveData(spender string, amount string) ([]byte, error) {
	if err := ValidateAddress(spender); err != nil {
		return nil, fmt.Errorf("invalid spender address: %w", err)
	}

	selector := functionSelector("approve(address,uint256)")
	addrBytes, err := abiEncodeAddress(spender)
	if err != nil {
		return nil, err
	}
	amountBytes, err := abiEncodeUint256(amount)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, 68)
	data = append(data, selector...)
	data = append(data, addrBytes...)
	data = append(data, amountBytes...)
	return data, nil
}

// AllowanceData encodes an ERC-20 allowance(address,address) call.
// Returns 68 bytes: 4-byte selector + 32-byte owner + 32-byte spender.
func (e *ERC20) AllowanceData(owner, spender string) ([]byte, error) {
	if err := ValidateAddress(owner); err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}
	if err := ValidateAddress(spender); err != nil {
		return nil, fmt.Errorf("invalid spender address: %w", err)
	}

	selector := functionSelector("allowance(address,address)")
	ownerBytes, err := abiEncodeAddress(owner)
	if err != nil {
		return nil, err
	}
	spenderBytes, err := abiEncodeAddress(spender)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, 68)
	data = append(data, selector...)
	data = append(data, ownerBytes...)
	data = append(data, spenderBytes...)
	return data, nil
}

// TransferData encodes an ERC-20 transfer(address,uint256) call.
// Returns 68 bytes: 4-byte selector + 32-byte address + 32-byte amount.
func (e *ERC20) TransferData(to string, amount string) ([]byte, error) {
	if err := ValidateAddress(to); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	selector := functionSelector("transfer(address,uint256)")
	addrBytes, err := abiEncodeAddress(to)
	if err != nil {
		return nil, err
	}
	amountBytes, err := abiEncodeUint256(amount)
	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, 68)
	data = append(data, selector...)
	data = append(data, addrBytes...)
	data = append(data, amountBytes...)
	return data, nil
}

// functionSelector computes the first 4 bytes of the hash of a function signature.
// TODO: Replace SHA-256 with Keccak-256 for EVM-compatible selectors.
func functionSelector(sig string) []byte {
	hash := sha256.Sum256([]byte(sig))
	return hash[:4]
}

// abiEncodeAddress encodes an Ethereum address as a 32-byte left-padded value.
func abiEncodeAddress(addr string) ([]byte, error) {
	addr = strings.TrimPrefix(addr, "0x")
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		return nil, fmt.Errorf("decode address: %w", err)
	}

	// Left-pad to 32 bytes.
	result := make([]byte, 32)
	copy(result[32-len(addrBytes):], addrBytes)
	return result, nil
}

// abiEncodeUint256 encodes a decimal string amount as a 32-byte big-endian value.
func abiEncodeUint256(amount string) ([]byte, error) {
	n := new(big.Int)
	if _, ok := n.SetString(amount, 10); !ok {
		return nil, fmt.Errorf("invalid uint256 value: %q", amount)
	}
	if n.Sign() < 0 {
		return nil, fmt.Errorf("uint256 cannot be negative")
	}

	b := n.Bytes()
	if len(b) > 32 {
		return nil, fmt.Errorf("value exceeds uint256 max")
	}

	// Left-pad to 32 bytes.
	result := make([]byte, 32)
	copy(result[32-len(b):], b)
	return result, nil
}
