// Package dapp provides dApp wallet connection and interaction automation.
package dapp

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

// cdpTarget represents a Chrome DevTools Protocol debuggable target.
type cdpTarget struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	URL                 string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// PreloadInjector injects the EIP-1193 provider via CDP before page load,
// so that wallet detection SDKs (Privy, Dynamic, etc.) see window.ethereum
// during their initialization.
type PreloadInjector struct {
	cdpPort    int
	walletAddr string
}

// NewPreloadInjector creates a new PreloadInjector for the given CDP port.
func NewPreloadInjector(cdpPort int, walletAddr string) *PreloadInjector {
	return &PreloadInjector{cdpPort: cdpPort, walletAddr: walletAddr}
}

// InjectAll registers the provider preload script on all active targets
// matching the given URL pattern (empty = all page targets).
// It also evaluates the script immediately in the current context.
func (p *PreloadInjector) InjectAll(urlPattern string) (int, error) {
	targets, err := p.listTargets()
	if err != nil {
		return 0, fmt.Errorf("list CDP targets: %w", err)
	}

	count := 0
	for _, t := range targets {
		if t.WebSocketDebuggerURL == "" {
			continue
		}
		if urlPattern != "" && !strings.Contains(t.URL, urlPattern) {
			continue
		}
		if err := p.injectIntoTarget(t); err != nil {
			continue // best-effort; skip unreachable targets
		}
		count++
	}
	return count, nil
}

