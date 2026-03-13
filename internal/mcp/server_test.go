package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerToolsList(t *testing.T) {
	s := NewServer(0)
	s.RegisterTool(Tool{
		Name:        "nav",
		Description: "Navigate to URL",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{"type": "string"},
			},
		},
	})
	s.RegisterTool(Tool{
		Name:        "snap",
		Description: "Take accessibility snapshot",
	})

	body := mustJSON(t, map[string]interface{}{
		"method": "tools/list",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Result.Tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(resp.Result.Tools))
	}
	if resp.Result.Tools[0].Name != "nav" {
		t.Errorf("first tool = %q, want %q", resp.Result.Tools[0].Name, "nav")
	}
	if resp.Result.Tools[1].Name != "snap" {
		t.Errorf("second tool = %q, want %q", resp.Result.Tools[1].Name, "snap")
	}
}

func TestServerToolsCall(t *testing.T) {
	s := NewServer(0)
	called := false
	s.RegisterTool(Tool{
		Name:        "nav",
		Description: "Navigate to URL",
		Handler: func(params map[string]interface{}) (interface{}, error) {
			called = true
			url, _ := params["url"].(string)
			return map[string]string{"navigated": url}, nil
		},
	})

	body := mustJSON(t, map[string]interface{}{
		"method": "tools/call",
		"params": map[string]interface{}{
			"name":      "nav",
			"arguments": map[string]interface{}{"url": "https://example.com"},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !called {
		t.Error("handler was not called")
	}

	var resp struct {
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Result["navigated"] != "https://example.com" {
		t.Errorf("navigated = %q, want %q", resp.Result["navigated"], "https://example.com")
	}
}

func TestServerToolsCallNotFound(t *testing.T) {
	s := NewServer(0)

	body := mustJSON(t, map[string]interface{}{
		"method": "tools/call",
		"params": map[string]interface{}{
			"name":      "nonexistent",
			"arguments": map[string]interface{}{},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	s.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for unknown tool")
	}
	if resp.Error.Code != -1 {
		t.Errorf("error code = %d, want -1", resp.Error.Code)
	}
}

func TestServerToolsCallInvalidParams(t *testing.T) {
	s := NewServer(0)
	s.RegisterTool(Tool{
		Name:        "nav",
		Description: "Navigate to URL",
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	})

	// Missing params entirely
	body := mustJSON(t, map[string]interface{}{
		"method": "tools/call",
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	s.handler().ServeHTTP(rr, req)

	var resp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for missing params")
	}
}

func TestServerStartStop(t *testing.T) {
	s := NewServer(0) // port 0 = let OS pick
	s.RegisterTool(Tool{
		Name:        "ping",
		Description: "Health check",
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return "pong", nil
		},
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	// Verify server is responding
	addr := s.Addr()
	body := mustJSON(t, map[string]interface{}{
		"method": "tools/list",
	})

	resp, err := http.Post("http://"+addr+"/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Stop and verify
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
