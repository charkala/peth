package tx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

// SimulationResult holds the result of a transaction simulation.
type SimulationResult struct {
	Success      bool
	GasUsed      uint64
	ReturnData   []byte
	RevertReason string
}

// Simulator simulates transactions without broadcasting.
type Simulator interface {
	Simulate(rpcURL string, tx *Transaction) (*SimulationResult, error)
}

// RPCSimulator simulates transactions via JSON-RPC eth_call.
type RPCSimulator struct {
	client *http.Client
}

// NewRPCSimulator creates a new RPCSimulator.
func NewRPCSimulator() *RPCSimulator {
	return &RPCSimulator{client: &http.Client{}}
}

// Simulate executes an eth_call and returns the simulation result.
func (s *RPCSimulator) Simulate(rpcURL string, tx *Transaction) (*SimulationResult, error) {
	callObj := buildCallObject(tx)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params:  []interface{}{callObj, "latest"},
		ID:      1,
	}

	resp, err := s.doRPC(rpcURL, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		reason := extractRevertReason(resp.Error.Message)
		return &SimulationResult{
			Success:      false,
			RevertReason: reason,
		}, nil
	}

	var resultHex string
	if err := json.Unmarshal(resp.Result, &resultHex); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	returnData, _ := hex.DecodeString(strings.TrimPrefix(resultHex, "0x"))

	return &SimulationResult{
		Success:    true,
		ReturnData: returnData,
	}, nil
}

// EstimateGas calls eth_estimateGas and returns the gas estimate.
func (s *RPCSimulator) EstimateGas(rpcURL string, tx *Transaction) (uint64, error) {
	callObj := buildCallObject(tx)

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_estimateGas",
		Params:  []interface{}{callObj},
		ID:      1,
	}

	resp, err := s.doRPC(rpcURL, req)
	if err != nil {
		return 0, err
	}

	if resp.Error != nil {
		return 0, fmt.Errorf("eth_estimateGas: %s", resp.Error.Message)
	}

	var gasHex string
	if err := json.Unmarshal(resp.Result, &gasHex); err != nil {
		return 0, fmt.Errorf("parse gas estimate: %w", err)
	}

	gasHex = strings.TrimPrefix(gasHex, "0x")
	gas, err := strconv.ParseUint(gasHex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse gas hex: %w", err)
	}

	return gas, nil
}

// doRPC sends a JSON-RPC request and returns the parsed response.
func (s *RPCSimulator) doRPC(rpcURL string, req rpcRequest) (*rpcResponse, error) {
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

	return &rpcResp, nil
}

// callObject is the transaction object for eth_call / eth_estimateGas.
type callObject struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to"`
	Value string `json:"value,omitempty"`
	Data  string `json:"data,omitempty"`
	Gas   string `json:"gas,omitempty"`
}

// buildCallObject converts a Transaction into a JSON-RPC call object.
func buildCallObject(tx *Transaction) callObject {
	obj := callObject{
		From: tx.From,
		To:   tx.To,
	}
	if tx.Value != "" && tx.Value != "0" {
		obj.Value = "0x" + fmt.Sprintf("%x", mustParseBigInt(tx.Value))
	}
	if len(tx.Data) > 0 {
		obj.Data = "0x" + hex.EncodeToString(tx.Data)
	}
	if tx.GasLimit > 0 {
		obj.Gas = "0x" + strconv.FormatUint(tx.GasLimit, 16)
	}
	return obj
}

// mustParseBigInt parses a decimal string to *big.Int, returning 0 on failure.
func mustParseBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}

// extractRevertReason attempts to extract a human-readable revert reason.
func extractRevertReason(msg string) string {
	// JSON-RPC errors may contain "execution reverted: <reason>"
	if idx := strings.Index(msg, "execution reverted:"); idx >= 0 {
		return strings.TrimSpace(msg[idx+len("execution reverted:"):])
	}
	return msg
}
