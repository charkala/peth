package script

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// call records a mock method invocation.
type call struct {
	Method string
	Args   []string
}

type mockBrowser struct {
	calls      []call
	navErr     error
	snapResult string
	clickErr   error
	fillErr    error
	pressErr   error
	evalResult string
	evalErr    error
}

func (m *mockBrowser) Nav(url string) error {
	m.calls = append(m.calls, call{Method: "Nav", Args: []string{url}})
	return m.navErr
}

func (m *mockBrowser) Snap() (string, error) {
	m.calls = append(m.calls, call{Method: "Snap"})
	return m.snapResult, nil
}

func (m *mockBrowser) Click(ref string) error {
	m.calls = append(m.calls, call{Method: "Click", Args: []string{ref}})
	return m.clickErr
}

func (m *mockBrowser) Fill(ref, text string) error {
	m.calls = append(m.calls, call{Method: "Fill", Args: []string{ref, text}})
	return m.fillErr
}

func (m *mockBrowser) Press(key string) error {
	m.calls = append(m.calls, call{Method: "Press", Args: []string{key}})
	return m.pressErr
}

func (m *mockBrowser) Eval(js string) (string, error) {
	m.calls = append(m.calls, call{Method: "Eval", Args: []string{js}})
	return m.evalResult, m.evalErr
}

type mockWallet struct {
	addr string
	err  error
}

func (m *mockWallet) Active() (string, error) {
	return m.addr, m.err
}

type mockChain struct {
	calls     []call
	switchErr error
}

func (m *mockChain) Switch(nameOrID string) error {
	m.calls = append(m.calls, call{Method: "Switch", Args: []string{nameOrID}})
	return m.switchErr
}

func TestLoadYAML(t *testing.T) {
	yaml := []byte(`name: test-script
steps:
  - nav: https://example.com
  - click: ref:button
  - fill:
      ref: input
      text: hello
  - connect-wallet:
  - chain: optimism
  - wait: 5
  - eval: "document.title"
`)

	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	script, err := r.LoadYAML(yaml)
	if err != nil {
		t.Fatalf("LoadYAML() error: %v", err)
	}

	if script.Name != "test-script" {
		t.Errorf("Name = %q, want %q", script.Name, "test-script")
	}

	if len(script.Steps) != 7 {
		t.Fatalf("len(Steps) = %d, want 7", len(script.Steps))
	}

	tests := []struct {
		idx      int
		stepType string
	}{
		{0, "nav"},
		{1, "click"},
		{2, "fill"},
		{3, "connect-wallet"},
		{4, "chain"},
		{5, "wait"},
		{6, "eval"},
	}

	for _, tt := range tests {
		if script.Steps[tt.idx].Type != tt.stepType {
			t.Errorf("Steps[%d].Type = %q, want %q", tt.idx, script.Steps[tt.idx].Type, tt.stepType)
		}
	}

	// Verify specific step values.
	if script.Steps[0].Nav.URL != "https://example.com" {
		t.Errorf("nav URL = %q", script.Steps[0].Nav.URL)
	}
	if script.Steps[1].Click.Ref != "ref:button" {
		t.Errorf("click ref = %q", script.Steps[1].Click.Ref)
	}
	if script.Steps[2].Fill.Ref != "input" || script.Steps[2].Fill.Text != "hello" {
		t.Errorf("fill = {%q, %q}", script.Steps[2].Fill.Ref, script.Steps[2].Fill.Text)
	}
	if script.Steps[4].Chain.Name != "optimism" {
		t.Errorf("chain = %q", script.Steps[4].Chain.Name)
	}
	if script.Steps[5].Wait.Seconds != 5 {
		t.Errorf("wait = %d", script.Steps[5].Wait.Seconds)
	}
	if script.Steps[6].Eval.Expression != "document.title" {
		t.Errorf("eval = %q", script.Steps[6].Eval.Expression)
	}
}

func TestLoadYAMLInvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"missing name", "steps:\n  - nav: https://example.com\n"},
		{"invalid wait", "name: t\nsteps:\n  - wait: abc\n"},
		{"unknown step", "name: t\nsteps:\n  - bogus: value\n"},
	}

	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.LoadYAML([]byte(tt.yaml))
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestRunNavStep(t *testing.T) {
	browser := &mockBrowser{}
	r := NewRunner(browser, &mockWallet{}, &mockChain{})

	err := r.RunStep(Step{Type: "nav", Nav: NavStep{URL: "https://example.com"}})
	if err != nil {
		t.Fatalf("RunStep(nav) error: %v", err)
	}

	if len(browser.calls) != 1 || browser.calls[0].Method != "Nav" {
		t.Fatalf("expected Nav call, got %+v", browser.calls)
	}
	if browser.calls[0].Args[0] != "https://example.com" {
		t.Errorf("Nav URL = %q", browser.calls[0].Args[0])
	}
}

func TestRunClickStep(t *testing.T) {
	browser := &mockBrowser{}
	r := NewRunner(browser, &mockWallet{}, &mockChain{})

	err := r.RunStep(Step{Type: "click", Click: ClickStep{Ref: "ref:submit"}})
	if err != nil {
		t.Fatalf("RunStep(click) error: %v", err)
	}

	if len(browser.calls) != 1 || browser.calls[0].Method != "Click" {
		t.Fatalf("expected Click call, got %+v", browser.calls)
	}
	if browser.calls[0].Args[0] != "ref:submit" {
		t.Errorf("Click ref = %q", browser.calls[0].Args[0])
	}
}

func TestRunFillStep(t *testing.T) {
	browser := &mockBrowser{}
	r := NewRunner(browser, &mockWallet{}, &mockChain{})

	err := r.RunStep(Step{Type: "fill", Fill: FillStep{Ref: "ref:input", Text: "hello"}})
	if err != nil {
		t.Fatalf("RunStep(fill) error: %v", err)
	}

	if len(browser.calls) != 1 || browser.calls[0].Method != "Fill" {
		t.Fatalf("expected Fill call, got %+v", browser.calls)
	}
	if browser.calls[0].Args[0] != "ref:input" || browser.calls[0].Args[1] != "hello" {
		t.Errorf("Fill args = %+v", browser.calls[0].Args)
	}
}

func TestRunConnectWallet(t *testing.T) {
	wallet := &mockWallet{addr: "0xabc123"}
	r := NewRunner(&mockBrowser{}, wallet, &mockChain{})

	err := r.RunStep(Step{Type: "connect-wallet"})
	if err != nil {
		t.Fatalf("RunStep(connect-wallet) error: %v", err)
	}
}

func TestRunConnectWalletNoActive(t *testing.T) {
	wallet := &mockWallet{err: fmt.Errorf("no active wallet")}
	r := NewRunner(&mockBrowser{}, wallet, &mockChain{})

	err := r.RunStep(Step{Type: "connect-wallet"})
	if err == nil {
		t.Fatal("expected error when no active wallet")
	}
}

func TestRunChainSwitch(t *testing.T) {
	ch := &mockChain{}
	r := NewRunner(&mockBrowser{}, &mockWallet{addr: "0x1"}, ch)

	err := r.RunStep(Step{Type: "chain", Chain: ChainStep{Name: "optimism"}})
	if err != nil {
		t.Fatalf("RunStep(chain) error: %v", err)
	}

	if len(ch.calls) != 1 || ch.calls[0].Args[0] != "optimism" {
		t.Errorf("expected Switch(optimism), got %+v", ch.calls)
	}
}

func TestRunWaitStep(t *testing.T) {
	var sleptDuration time.Duration
	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	r.sleepFunc = func(d time.Duration) {
		sleptDuration = d
	}

	err := r.RunStep(Step{Type: "wait", Wait: WaitStep{Seconds: 3}})
	if err != nil {
		t.Fatalf("RunStep(wait) error: %v", err)
	}

	if sleptDuration != 3*time.Second {
		t.Errorf("slept %v, want 3s", sleptDuration)
	}
}

