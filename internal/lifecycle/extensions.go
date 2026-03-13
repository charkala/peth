package lifecycle

import (
	"fmt"
	"strings"
)

// ExtensionOpts holds configuration for Chrome extensions.
type ExtensionOpts struct {
	ExtensionPaths []string // paths to unpacked extension directories
}

// ChromeFlags builds Chrome command-line flags from StartOpts.
// These flags are used to configure Chrome when launching via Pinchtab.
func ChromeFlags(opts StartOpts) []string {
	var flags []string

	if opts.Headless {
		flags = append(flags, "--headless=new")
	}

	if opts.Port != 0 {
		flags = append(flags, fmt.Sprintf("--remote-debugging-port=%d", opts.Port))
	}

	if opts.Profile != "" {
		flags = append(flags, fmt.Sprintf("--user-data-dir=%s", opts.Profile))
	}

	if len(opts.Extensions) > 0 {
		joined := strings.Join(opts.Extensions, ",")
		flags = append(flags, fmt.Sprintf("--disable-extensions-except=%s", joined))
		flags = append(flags, fmt.Sprintf("--load-extension=%s", joined))
	}

	return flags
}
