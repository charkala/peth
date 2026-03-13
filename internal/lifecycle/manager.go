// Package lifecycle manages the Pinchtab subprocess lifecycle.
package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Commander abstracts process execution for testability.
type Commander interface {
	Start(ctx context.Context, binPath string, args []string) (*os.Process, error)
	Signal(p *os.Process, sig os.Signal) error
	Wait(p *os.Process) error
}

// ExecCommander is the real implementation using os/exec.
type ExecCommander struct{}

// Start launches a process.
func (e *ExecCommander) Start(_ context.Context, binPath string, args []string) (*os.Process, error) {
	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

// Signal sends a signal to a process.
func (e *ExecCommander) Signal(p *os.Process, sig os.Signal) error {
	return p.Signal(sig)
}

// Wait waits for a process to exit.
func (e *ExecCommander) Wait(p *os.Process) error {
	_, err := p.Wait()
	return err
}

// StartOpts configures how Pinchtab is launched.
type StartOpts struct {
	Headless   bool
	Port       int
	Profile    string
	BinPath    string
	Extensions []string // paths to unpacked Chrome extension directories
}

func (o StartOpts) toArgs() []string {
	var args []string
	if o.Headless {
		args = append(args, "--headless")
	}
	if o.Port != 0 {
		args = append(args, "--port", strconv.Itoa(o.Port))
	}
	if o.Profile != "" {
		args = append(args, "--profile", o.Profile)
	}
	return args
}

// Manager controls the Pinchtab subprocess.
type Manager struct {
	cmd     Commander
	process *os.Process
	PIDFile string // path to PID file for cross-process stop
}

// NewManager creates a Manager with the given Commander.
func NewManager(cmd Commander) *Manager {
	return &Manager{cmd: cmd}
}

// Start launches the Pinchtab subprocess.
func (m *Manager) Start(opts StartOpts) error {
	if m.process != nil {
		return fmt.Errorf("pinchtab is already running")
	}
	p, err := m.cmd.Start(context.Background(), opts.BinPath, opts.toArgs())
	if err != nil {
		return fmt.Errorf("failed to start pinchtab: %w", err)
	}
	m.process = p
	if m.PIDFile != "" {
		if err := os.WriteFile(m.PIDFile, []byte(strconv.Itoa(p.Pid)), 0600); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
	}
	return nil
}

// Stop sends SIGTERM to the Pinchtab subprocess and waits for exit.
// If no in-memory process is tracked, it reads the PID file written by Start.
func (m *Manager) Stop() error {
	if m.process == nil && m.PIDFile != "" {
		data, err := os.ReadFile(m.PIDFile)
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err == nil {
				p, _ := os.FindProcess(pid)
				// Verify the process is alive before adopting it.
				if p != nil && syscall.Kill(pid, 0) == nil {
					m.process = p
				}
			}
		}
	}
	if m.process == nil {
		// Clean up stale PID file if present.
		if m.PIDFile != "" {
			os.Remove(m.PIDFile)
		}
		return fmt.Errorf("pinchtab is not running")
	}
	sigErr := m.cmd.Signal(m.process, syscall.SIGTERM)
	if sigErr == nil {
		m.cmd.Wait(m.process)
	}
	m.process = nil
	if m.PIDFile != "" {
		os.Remove(m.PIDFile)
	}
	if sigErr != nil {
		return fmt.Errorf("failed to stop pinchtab: %w", sigErr)
	}
	return nil
}

// IsRunning returns true if the Pinchtab subprocess is tracked.
func (m *Manager) IsRunning() bool {
	return m.process != nil
}
