// Package script provides a YAML-driven dApp automation script runner.
package script

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ScriptBrowser defines browser automation methods needed by the runner.
type ScriptBrowser interface {
	Nav(url string) error
	Snap() (string, error)
	Click(ref string) error
	Fill(ref, text string) error
	Press(key string) error
	Eval(js string) (string, error)
}

// ScriptWallet provides the active wallet address.
type ScriptWallet interface {
	Active() (string, error)
}

// ScriptChain provides chain switching.
type ScriptChain interface {
	Switch(nameOrID string) error
}

// Step represents a single automation step with a discriminated union approach.
type Step struct {
	Type          string
	Nav           NavStep
	Click         ClickStep
	Fill          FillStep
	Press         PressStep
	Eval          EvalStep
	ConnectWallet ConnectWalletStep
	Chain         ChainStep
	ApproveTx     ApproveTxStep
	Wait          WaitStep
	Assert        AssertStep
}

// NavStep navigates to a URL.
type NavStep struct {
	URL string
}

// ClickStep clicks an element by reference.
type ClickStep struct {
	Ref string
}

// FillStep fills an input element.
type FillStep struct {
	Ref  string
	Text string
}

// PressStep presses a keyboard key.
type PressStep struct {
	Key string
}

// EvalStep evaluates JavaScript.
type EvalStep struct {
	Expression string
}

// ConnectWalletStep triggers wallet connection (no fields needed).
type ConnectWalletStep struct{}

// ChainStep switches the chain.
type ChainStep struct {
	Name string
}

// ApproveTxStep approves a transaction.
type ApproveTxStep struct {
	MaxGas string
}

// WaitStep waits for a duration.
type WaitStep struct {
	Seconds int
}

// AssertStep performs an assertion.
type AssertStep struct {
	Type string
	Args map[string]string
}

// Script represents a named sequence of automation steps.
type Script struct {
	Name  string
	Steps []Step
}

// Runner executes automation scripts.
type Runner struct {
	browser ScriptBrowser
	wallet  ScriptWallet
	chain   ScriptChain
	// sleepFunc allows tests to override time.Sleep.
	sleepFunc func(time.Duration)
}

// NewRunner creates a new Runner with the given dependencies.
func NewRunner(browser ScriptBrowser, wallet ScriptWallet, chain ScriptChain) *Runner {
	return &Runner{
		browser:   browser,
		wallet:    wallet,
		chain:     chain,
		sleepFunc: time.Sleep,
	}
}

// LoadFile reads and parses a YAML script from a file path.
func (r *Runner) LoadFile(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read script file: %w", err)
	}
	return r.LoadYAML(data)
}

// LoadYAML parses YAML bytes into a Script.
func (r *Runner) LoadYAML(data []byte) (*Script, error) {
	return parseYAML(data)
}

// Run executes all steps in a script sequentially. Stops on first error.
func (r *Runner) Run(script *Script) error {
	for i, step := range script.Steps {
		if err := r.RunStep(step); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Type, err)
		}
	}
	return nil
}

