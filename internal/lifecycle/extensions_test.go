package lifecycle

import (
	"strings"
	"testing"
)

func TestChromeFlagsHeadless(t *testing.T) {
	opts := StartOpts{Headless: true}
	flags := ChromeFlags(opts)

	found := false
	for _, f := range flags {
		if f == "--headless=new" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --headless=new in flags %v", flags)
	}
}

func TestChromeFlagsExtensions(t *testing.T) {
	opts := StartOpts{
		Extensions: []string{"/path/to/ext1", "/path/to/ext2"},
	}
	flags := ChromeFlags(opts)

	wantDisable := "--disable-extensions-except=/path/to/ext1,/path/to/ext2"
	wantLoad := "--load-extension=/path/to/ext1,/path/to/ext2"

	var foundDisable, foundLoad bool
	for _, f := range flags {
		if f == wantDisable {
			foundDisable = true
		}
		if f == wantLoad {
			foundLoad = true
		}
	}

	if !foundDisable {
		t.Errorf("expected %q in flags %v", wantDisable, flags)
	}
	if !foundLoad {
		t.Errorf("expected %q in flags %v", wantLoad, flags)
	}
}

func TestChromeFlagsProfile(t *testing.T) {
	opts := StartOpts{Profile: "/tmp/chrome-profile"}
	flags := ChromeFlags(opts)

	found := false
	for _, f := range flags {
		if f == "--user-data-dir=/tmp/chrome-profile" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --user-data-dir=/tmp/chrome-profile in flags %v", flags)
	}
}

func TestChromeFlagsCombined(t *testing.T) {
	opts := StartOpts{
		Headless:   true,
		Port:       9222,
		Profile:    "/tmp/profile",
		Extensions: []string{"/ext/metamask"},
	}
	flags := ChromeFlags(opts)

	want := map[string]bool{
		"--headless=new":                          false,
		"--remote-debugging-port=9222":            false,
		"--user-data-dir=/tmp/profile":            false,
		"--disable-extensions-except=/ext/metamask": false,
		"--load-extension=/ext/metamask":           false,
	}

	for _, f := range flags {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}

	for flag, found := range want {
		if !found {
			t.Errorf("expected flag %q in %v", flag, flags)
		}
	}
}

func TestChromeFlagsNoExtensions(t *testing.T) {
	opts := StartOpts{}
	flags := ChromeFlags(opts)

	for _, f := range flags {
		if strings.HasPrefix(f, "--disable-extensions-except") ||
			strings.HasPrefix(f, "--load-extension") {
			t.Errorf("unexpected extension flag %q when no extensions set", f)
		}
	}
}

func TestChromeFlagsNoPort(t *testing.T) {
	opts := StartOpts{}
	flags := ChromeFlags(opts)

	for _, f := range flags {
		if strings.HasPrefix(f, "--remote-debugging-port") {
			t.Errorf("unexpected port flag %q when port is 0", f)
		}
	}
}
