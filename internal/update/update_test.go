package update

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// mockDownloader records calls and returns canned responses.
type mockDownloader struct {
	data    []byte
	err     error
	calledURL string
}

func (m *mockDownloader) Download(url string) ([]byte, error) {
	m.calledURL = url
	return m.data, m.err
}

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		want         string
	}{
		{"darwin", "arm64", "peth-darwin-arm64"},
		{"darwin", "amd64", "peth-darwin-amd64"},
		{"linux", "amd64", "peth-linux-amd64"},
		{"linux", "arm64", "peth-linux-arm64"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.goarch, func(t *testing.T) {
			got := AssetName(tt.goos, tt.goarch)
			if got != tt.want {
				t.Errorf("AssetName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestDownloadURL(t *testing.T) {
	want := "https://github.com/charkala/peth/releases/latest/download/peth-darwin-arm64"
	got := DownloadURL("peth-darwin-arm64")
	if got != want {
		t.Errorf("DownloadURL = %q, want %q", got, want)
	}
}

func TestUpdaterSuccess(t *testing.T) {
	binContent := []byte("#!/bin/sh\necho updated")
	dl := &mockDownloader{data: binContent}

	target := t.TempDir() + "/peth"
	// Write an "old" binary.
	os.WriteFile(target, []byte("old"), 0755)

	u := &Updater{
		Downloader: dl,
		BinPath:    target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
	}

	err := u.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != string(binContent) {
		t.Errorf("binary content = %q, want %q", got, binContent)
	}

	info, _ := os.Stat(target)
	if perm := info.Mode().Perm(); perm&0111 == 0 {
		t.Error("expected binary to be executable")
	}

	wantURL := "https://github.com/charkala/peth/releases/latest/download/peth-darwin-arm64"
	if dl.calledURL != wantURL {
		t.Errorf("downloaded from %q, want %q", dl.calledURL, wantURL)
	}
}

func TestUpdaterDownloadFails(t *testing.T) {
	dl := &mockDownloader{err: fmt.Errorf("404 not found")}

	u := &Updater{
		Downloader: dl,
		BinPath:    t.TempDir() + "/peth",
		GOOS:       "linux",
		GOARCH:     "amd64",
	}

	err := u.Run()
	if err == nil {
		t.Fatal("expected error on download failure")
	}
}

func TestUpdaterReplacesAtomically(t *testing.T) {
	// Verifies the old binary remains if download succeeds but is then replaced.
	binContent := []byte("new-binary")
	dl := &mockDownloader{data: binContent}

	target := t.TempDir() + "/peth"
	os.WriteFile(target, []byte("old-binary"), 0755)

	u := &Updater{
		Downloader: dl,
		BinPath:    target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
	}

	if err := u.Run(); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "new-binary" {
		t.Errorf("binary = %q, want %q", got, "new-binary")
	}
}

func TestHintSudoPermissionError(t *testing.T) {
	err := fmt.Errorf("failed to create temp file: %w", os.ErrPermission)
	result := HintSudo(err)
	if result == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(result.Error(), "sudo peth update") {
		t.Errorf("expected sudo hint, got: %s", result.Error())
	}
}

func TestHintSudoOtherError(t *testing.T) {
	err := fmt.Errorf("network timeout")
	result := HintSudo(err)
	if result.Error() != err.Error() {
		t.Errorf("expected original error, got: %s", result.Error())
	}
}

func TestHintSudoNil(t *testing.T) {
	if HintSudo(nil) != nil {
		t.Error("expected nil for nil input")
	}
}
