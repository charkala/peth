// Package client provides an HTTP client for the Pinchtab browser automation API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Pinchtab defines the interface for interacting with a Pinchtab instance.
type Pinchtab interface {
	Health() (*HealthResponse, error)
	Nav(url string) error
	Snap() (string, error)
	Eval(expression string) (string, error)
	Click(ref string) error
	Fill(ref, text string) error
	Press(key string) error
}

// HealthResponse represents the JSON response from the /health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Option is a functional option for configuring HTTPClient.
type Option func(*HTTPClient)

// WithToken sets the bearer token for authentication.
func WithToken(token string) Option {
	return func(c *HTTPClient) {
		c.token = token
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *HTTPClient) {
		c.httpClient = hc
	}
}

// HTTPClient implements the Pinchtab interface over HTTP.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a new HTTPClient for the given Pinchtab base URL.
func New(baseURL string, opts ...Option) *HTTPClient {
	c := &HTTPClient{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *HTTPClient) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *HTTPClient) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("pinchtab: HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func (c *HTTPClient) postJSON(path string, payload any) ([]byte, error) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(http.MethodPost, path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Health calls GET /health and returns the parsed response.
func (c *HTTPClient) Health() (*HealthResponse, error) {
	req, err := c.newRequest(http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var hr HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, fmt.Errorf("pinchtab: invalid health response: %w", err)
	}
	return &hr, nil
}

// Nav calls POST /navigate with the given URL.
func (c *HTTPClient) Nav(url string) error {
	_, err := c.postJSON("/navigate", map[string]string{"url": url})
	return err
}

// Snap calls GET /snapshot and returns the raw JSON accessibility tree.
func (c *HTTPClient) Snap() (string, error) {
	req, err := c.newRequest(http.MethodGet, "/snapshot", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("pinchtab: failed to read snapshot response: %w", err)
	}
	return string(body), nil
}

// Click calls POST /action with kind "click" for the given element reference.
func (c *HTTPClient) Click(ref string) error {
	_, err := c.postJSON("/action", map[string]string{"kind": "click", "ref": ref})
	return err
}

// Fill calls POST /action with kind "fill" for the given element reference and text.
func (c *HTTPClient) Fill(ref, text string) error {
	_, err := c.postJSON("/action", map[string]string{"kind": "fill", "ref": ref, "text": text})
	return err
}

// Press calls POST /action with kind "press" for the given key.
func (c *HTTPClient) Press(key string) error {
	_, err := c.postJSON("/action", map[string]string{"kind": "press", "key": key})
	return err
}

// Eval calls POST /evaluate with the given JavaScript expression.
func (c *HTTPClient) Eval(expression string) (string, error) {
	data, err := c.postJSON("/evaluate", map[string]string{"expression": expression})
	if err != nil {
		return "", err
	}
	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("pinchtab: invalid eval response: %w", err)
	}
	return result.Result, nil
}
