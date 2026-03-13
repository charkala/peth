package main

import (
	"fmt"
	"os"
	"os/exec"
)

// passthroughFunc is the signature for executing a pinchtab command.
// It receives the command name and arguments, and returns an error if the command fails.
type passthroughFunc func(command string, args []string) error

// exitError wraps an exit code from a failed pinchtab command.
type exitError struct {
	code int
	cmd  string
}

func (e *exitError) Error() string {
	return fmt.Sprintf("pinchtab %s exited with code %d", e.cmd, e.code)
}

// isPinchtabCommand returns true if the command should be passed through to pinchtab.
func isPinchtabCommand(cmd string) bool {
	switch cmd {
	case "nav", "snap", "click", "type", "press", "fill",
		"hover", "scroll", "select", "focus", "text",
		"tabs", "ss", "eval", "pdf", "health", "quick":
		return true
	}
	return false
}

// execPassthrough executes a pinchtab command by shelling out to the pinchtab binary.
// It inherits stdin, stdout, and stderr from the parent process.
func execPassthrough(command string, args []string) error {
	allArgs := append([]string{command}, args...)
	cmd := exec.Command("pinchtab", allArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &exitError{code: exitErr.ExitCode(), cmd: command}
		}
		return fmt.Errorf("pinchtab: %w", err)
	}
	return nil
}
