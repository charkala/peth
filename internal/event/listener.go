// Package event provides blockchain event listening and filtering.
package event

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EventFilter describes which on-chain events to listen for.
type EventFilter struct {
	ContractAddress string
	EventSignature  string
	Topics          []string
}

// Event represents an on-chain log event.
type Event struct {
	Address     string
	Topics      []string
	Data        []byte
	BlockNumber uint64
	TxHash      string
}

// Listener polls an EVM JSON-RPC endpoint for log events.
type Listener struct {
	rpcURL     string
	httpClient *http.Client
	pollInterval time.Duration
}

// NewListener creates a new Listener for the given RPC URL.
func NewListener(rpcURL string) *Listener {
	return &Listener{
		rpcURL:       rpcURL,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		pollInterval: 2 * time.Second,
	}
}

// Subscribe starts polling for events matching the filter.
// Returns an event channel and a cancel function to stop polling.
func (l *Listener) Subscribe(filter EventFilter) (<-chan Event, func(), error) {
	ch := make(chan Event, 16)
	var once sync.Once
	done := make(chan struct{})

	cancel := func() {
		once.Do(func() {
			close(done)
		})
	}

	fromBlock := "latest"

	go func() {
		defer close(ch)
		for {
			select {
			case <-done:
				return
			default:
			}

			events, lastBlock, err := l.getLogs(filter, fromBlock)
			if err == nil {
				for _, evt := range events {
					select {
					case ch <- evt:
					case <-done:
						return
					}
				}
				if lastBlock > 0 {
					fromBlock = "0x" + strconv.FormatUint(lastBlock+1, 16)
				}
			}

			select {
			case <-done:
				return
			case <-time.After(l.pollInterval):
			}
		}
	}()

	return ch, cancel, nil
}

// WaitForEvent blocks until an event matching the filter is received or timeout expires.
func (l *Listener) WaitForEvent(filter EventFilter, timeout time.Duration) (*Event, error) {
	ch, cancel, err := l.Subscribe(filter)
	if err != nil {
		return nil, err
	}
	defer cancel()

	select {
	case evt, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("event channel closed")
		}
		return &evt, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for event after %v", timeout)
	}
}

// EventSignatureHash computes topic0 from an event signature.
// TODO: Replace SHA-256 with keccak256 when go-ethereum is available.
func EventSignatureHash(sig string) string {
	hash := sha256.Sum256([]byte(sig))
	return "0x" + hex.EncodeToString(hash[:])
}

// --- JSON-RPC helpers ---

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type logEntry struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
}

// getLogs calls eth_getLogs on the RPC endpoint.
func (l *Listener) getLogs(filter EventFilter, fromBlock string) ([]Event, uint64, error) {
	// Build topics array.
	topics := make([]any, 0)
	if filter.EventSignature != "" {
		topics = append(topics, EventSignatureHash(filter.EventSignature))
	} else if len(filter.Topics) > 0 {
		for _, t := range filter.Topics {
			topics = append(topics, t)
		}
	}

	filterObj := map[string]any{
		"fromBlock": fromBlock,
		"toBlock":   "latest",
	}
	if filter.ContractAddress != "" {
		filterObj["address"] = filter.ContractAddress
	}
	if len(topics) > 0 {
		filterObj["topics"] = topics
	}

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_getLogs",
		Params:  []any{filterObj},
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}

	resp, err := l.httpClient.Post(l.rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("rpc request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, 0, fmt.Errorf("parse rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, 0, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var logs []logEntry
	if err := json.Unmarshal(rpcResp.Result, &logs); err != nil {
		return nil, 0, fmt.Errorf("parse logs: %w", err)
	}

	events := make([]Event, 0, len(logs))
	var maxBlock uint64

	for _, log := range logs {
		bn := parseHexUint64(log.BlockNumber)
		if bn > maxBlock {
			maxBlock = bn
		}

		dataBytes, _ := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))

		events = append(events, Event{
			Address:     log.Address,
			Topics:      log.Topics,
			Data:        dataBytes,
			BlockNumber: bn,
			TxHash:      log.TxHash,
		})
	}

	return events, maxBlock, nil
}

// parseHexUint64 parses a 0x-prefixed hex string to uint64.
func parseHexUint64(s string) uint64 {
	s = strings.TrimPrefix(s, "0x")
	n, _ := strconv.ParseUint(s, 16, 64)
	return n
}