// RunStep executes a single step.
func (r *Runner) RunStep(step Step) error {
	switch step.Type {
	case "nav":
		return r.browser.Nav(step.Nav.URL)
	case "snap":
		_, err := r.browser.Snap()
		return err
	case "click":
		return r.browser.Click(step.Click.Ref)
	case "fill":
		return r.browser.Fill(step.Fill.Ref, step.Fill.Text)
	case "press":
		return r.browser.Press(step.Press.Key)
	case "eval":
		_, err := r.browser.Eval(step.Eval.Expression)
		return err
	case "connect-wallet":
		// Delegate to wallet + browser to perform connect flow.
		addr, err := r.wallet.Active()
		if err != nil {
			return fmt.Errorf("get active wallet: %w", err)
		}
		if addr == "" {
			return fmt.Errorf("no active wallet")
		}
		return nil
	case "chain":
		return r.chain.Switch(step.Chain.Name)
	case "approve-tx":
		// TODO: Implement transaction approval flow.
		return nil
	case "wait":
		r.sleepFunc(time.Duration(step.Wait.Seconds) * time.Second)
		return nil
	case "assert":
		// TODO: Implement assertion logic.
		return nil
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// --- Minimal YAML parser ---

// parseYAML implements a minimal YAML parser for the script format.
// It handles the specific structure used by peth scripts without external deps.
func parseYAML(data []byte) (*Script, error) {
	lines := strings.Split(string(data), "\n")
	script := &Script{}

	var inSteps bool
	var i int

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		if !inSteps {
			// Parse top-level keys.
			if strings.HasPrefix(trimmed, "name:") {
				script.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				i++
				continue
			}
			if trimmed == "steps:" {
				inSteps = true
				i++
				continue
			}
			i++
			continue
		}

		// Parse steps (lines starting with "- ").
		if !strings.HasPrefix(trimmed, "- ") {
			i++
			continue
		}

		stepContent := strings.TrimPrefix(trimmed, "- ")

		step, consumed, err := parseStep(stepContent, lines, i)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		script.Steps = append(script.Steps, step)
		i += consumed
	}

	if script.Name == "" {
		return nil, fmt.Errorf("script missing 'name' field")
	}

	return script, nil
}

// parseStep parses a single step from the content after "- ".
// Returns the step, number of lines consumed, and any error.
func parseStep(content string, lines []string, startLine int) (Step, int, error) {
	// Handle simple key: value steps (e.g., "nav: https://example.com").
	if idx := strings.Index(content, ":"); idx >= 0 {
		key := strings.TrimSpace(content[:idx])
		value := strings.TrimSpace(content[idx+1:])

		switch key {
		case "nav":
			return Step{Type: "nav", Nav: NavStep{URL: value}}, 1, nil
		case "snap":
			return Step{Type: "snap"}, 1, nil
		case "click":
			return Step{Type: "click", Click: ClickStep{Ref: value}}, 1, nil
		case "press":
			return Step{Type: "press", Press: PressStep{Key: value}}, 1, nil
		case "eval":
			// Handle quoted strings.
			value = unquote(value)
			return Step{Type: "eval", Eval: EvalStep{Expression: value}}, 1, nil
		case "connect-wallet":
			return Step{Type: "connect-wallet"}, 1, nil
		case "chain":
			return Step{Type: "chain", Chain: ChainStep{Name: value}}, 1, nil
		case "wait":
			secs, err := strconv.Atoi(value)
			if err != nil {
				return Step{}, 0, fmt.Errorf("invalid wait seconds: %s", value)
			}
			return Step{Type: "wait", Wait: WaitStep{Seconds: secs}}, 1, nil
		case "fill":
			// Fill can be a simple "fill:" or a block with sub-keys.
			if value == "" {
				// Block format: parse indented sub-keys.
				return parseFillBlock(lines, startLine)
			}
			return Step{}, 0, fmt.Errorf("fill step requires block format with ref and text")
		case "approve-tx":
			if value == "" {
				return parseApproveTxBlock(lines, startLine)
			}
			return Step{Type: "approve-tx", ApproveTx: ApproveTxStep{MaxGas: value}}, 1, nil
		case "assert":
			if value == "" {
				return parseAssertBlock(lines, startLine)
			}
			return Step{}, 0, fmt.Errorf("assert step requires block format")
		default:
			return Step{}, 0, fmt.Errorf("unknown step type: %s", key)
		}
	}

	return Step{}, 0, fmt.Errorf("invalid step format: %s", content)
}

// parseFillBlock parses a fill step with indented ref and text sub-keys.
func parseFillBlock(lines []string, startLine int) (Step, int, error) {
	step := Step{Type: "fill"}
	consumed := 1

	for j := startLine + 1; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)

		// Stop at empty lines, comments, or new steps.
		if trimmed == "" || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "#") {
			break
		}

		// Must be indented more than the parent "- fill:".
		if !isIndented(line, lines[startLine]) {
			break
		}

		if idx := strings.Index(trimmed, ":"); idx >= 0 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+1:])
			value = unquote(value)
			switch key {
			case "ref":
				step.Fill.Ref = value
			case "text":
				step.Fill.Text = value
			}
		}
		consumed++
	}

	if step.Fill.Ref == "" {
		return Step{}, 0, fmt.Errorf("fill step missing 'ref'")
	}

	return step, consumed, nil
}

// parseApproveTxBlock parses an approve-tx step with sub-keys.
func parseApproveTxBlock(lines []string, startLine int) (Step, int, error) {
	step := Step{Type: "approve-tx"}
	consumed := 1

	for j := startLine + 1; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "- ") {
			break
		}

		if !isIndented(line, lines[startLine]) {
			break
		}

		if idx := strings.Index(trimmed, ":"); idx >= 0 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+1:])
			value = unquote(value)
			switch key {
			case "max-gas":
				step.ApproveTx.MaxGas = value
			}
		}
		consumed++
	}

	return step, consumed, nil
}

// parseAssertBlock parses an assert step with sub-keys.
func parseAssertBlock(lines []string, startLine int) (Step, int, error) {
	step := Step{Type: "assert", Assert: AssertStep{Args: make(map[string]string)}}
	consumed := 1

	for j := startLine + 1; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "- ") {
			break
		}

		if !isIndented(line, lines[startLine]) {
			break
		}

		if idx := strings.Index(trimmed, ":"); idx >= 0 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+1:])
			value = unquote(value)
			switch key {
			case "type":
				step.Assert.Type = value
			default:
				step.Assert.Args[key] = value
			}
		}
		consumed++
	}

	return step, consumed, nil
}

// isIndented returns true if line is more indented than parent.
func isIndented(line, parent string) bool {
	lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
	parentIndent := len(parent) - len(strings.TrimLeft(parent, " \t"))
	return lineIndent > parentIndent
}

// unquote removes surrounding quotes (single or double) from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