// RegisterPreloadOnTarget registers Page.addScriptToEvaluateOnNewDocument
// on the target that currently has the given URL loaded.
// Returns the script identifier.
func (p *PreloadInjector) RegisterPreloadOnTarget(targetID string) (string, error) {
	conn, err := p.dialCDP(targetID)
	if err != nil {
		return "", fmt.Errorf("dial CDP: %w", err)
	}
	defer conn.Close()

	// Enable Page domain first (required before addScriptToEvaluateOnNewDocument).
	if err := p.sendCDP(conn, 0, "Page.enable", nil); err != nil {
		return "", fmt.Errorf("Page.enable: %w", err)
	}
	p.recvCDP(conn, 1, 500*time.Millisecond)

	// Register script to run before any page JS on next navigation.
	params := map[string]interface{}{
		"source":         p.providerScript(),
		"runImmediately": false,
	}
	if err := p.sendCDP(conn, 1, "Page.addScriptToEvaluateOnNewDocument", params); err != nil {
		return "", fmt.Errorf("addScriptToEvaluateOnNewDocument: %w", err)
	}

	results := p.recvCDP(conn, 2, time.Second)
	for _, r := range results {
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Identifier string `json:"identifier"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(r), &resp); err == nil && resp.ID == 1 {
			return resp.Result.Identifier, nil
		}
	}
	return "", nil
}

// FindActiveTarget finds the CDP target ID for the tab currently showing the given URL.
func (p *PreloadInjector) FindActiveTarget(url string) (string, error) {
	targets, err := p.listTargets()
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, url) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no active target found for URL: %s", url)
}

// ListPageTargets returns target IDs for all active page targets.
func (p *PreloadInjector) ListPageTargets() ([]string, error) {
	targets, err := p.listTargets()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, t := range targets {
		if t.WebSocketDebuggerURL == "" {
			continue
		}
		id := t.WebSocketDebuggerURL[strings.LastIndex(t.WebSocketDebuggerURL, "/")+1:]
		ids = append(ids, id)
	}
	return ids, nil
}

// RegisterPreload registers Page.addScriptToEvaluateOnNewDocument on the given target.
// This is a convenience wrapper around RegisterPreloadOnTarget that ignores errors.
func (p *PreloadInjector) RegisterPreload(targetID string) {
	conn, err := p.dialCDP(targetID)
	if err != nil {
		return
	}
	defer conn.Close()

	// Enable Page domain first.
	p.sendCDP(conn, 0, "Page.enable", nil)
	p.recvCDP(conn, 1, 300*time.Millisecond)

	// Register script to run before any page JS on next navigation.
	p.sendCDP(conn, 1, "Page.addScriptToEvaluateOnNewDocument", map[string]interface{}{
		"source":         p.providerScript(),
		"runImmediately": false,
	})
	p.recvCDP(conn, 1, 300*time.Millisecond)
}

// injectIntoTarget registers and immediately evaluates the provider script
// in the given target.
func (p *PreloadInjector) injectIntoTarget(t cdpTarget) error {
	targetID := t.WebSocketDebuggerURL[strings.LastIndex(t.WebSocketDebuggerURL, "/")+1:]

	conn, err := p.dialCDP(targetID)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Enable Page domain.
	p.sendCDP(conn, 0, "Page.enable", nil)
	p.recvCDP(conn, 1, 300*time.Millisecond)

	// Register preload.
	p.sendCDP(conn, 1, "Page.addScriptToEvaluateOnNewDocument", map[string]interface{}{
		"source":         p.providerScript(),
		"runImmediately": false,
	})
	p.recvCDP(conn, 1, 300*time.Millisecond)

	// Eval immediately for already-loaded pages.
	p.sendCDP(conn, 2, "Runtime.evaluate", map[string]interface{}{
		"expression":   p.providerScript(),
		"returnByValue": true,
	})
	p.recvCDP(conn, 1, 300*time.Millisecond)

	return nil
}

// listTargets fetches all debuggable targets from the CDP HTTP endpoint.
func (p *PreloadInjector) listTargets() ([]cdpTarget, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/list", p.cdpPort)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("CDP list: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("parse targets: %w", err)
	}
	return targets, nil
}

// dialCDP opens a raw WebSocket connection to a CDP target.
func (p *PreloadInjector) dialCDP(targetID string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", p.cdpPort), 3*time.Second)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/devtools/page/%s", targetID)
	host := fmt.Sprintf("127.0.0.1:%d", p.cdpPort)
	handshake := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
			"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host,
	)
	if _, err := conn.Write([]byte(handshake)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read HTTP upgrade response.
	buf := make([]byte, 1024)
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	resp := ""
	for !strings.Contains(resp, "\r\n\r\n") {
		n, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("upgrade response: %w", err)
		}
		resp += string(buf[:n])
	}
	conn.SetDeadline(time.Time{}) // clear deadline
	return conn, nil
}

// sendCDP sends a CDP command as a masked WebSocket text frame.
func (p *PreloadInjector) sendCDP(conn net.Conn, id int, method string, params map[string]interface{}) error {
	if params == nil {
		params = map[string]interface{}{}
	}
	msg, err := json.Marshal(map[string]interface{}{
		"id":     id,
		"method": method,
		"params": params,
	})
	if err != nil {
		return err
	}
	frame := wsFrame(msg)
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = conn.Write(frame)
	conn.SetDeadline(time.Time{})
	return err
}

// recvCDP reads up to n WebSocket frames within the timeout.
func (p *PreloadInjector) recvCDP(conn net.Conn, n int, timeout time.Duration) []string {
	var results []string
	conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	for i := 0; i < n; i++ {
		header := make([]byte, 2)
		if _, err := io.ReadFull(conn, header); err != nil {
			break
		}
		length := int(header[1] & 0x7F)
		if length == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(conn, ext); err != nil {
				break
			}
			length = int(ext[0])<<8 | int(ext[1])
		} else if length == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(conn, ext); err != nil {
				break
			}
			length = 0
			for _, b := range ext {
				length = length<<8 | int(b)
			}
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(conn, data); err != nil {
			break
		}
		results = append(results, string(data))
	}
	return results
}

// wsFrame encodes a masked WebSocket text frame (client→server).
func wsFrame(data []byte) []byte {
	mask := [4]byte{
		byte(rand.Intn(256)),
		byte(rand.Intn(256)),
		byte(rand.Intn(256)),
		byte(rand.Intn(256)),
	}
	masked := make([]byte, len(data))
	for i, b := range data {
		masked[i] = b ^ mask[i%4]
	}

	var header []byte
	header = append(header, 0x81) // FIN + text frame
	l := len(data)
	switch {
	case l < 126:
		header = append(header, byte(0x80|l))
	case l < 65536:
		header = append(header, 0x80|126, byte(l>>8), byte(l))
	default:
		header = append(header, 0x80|127)
		for i := 7; i >= 0; i-- {
			header = append(header, byte(l>>(uint(i)*8)))
		}
	}
	header = append(header, mask[:]...)
	return append(header, masked...)
}

// providerScript returns the JavaScript to inject as window.ethereum.
func (p *PreloadInjector) providerScript() string {
	return fmt.Sprintf(`
(function() {
  if (window.ethereum && window.ethereum.__pethProvider) return;
  var accounts = [%q];
  var provider = {
    isMetaMask: true,
    selectedAddress: accounts[0],
    chainId: '0x1',
    networkVersion: '1',
    _events: {},
    request: function(args) {
      console.log('[peth]', args.method);
      switch (args.method) {
        case 'eth_requestAccounts':
        case 'eth_accounts':
          return Promise.resolve(accounts);
        case 'eth_chainId':
          return Promise.resolve(provider.chainId);
        case 'net_version':
          return Promise.resolve(provider.networkVersion);
        case 'personal_sign':
          return new Promise(function(res, rej) {
            window.__pethPendingSign = {
              message: args.params[0],
              resolve: res,
              reject: rej
            };
            console.log('[peth] personal_sign pending');
          });
        default:
          return Promise.reject(new Error('peth: unsupported: ' + args.method));
      }
    },
    on: function(e, cb) {
      if (!provider._events[e]) provider._events[e] = [];
      provider._events[e].push(cb);
      return provider;
    },
    removeListener: function(e, cb) { return provider; },
    emit: function(e, d) {
      (provider._events[e] || []).forEach(function(f) { f(d); });
    },
    _setAccounts: function(addrs) {
      accounts.length = 0;
      addrs.forEach(function(a) { accounts.push(a); });
      provider.selectedAddress = accounts[0] || null;
      provider.emit('accountsChanged', accounts);
    },
    _setChainId: function(id) {
      provider.chainId = id;
      provider.networkVersion = String(parseInt(id, 16));
      provider.emit('chainChanged', id);
    },
    __pethProvider: true
  };
  window.ethereum = provider;
  console.log('[peth] provider injected:', location.href);
})()`, p.walletAddr)
}
