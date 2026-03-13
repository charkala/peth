package tx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

// AssertionResult holds the outcome of an assertion check.
type AssertionResult struct {
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Actual   string `json:"actual"`
	Expected string `json:"expected"`
}

// AssertBalance checks an address's ETH balance against a threshold.
// operator must be one of: "gte", "lte", "eq", "gt", "lt".
// amount is in ETH (e.g., "1.5").
func AssertBalance(rpcURL string, address string, operator string, amount string) (*AssertionResult, error) {
	// Validate operator
	validOps := map[string]bool{"gte": true, "lte": true, "eq": true, "gt": true, "lt": true}
	if !validOps[operator] {
		return nil, fmt.Errorf("invalid operator %q: must be one of gte, lte, eq, gt, lt", operator)
	}

	// Call eth_getBalance
	result, err := callRPC(rpcURL, "eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		return nil, fmt.Errorf("eth_getBalance: %w", err)
	}

	// Parse hex balance
	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}

	balanceWei := new(big.Int)
	balanceHex = strings.TrimPrefix(balanceHex, "0x")
	balanceWei.SetString(balanceHex, 16)

	// Convert expected amount from ETH to wei
	expectedWei, err := ethToWeiBig(amount)
	if err != nil {
		return nil, fmt.Errorf("parse expected amount: %w", err)
	}

	// Convert actual balance to ETH for display
	actualETH, _ := WeiToEth(balanceWei.String())

	// Compare
	cmp := balanceWei.Cmp(expectedWei)
	var passed bool
	switch operator {
	case "gte":
		passed = cmp >= 0
	case "lte":
		passed = cmp <= 0
	case "eq":
		passed = cmp == 0
	case "gt":
		passed = cmp > 0
	case "lt":
		passed = cmp < 0
	}

	msg := fmt.Sprintf("balance %s ETH %s %s ETH", actualETH, operator, amount)
	if passed {
		msg = "PASS: " + msg
	} else {
		msg = "FAIL: " + msg
	}

	return &AssertionResult{
		Passed:   passed,
		Message:  msg,
		Actual:   actualETH,
		Expected: amount,
	}, nil
}

// AssertTxStatus checks a transaction receipt status.
// expectedStatus must be "success" (status=0x1) or "failed" (status=0x0).
func AssertTxStatus(rpcURL string, txHash string, expectedStatus string) (*AssertionResult, error) {
	result, err := callRPC(rpcURL, "eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, fmt.Errorf("eth_getTransactionReceipt: %w", err)
	}

	// Check for null result (pending transaction)
	if string(result) == "null" {
		return &AssertionResult{
			Passed:   false,
			Message:  "FAIL: transaction is pending (no receipt)",
			Actual:   "pending",
			Expected: expectedStatus,
		}, nil
	}

	// Parse receipt
	var receipt struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &receipt); err != nil {
		return nil, fmt.Errorf("parse receipt: %w", err)
	}

	actualStatus := "failed"
	if receipt.Status == "0x1" {
		actualStatus = "success"
	}

	passed := actualStatus == expectedStatus
	msg := fmt.Sprintf("tx status: %s", actualStatus)
	if passed {
		msg = "PASS: " + msg
	} else {
		msg = "FAIL: " + msg + ", expected " + expectedStatus
	}

	return &AssertionResult{
		Passed:   passed,
		Message:  msg,
		Actual:   actualStatus,
		Expected: expectedStatus,
	}, nil
}

// AssertChainID checks the RPC node's chain ID against an expected value.
func AssertChainID(rpcURL string, expectedID uint64) (*AssertionResult, error) {
	result, err := callRPC(rpcURL, "eth_chainId", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("eth_chainId: %w", err)
	}

	var chainHex string
	if err := json.Unmarshal(result, &chainHex); err != nil {
		return nil, fmt.Errorf("parse chain ID: %w", err)
	}

	chainHex = strings.TrimPrefix(chainHex, "0x")
	actualID, err := strconv.ParseUint(chainHex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("parse chain ID hex: %w", err)
	}

	passed := actualID == expectedID
	actualStr := strconv.FormatUint(actualID, 10)
	expectedStr := strconv.FormatUint(expectedID, 10)

	msg := fmt.Sprintf("chain ID: %s", actualStr)
	if passed {
		msg = "PASS: " + msg
	} else {
		msg = "FAIL: " + msg + ", expected " + expectedStr
	}

	return &AssertionResult{
		Passed:   passed,
		Message:  msg,
		Actual:   actualStr,
		Expected: expectedStr,
	}, nil
}

// callRPC makes a JSON-RPC 2.0 call and returns the raw result.
func callRPC(rpcURL string, method string, params []interface{}) (json.RawMessage, error) {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// ethToWeiBig converts an ETH amount string to a *big.Int in wei.
func ethToWeiBig(eth string) (*big.Int, error) {
	weiStr, err := EthToWei(eth)
	if err != nil {
		return nil, err
	}
	wei := new(big.Int)
	if _, ok := wei.SetString(weiStr, 10); !ok {
		return nil, fmt.Errorf("invalid wei value: %s", weiStr)
	}
	return wei, nil
}
