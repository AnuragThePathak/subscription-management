package lib

import "os"

// GetPodIdentity safely resolves the pod name regardless of environment
func GetPodIdentity() string {
	// Try the exact K8s Downward API first (The Enterprise Standard)
	if name := os.Getenv("POD_NAME"); name != "" {
		return name
	}

	// Fall back to the OS Hostname (Good for Docker/Local OS)
	if name := os.Getenv("HOSTNAME"); name != "" {
		return name
	}

	// Fallback to OS-provided hostname (Good for Local/VMs)
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}

	// Absolute worst-case fallback
	return "unknown-instance"
}
