package tx

import (
	"errors"
	"testing"
)

// mockSigner implements Signer for testing.
type mockSigner struct {
	signFn func(tx *Transaction, privateKey []byte) (*SignedTransaction, error)
}

func (m *mockSigner) Sign(tx *Transaction, privateKey []byte) (*SignedTransaction, error) {
	if m.signFn != nil {
		return m.signFn(tx, privateKey)
	}
	return &SignedTransaction{
		RawTx: []byte{0xde, 0xad},
		Hash:  "0xmockhash",
		From:  tx.From,
		To:    tx.To,
		Value: tx.Value,
		Nonce: tx.Nonce,
	}, nil
}

// mockSender implements Sender for testing.
type mockSender struct {
	sendFn func(rpcURL string, rawTx []byte) (*SendResult, error)
}

func (m *mockSender) SendRaw(rpcURL string, rawTx []byte) (*SendResult, error) {
	if m.sendFn != nil {
		return m.sendFn(rpcURL, rawTx)
	}
	return &SendResult{
		Hash:   "0xmockhash",
		Status: "pending",
	}, nil
}

func newTestInterceptor(policy Policy) *Interceptor {
	i := NewInterceptor(&mockSigner{}, &mockSender{})
	i.SetPolicy(policy)
	i.SetKey([]byte("test-key-32-bytes-padding-here!!"))
	i.SetRPCURL("http://localhost:8545")
	return i
}

func TestInterceptorAutoApprove(t *testing.T) {
	i := newTestInterceptor(PolicyAutoApprove)

	tx := &Transaction{
		From:     "0x1234567890abcdef1234567890abcdef12345678",
		To:       "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value:    "1000000000000000000",
		GasLimit: 21000,
		GasPrice: "20000000000",
	}

	result, err := i.HandleTransaction(tx)
	if err != nil {
		t.Fatalf("HandleTransaction() returned error: %v", err)
	}
	if result.Hash != "0xmockhash" {
		t.Errorf("Hash = %q, want 0xmockhash", result.Hash)
	}
	if result.Status != "pending" {
		t.Errorf("Status = %q, want pending", result.Status)
	}
}

func TestInterceptorReject(t *testing.T) {
	i := newTestInterceptor(PolicyReject)

	tx := &Transaction{
		From:  "0x1234567890abcdef1234567890abcdef12345678",
		To:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value: "1000000000000000000",
	}

	_, err := i.HandleTransaction(tx)
	if err == nil {
		t.Fatal("HandleTransaction() expected error, got nil")
	}
	if !errors.Is(err, ErrTransactionRejected) {
		t.Errorf("error = %v, want ErrTransactionRejected", err)
	}
}

func TestInterceptorMaxGasPass(t *testing.T) {
	i := newTestInterceptor(PolicyAutoApprove)
	i.AddRule(Rule{
		Policy:    PolicyMaxGas,
		MaxGasWei: "1000000000000000", // 0.001 ETH max gas cost
	})

	tx := &Transaction{
		From:     "0x1234567890abcdef1234567890abcdef12345678",
		To:       "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value:    "1000000000000000000",
		GasLimit: 21000,
		GasPrice: "20000000000", // 20 Gwei -> total = 420000 Gwei = 0.00042 ETH
	}

	result, err := i.HandleTransaction(tx)
	if err != nil {
		t.Fatalf("HandleTransaction() returned error: %v", err)
	}
	if result.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestInterceptorMaxGasFail(t *testing.T) {
	i := newTestInterceptor(PolicyAutoApprove)
	i.AddRule(Rule{
		Policy:    PolicyMaxGas,
		MaxGasWei: "100000000000", // very low max
	})

	tx := &Transaction{
		From:     "0x1234567890abcdef1234567890abcdef12345678",
		To:       "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value:    "1000000000000000000",
		GasLimit: 21000,
		GasPrice: "20000000000", // 20 Gwei -> total = 420000000000000 > 100000000000
	}

	_, err := i.HandleTransaction(tx)
	if err == nil {
		t.Fatal("HandleTransaction() expected error for exceeding max gas, got nil")
	}
	if !errors.Is(err, ErrTransactionRejected) {
		t.Errorf("error = %v, want ErrTransactionRejected", err)
	}
}

func TestInterceptorPrompt(t *testing.T) {
	i := newTestInterceptor(PolicyPrompt)

	tx := &Transaction{
		From:  "0x1234567890abcdef1234567890abcdef12345678",
		To:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Value: "1000000000000000000",
	}

	_, err := i.HandleTransaction(tx)
	if err == nil {
		t.Fatal("HandleTransaction() expected error, got nil")
	}
	if !errors.Is(err, ErrPromptRequired) {
		t.Errorf("error = %v, want ErrPromptRequired", err)
	}
}

func TestInterceptorSignError(t *testing.T) {
	failSigner := &mockSigner{
		signFn: func(tx *Transaction, privateKey []byte) (*SignedTransaction, error) {
			return nil, errors.New("sign failed")
		},
	}
	i := NewInterceptor(failSigner, &mockSender{})
	i.SetPolicy(PolicyAutoApprove)
	i.SetKey([]byte("key"))

	tx := &Transaction{
		From: "0x1234567890abcdef1234567890abcdef12345678",
		To:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	_, err := i.HandleTransaction(tx)
	if err == nil {
		t.Fatal("expected error from failed sign")
	}
}

func TestInterceptorSendError(t *testing.T) {
	failSender := &mockSender{
		sendFn: func(rpcURL string, rawTx []byte) (*SendResult, error) {
			return nil, errors.New("send failed")
		},
	}
	i := NewInterceptor(&mockSigner{}, failSender)
	i.SetPolicy(PolicyAutoApprove)
	i.SetKey([]byte("key"))

	tx := &Transaction{
		From: "0x1234567890abcdef1234567890abcdef12345678",
		To:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}

	_, err := i.HandleTransaction(tx)
	if err == nil {
		t.Fatal("expected error from failed send")
	}
}
