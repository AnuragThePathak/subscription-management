package lib

import (
	"os"
	"sync"
)

var (
	hostname     string
	hostnameOnce sync.Once
)

// Hostname safely resolves the host name regardless of environment.
// The result is cached after the first call to avoid repeated env reads and syscalls.
func Hostname() string {
	hostnameOnce.Do(func() {
		// Try Kubernetes Downward API first
		if name := os.Getenv("POD_NAME"); name != "" {
			hostname = name
			return
		}

		// Fall back to the OS environment variable
		if name := os.Getenv("HOSTNAME"); name != "" {
			hostname = name
			return
		}

		// Fall back to OS-provided hostname via syscall
		if name, err := os.Hostname(); err == nil && name != "" {
			hostname = name
			return
		}

		// Absolute worst-case fallback
		hostname = "unknown-host"
	})

	return hostname
}
