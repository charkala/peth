package tx

import (
	"errors"
	"fmt"
	"math/big"
)

// Policy determines how transactions are handled by the Interceptor.
type Policy int

const (
	// PolicyAutoApprove signs and sends transactions automatically.
	PolicyAutoApprove Policy = iota
	// PolicyPrompt requires user confirmation before signing.
	PolicyPrompt
	// PolicyReject rejects all transactions.
	PolicyReject
	// PolicyMaxGas approves transactions under a gas ceiling, rejects above.
	PolicyMaxGas
)

// ErrTransactionRejected is returned when a transaction is rejected by policy.
var ErrTransactionRejected = errors.New("transaction rejected")

// ErrPromptRequired is returned when user confirmation is needed.
var ErrPromptRequired = errors.New("user confirmation required")

// Rule defines a transaction handling rule.
type Rule struct {
	Policy    Policy
	MaxGasWei string // used with PolicyMaxGas
}

// Interceptor evaluates transactions against rules and policies.
type Interceptor struct {
	policy Policy
	rules  []Rule
	signer Signer
	sender Sender
	rpcURL string
	key    []byte
}

// NewInterceptor creates a new Interceptor with the given signer and sender.
func NewInterceptor(signer Signer, sender Sender) *Interceptor {
	return &Interceptor{
		policy: PolicyPrompt, // safe default
		signer: signer,
		sender: sender,
	}
}

// SetPolicy sets the default transaction policy.
func (i *Interceptor) SetPolicy(p Policy) {
	i.policy = p
}

// AddRule appends a transaction handling rule.
func (i *Interceptor) AddRule(r Rule) {
	i.rules = append(i.rules, r)
}

// SetRPCURL sets the RPC URL for sending transactions.
func (i *Interceptor) SetRPCURL(url string) {
	i.rpcURL = url
}

// SetKey sets the private key for signing transactions.
func (i *Interceptor) SetKey(key []byte) {
	i.key = key
}

// HandleTransaction evaluates a transaction against the interceptor's rules and policy.
// It signs and sends approved transactions, or returns an error for rejected/prompted ones.
func (i *Interceptor) HandleTransaction(tx *Transaction) (*SendResult, error) {
	// Check rules first (they override default policy).
	for _, rule := range i.rules {
		switch rule.Policy {
		case PolicyMaxGas:
			totalGas := computeTotalGas(tx)
			maxGas := new(big.Int)
			maxGas.SetString(rule.MaxGasWei, 10)
			if totalGas.Cmp(maxGas) > 0 {
				return nil, fmt.Errorf("%w: gas %s exceeds max %s", ErrTransactionRejected, totalGas.String(), rule.MaxGasWei)
			}
		}
	}

	// Apply default policy.
	switch i.policy {
	case PolicyAutoApprove:
		return i.signAndSend(tx)
	case PolicyReject:
		return nil, ErrTransactionRejected
	case PolicyPrompt:
		return nil, ErrPromptRequired
	case PolicyMaxGas:
		// Default policy is MaxGas but no rule matched — treat as prompt.
		return nil, ErrPromptRequired
	}

	return nil, fmt.Errorf("unknown policy: %d", i.policy)
}

// signAndSend signs the transaction and broadcasts it.
func (i *Interceptor) signAndSend(tx *Transaction) (*SendResult, error) {
	signed, err := i.signer.Sign(tx, i.key)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	result, err := i.sender.SendRaw(i.rpcURL, signed.RawTx)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	return result, nil
}

// computeTotalGas computes the total gas cost in wei for a transaction.
// For legacy transactions: gasLimit * gasPrice.
func computeTotalGas(tx *Transaction) *big.Int {
	gasPrice := new(big.Int)
	if tx.GasPrice != "" {
		gasPrice.SetString(tx.GasPrice, 10)
	}
	gasLimit := new(big.Int).SetUint64(tx.GasLimit)
	return new(big.Int).Mul(gasLimit, gasPrice)
}
