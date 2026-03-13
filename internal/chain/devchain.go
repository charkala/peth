package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
)

// DevCommander abstracts subprocess execution for testability.
type DevCommander interface {
	Start(name string, args ...string) (DevProcess, error)
}

// DevProcess abstracts a running subprocess.
type DevProcess interface {
	Stop() error
	IsRunning() bool
}

// DevChainOpts configures a local development chain.
type DevChainOpts struct {
	Tool      string // "anvil" or "hardhat"
	Port      int
	ChainID   uint64
	Accounts  int
	BlockTime int
}

// DevChain manages a local development blockchain (Anvil or Hardhat).
type DevChain struct {
	opts           DevChainOpts
	commander      DevCommander
	process        DevProcess
	rpcURLOverride string // for testing without a real subprocess
}

// NewDevChain creates a DevChain with the given options.
func NewDevChain(opts DevChainOpts) *DevChain {
	return &DevChain{opts: opts}
}

// Start launches the development chain as a subprocess.
func (dc *DevChain) Start() error {
	if dc.process != nil {
		return fmt.Errorf("devchain is already running")
	}

	name, args := dc.buildCommand()

	proc, err := dc.commander.Start(name, args...)
	if err != nil {
		return fmt.Errorf("start devchain: %w", err)
	}
	dc.process = proc
	return nil
}

// Stop terminates the development chain subprocess.
func (dc *DevChain) Stop() error {
	if dc.process == nil {
		return fmt.Errorf("devchain is not running")
	}
	if err := dc.process.Stop(); err != nil {
		return fmt.Errorf("stop devchain: %w", err)
	}
	dc.process = nil
	return nil
}

// IsRunning returns true if the subprocess is tracked and alive.
func (dc *DevChain) IsRunning() bool {
	return dc.process != nil && dc.process.IsRunning()
}

// RPCURL returns the JSON-RPC URL for this dev chain.
func (dc *DevChain) RPCURL() string {
	if dc.rpcURLOverride != "" {
		return dc.rpcURLOverride
	}
	return fmt.Sprintf("http://localhost:%d", dc.opts.Port)
}

// Snapshot creates an EVM state snapshot and returns the snapshot ID.
func (dc *DevChain) Snapshot() (string, error) {
	result, err := dc.rpcCall("evm_snapshot", []interface{}{})
	if err != nil {
		return "", err
	}
	var id string
	if err := json.Unmarshal(result, &id); err != nil {
		return "", fmt.Errorf("parse snapshot id: %w", err)
	}
	return id, nil
}

// Revert reverts the EVM state to a previous snapshot.
func (dc *DevChain) Revert(snapshotID string) error {
	_, err := dc.rpcCall("evm_revert", []interface{}{snapshotID})
	return err
}

// FundAccount sets the balance for an address using the tool-specific method.
func (dc *DevChain) FundAccount(address string, amountETH string) error {
	// Convert ETH to wei hex
	weiStr, err := ethToWeiDC(amountETH)
	if err != nil {
		return fmt.Errorf("convert amount: %w", err)
	}
	weiHex := "0x" + new(big.Int).SetBytes(weiFromDecimal(weiStr)).Text(16)

	method := "anvil_setBalance"
	if dc.opts.Tool == "hardhat" {
		method = "hardhat_setBalance"
	}

	_, err = dc.rpcCall(method, []interface{}{address, weiHex})
	return err
}

// buildCommand returns the command name and arguments for starting the chain.
func (dc *DevChain) buildCommand() (string, []string) {
	port := strconv.Itoa(dc.opts.Port)

	if dc.opts.Tool == "hardhat" {
		args := []string{"hardhat", "node", "--port", port}
		if dc.opts.ChainID > 0 {
			args = append(args, "--chain-id", strconv.FormatUint(dc.opts.ChainID, 10))
		}
		return "npx", args
	}

	// Default: anvil
	args := []string{"--port", port}
	if dc.opts.ChainID > 0 {
		args = append(args, "--chain-id", strconv.FormatUint(dc.opts.ChainID, 10))
	}
	if dc.opts.Accounts > 0 {
		args = append(args, "--accounts", strconv.Itoa(dc.opts.Accounts))
	}
	if dc.opts.BlockTime > 0 {
		args = append(args, "--block-time", strconv.Itoa(dc.opts.BlockTime))
	}
	return "anvil", args
}

// rpcCall makes a JSON-RPC call to the dev chain.
func (dc *DevChain) rpcCall(method string, params []interface{}) (json.RawMessage, error) {
	type rpcReq struct {
		JSONRPC string        `json:"jsonrpc"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
		ID      int           `json:"id"`
	}

	body, err := json.Marshal(rpcReq{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(dc.RPCURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	var rr rpcResp
	if err := json.Unmarshal(respBody, &rr); err != nil {
		return nil, err
	}

	if rr.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rr.Error.Code, rr.Error.Message)
	}

	return rr.Result, nil
}

// ethToWeiDC converts ETH to wei decimal string (local helper to avoid circular deps).
func ethToWeiDC(eth string) (string, error) {
	// Simple conversion: multiply by 1e18
	val := new(big.Float).SetPrec(256)
	if _, ok := val.SetString(eth); !ok {
		return "", fmt.Errorf("invalid ETH amount: %s", eth)
	}
	weiPerEth := new(big.Float).SetPrec(256).SetUint64(1000000000000000000)
	val.Mul(val, weiPerEth)
	wei := new(big.Int)
	val.Int(wei)
	return wei.String(), nil
}

// weiFromDecimal converts a decimal string to big-endian bytes.
func weiFromDecimal(s string) []byte {
	n := new(big.Int)
	n.SetString(s, 10)
	return n.Bytes()
}