func TestRunFullScript(t *testing.T) {
	browser := &mockBrowser{}
	ch := &mockChain{}
	wallet := &mockWallet{addr: "0xabc"}

	r := NewRunner(browser, wallet, ch)
	r.sleepFunc = func(d time.Duration) {} // no-op for tests

	script := &Script{
		Name: "full-test",
		Steps: []Step{
			{Type: "nav", Nav: NavStep{URL: "https://app.example.com"}},
			{Type: "click", Click: ClickStep{Ref: "ref:connect"}},
			{Type: "connect-wallet"},
			{Type: "chain", Chain: ChainStep{Name: "optimism"}},
			{Type: "fill", Fill: FillStep{Ref: "ref:amount", Text: "1.0"}},
			{Type: "wait", Wait: WaitStep{Seconds: 1}},
			{Type: "click", Click: ClickStep{Ref: "ref:swap"}},
		},
	}

	if err := r.Run(script); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify browser calls in order.
	expectedMethods := []string{"Nav", "Click", "Fill", "Click"}
	var browserMethods []string
	for _, c := range browser.calls {
		browserMethods = append(browserMethods, c.Method)
	}

	if len(browserMethods) != len(expectedMethods) {
		t.Fatalf("browser calls = %v, want %v", browserMethods, expectedMethods)
	}
	for i, m := range expectedMethods {
		if browserMethods[i] != m {
			t.Errorf("browser call[%d] = %q, want %q", i, browserMethods[i], m)
		}
	}

	// Verify chain switch.
	if len(ch.calls) != 1 || ch.calls[0].Args[0] != "optimism" {
		t.Errorf("chain calls = %+v", ch.calls)
	}
}

func TestRunStepError(t *testing.T) {
	browser := &mockBrowser{
		clickErr: fmt.Errorf("element not found"),
	}

	r := NewRunner(browser, &mockWallet{addr: "0x1"}, &mockChain{})

	script := &Script{
		Name: "error-test",
		Steps: []Step{
			{Type: "nav", Nav: NavStep{URL: "https://example.com"}},
			{Type: "click", Click: ClickStep{Ref: "ref:missing"}},
			{Type: "nav", Nav: NavStep{URL: "https://should-not-reach.com"}},
		},
	}

	err := r.Run(script)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "step 2") {
		t.Errorf("error should mention step 2: %v", err)
	}

	// Only 2 browser calls: Nav + failed Click. Third step should not run.
	if len(browser.calls) != 2 {
		t.Errorf("expected 2 browser calls, got %d: %+v", len(browser.calls), browser.calls)
	}
}

func TestRunUnknownStepType(t *testing.T) {
	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	err := r.RunStep(Step{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown step type")
	}
}

func TestLoadYAMLWithComments(t *testing.T) {
	yaml := []byte(`# This is a comment
name: commented-script
steps:
  # Navigate first
  - nav: https://example.com
  - click: ref:btn
`)

	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	script, err := r.LoadYAML(yaml)
	if err != nil {
		t.Fatalf("LoadYAML() error: %v", err)
	}

	if len(script.Steps) != 2 {
		t.Errorf("len(Steps) = %d, want 2", len(script.Steps))
	}
}

func TestLoadYAMLPressStep(t *testing.T) {
	yaml := []byte(`name: press-test
steps:
  - press: Enter
`)

	r := NewRunner(&mockBrowser{}, &mockWallet{}, &mockChain{})
	script, err := r.LoadYAML(yaml)
	if err != nil {
		t.Fatalf("LoadYAML() error: %v", err)
	}

	if len(script.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(script.Steps))
	}
	if script.Steps[0].Type != "press" || script.Steps[0].Press.Key != "Enter" {
		t.Errorf("step = %+v", script.Steps[0])
	}
}
