package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charkala/peth/testutil"
)

func TestNew(t *testing.T) {
	c := New("http://localhost:9867")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "http://localhost:9867" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost:9867")
	}
}

func TestNewWithOptions(t *testing.T) {
	custom := &http.Client{}
	c := New("http://localhost:9867",
		WithToken("secret"),
		WithHTTPClient(custom),
	)
	if c.token != "secret" {
		t.Errorf("token = %q, want %q", c.token, "secret")
	}
	if c.httpClient != custom {
		t.Error("expected custom HTTP client to be set")
	}
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantStatus string
		wantVer    string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"status":"ok","version":"1.2.3"}`,
			wantErr:    false,
			wantStatus: "ok",
			wantVer:    "1.2.3",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"internal"}`,
			wantErr:    true,
		},
		{
			name:       "invalid JSON",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewTestServer(t, tt.statusCode, tt.body)
			c := New(srv.URL)

			resp, err := c.Health()
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, resp.Status, tt.wantStatus)
			testutil.AssertEqual(t, resp.Version, tt.wantVer)
		})
	}
}

func TestHealthConnectionRefused(t *testing.T) {
	c := New("http://127.0.0.1:1") // nothing listening
	_, err := c.Health()
	testutil.AssertError(t, err)
}

func TestHealthAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthResponse{Status: "ok", Version: "1.0.0"})
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, WithToken("mytoken"))
	_, err := c.Health()
	testutil.AssertNoError(t, err)

	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestNav(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		url        string
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"ok":true}`,
			url:        "https://example.com",
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"fail"}`,
			url:        "https://example.com",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/navigate") {
					t.Errorf("path = %s, want /navigate", r.URL.Path)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			err := c.Nav(tt.url)
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			if gotBody["url"] != tt.url {
				t.Errorf("sent url = %q, want %q", gotBody["url"], tt.url)
			}
		})
	}
}

func TestSnap(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantSnap   string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"count":2,"nodes":[{"ref":"e0","role":"button","name":"Click me"}],"title":"Test","url":"https://example.com"}`,
			wantErr:    false,
			wantSnap:   `{"count":2,"nodes":[{"ref":"e0","role":"button","name":"Click me"}],"title":"Test","url":"https://example.com"}`,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/snapshot") {
					t.Errorf("path = %s, want /snapshot", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			snap, err := c.Snap()
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, snap, tt.wantSnap)
		})
	}
}

func TestClick(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		ref        string
		wantErr    bool
	}{
		{"success", http.StatusOK, "btn1", false},
		{"server error", http.StatusInternalServerError, "btn1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/action") {
					t.Errorf("path = %s, want /action", r.URL.Path)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"success":true,"result":{"clicked":true}}`))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			err := c.Click(tt.ref)
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			if gotBody["kind"] != "click" {
				t.Errorf("sent kind = %q, want %q", gotBody["kind"], "click")
			}
			if gotBody["ref"] != tt.ref {
				t.Errorf("sent ref = %q, want %q", gotBody["ref"], tt.ref)
			}
		})
	}
}

func TestFill(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		ref, text  string
		wantErr    bool
	}{
		{"success", http.StatusOK, "input1", "hello", false},
		{"server error", http.StatusInternalServerError, "input1", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/action") {
					t.Errorf("path = %s, want /action", r.URL.Path)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"success":true}`))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			err := c.Fill(tt.ref, tt.text)
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			if gotBody["kind"] != "fill" {
				t.Errorf("sent kind = %q, want %q", gotBody["kind"], "fill")
			}
			if gotBody["ref"] != tt.ref {
				t.Errorf("sent ref = %q, want %q", gotBody["ref"], tt.ref)
			}
			if gotBody["text"] != tt.text {
				t.Errorf("sent text = %q, want %q", gotBody["text"], tt.text)
			}
		})
	}
}

func TestPress(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		key        string
		wantErr    bool
	}{
		{"success", http.StatusOK, "Enter", false},
		{"server error", http.StatusInternalServerError, "Enter", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/action") {
					t.Errorf("path = %s, want /action", r.URL.Path)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"success":true}`))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			err := c.Press(tt.key)
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			if gotBody["kind"] != "press" {
				t.Errorf("sent kind = %q, want %q", gotBody["kind"], "press")
			}
			if gotBody["key"] != tt.key {
				t.Errorf("sent key = %q, want %q", gotBody["key"], tt.key)
			}
		})
	}
}

func TestEval(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expr       string
		wantErr    bool
		wantResult string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"result":"42"}`,
			expr:       "1+1",
			wantErr:    false,
			wantResult: "42",
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{}`,
			expr:       "bad()",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method = %s, want POST", r.Method)
				}
				json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			c := New(srv.URL)
			result, err := c.Eval(tt.expr)
			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}
			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, result, tt.wantResult)
			if gotBody["expression"] != tt.expr {
				t.Errorf("sent expression = %q, want %q", gotBody["expression"], tt.expr)
			}
		})
	}
}
