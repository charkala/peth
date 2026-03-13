// Package update provides self-update functionality for the peth binary.
package update

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const repo = "charkala/peth"

// Downloader abstracts binary download for testability.
type Downloader interface {
	Download(url string) ([]byte, error)
}

// HTTPDownloader downloads via HTTP GET.
type HTTPDownloader struct{}

// Download fetches the given URL and returns the response body.
func (h *HTTPDownloader) Download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// AssetName returns the release asset name for the given OS and architecture.
func AssetName(goos, goarch string) string {
	return fmt.Sprintf("peth-%s-%s", goos, goarch)
}

// DownloadURL returns the GitHub release download URL for the given asset.
func DownloadURL(asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, asset)
}

// Updater handles self-updating the peth binary.
type Updater struct {
	Downloader Downloader
	BinPath    string // path to the binary to replace
	GOOS       string
	GOARCH     string
}

// NewUpdater creates an Updater configured for the current platform.
func NewUpdater() (*Updater, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve executable path: %w", err)
	}
	return &Updater{
		Downloader: &HTTPDownloader{},
		BinPath:    exe,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	}, nil
}

// Run downloads the latest release and replaces the current binary.
func (u *Updater) Run() error {
	asset := AssetName(u.GOOS, u.GOARCH)
	url := DownloadURL(asset)

	data, err := u.Downloader.Download(url)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	// Write to a temp file in the same directory for atomic rename.
	dir := filepath.Dir(u.BinPath)
	tmp, err := os.CreateTemp(dir, "peth-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write update: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpPath, u.BinPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// HintSudo wraps a permission error with a suggestion to use sudo.
func HintSudo(err error) error {
	if err != nil && errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w\n\nTry running: sudo peth update", err)
	}
	return err
}
