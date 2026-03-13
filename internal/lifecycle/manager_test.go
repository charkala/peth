package lifecycle

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// mockCommander records calls and simulates command execution.
type mockCommander struct {
	started    bool
	stopped    bool
	startErr   error
	waitErr    error
	signalErr  error
	pid        int
	processSet bool
}

func (m *mockCommander) Start(ctx context.Context, binPath string, args []string) (*os.Process, error) {
	if m.startErr != nil {
		return nil, m.startErr
	}
	m.started = true
	// Return a fake process — we use pid 0 which won't match a real process.
	// We just need a non-nil *os.Process for the manager to track.
	return &os.Process{Pid: m.pid}, nil
}

func (m *mockCommander) Signal(p *os.Process, sig os.Signal) error {
	m.stopped = true
	return m.signalErr
}

func (m *mockCommander) Wait(p *os.Process) error {
	return m.waitErr
}

func TestStartOpts(t *testing.T) {
	opts := StartOpts{
		Headless: true,
		Port:     9867,
		Profile:  "default",
		BinPath:  "/usr/local/bin/pinchtab",
	}

	args := opts.toArgs()

	wantContains := map[string]bool{
		"--headless": false,
		"--port":     false,
		"9867":       false,
		"--profile":  false,
		"default":    false,
	}

	for _, a := range args {
		if _, ok := wantContains[a]; ok {
			wantContains[a] = true
		}
	}

	for k, found := range wantContains {
		if !found {
			t.Errorf("expected arg %q in %v", k, args)
		}
	}
}

func TestStartOptsMinimal(t *testing.T) {
	opts := StartOpts{
		Port:    0,
		BinPath: "pinchtab",
	}
	args := opts.toArgs()
	for _, a := range args {
		if a == "--headless" {
			t.Error("headless flag should not be set when Headless is false")
		}
		if a == "--profile" {
			t.Error("profile flag should not be set when Profile is empty")
		}
	}
}

func TestManagerStart(t *testing.T) {
	tests := []struct {
		name     string
		startErr error
		wantErr  bool
	}{
		{"success", nil, false},
		{"start fails", fmt.Errorf("exec: not found"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommander{startErr: tt.startErr, pid: 99999}
			m := NewManager(mock)

			err := m.Start(StartOpts{BinPath: "pinchtab"})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !m.IsRunning() {
				t.Error("expected IsRunning() == true after Start")
			}
		})
	}
}

func TestManagerStartAlreadyRunning(t *testing.T) {
	mock := &mockCommander{pid: 99999}
	m := NewManager(mock)

	if err := m.Start(StartOpts{BinPath: "pinchtab"}); err != nil {
		t.Fatal(err)
	}

	err := m.Start(StartOpts{BinPath: "pinchtab"})
	if err == nil {
		t.Fatal("expected error when starting while already running")
	}
}

func TestManagerStop(t *testing.T) {
	tests := []struct {
		name      string
		signalErr error
		wantErr   bool
	}{
		{"success", nil, false},
		{"signal fails", fmt.Errorf("permission denied"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCommander{pid: 99999, signalErr: tt.signalErr}
			m := NewManager(mock)

			if err := m.Start(StartOpts{BinPath: "pinchtab"}); err != nil {
				t.Fatal(err)
			}

			err := m.Stop()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.IsRunning() {
				t.Error("expected IsRunning() == false after Stop")
			}
		})
	}
}

func TestManagerStopNotRunning(t *testing.T) {
	mock := &mockCommander{}
	m := NewManager(mock)

	err := m.Stop()
	if err == nil {
		t.Fatal("expected error when stopping while not running")
	}
}

func TestManagerIsRunning(t *testing.T) {
	mock := &mockCommander{pid: 99999}
	m := NewManager(mock)

	if m.IsRunning() {
		t.Error("expected IsRunning() == false before Start")
	}

	m.Start(StartOpts{BinPath: "pinchtab"})

	if !m.IsRunning() {
		t.Error("expected IsRunning() == true after Start")
	}
}
