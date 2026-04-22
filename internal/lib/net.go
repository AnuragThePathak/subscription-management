package lib

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP extracts the true client IP, heavily defending against X-Forwarded-For spoofing.
func ClientIP(r *http.Request) (string, error) {
	// Establish the Trust Boundary
	remoteIPStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("failed to parse RemoteAddr: %w", err)
	}

	remoteAddr, err := netip.ParseAddr(remoteIPStr)
	if err != nil {
		return "", fmt.Errorf("invalid RemoteAddr format: %w", err)
	}

	// If the direct caller isn't a private internal IP, it's an untrusted direct connection.
	// We MUST NOT trust X-Forwarded-For or X-Real-IP.
	if !remoteAddr.IsPrivate() && !remoteAddr.IsLoopback() {
		return remoteIPStr, nil
	}

	// Safely parse X-Forwarded-For with zero-allocation
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		remaining := xff
		for {
			lastComma := strings.LastIndexByte(remaining, ',')
			var ipStr string
			if lastComma == -1 {
				ipStr = remaining // Last or only IP in the list
			} else {
				ipStr = remaining[lastComma+1:]
			}
			ipStr = strings.TrimSpace(ipStr)

			if ip, err := netip.ParseAddr(ipStr); err == nil {
				// The first public IP we hit from the right is the true client.
				if !ip.IsPrivate() && !ip.IsLoopback() {
					return ip.String(), nil
				}
			}

			if lastComma == -1 {
				break // We've evaluated the leftmost IP
			}
			// Move the cursor left
			remaining = remaining[:lastComma]
		}
	}

	// Try X-Real-IP
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		if ip, err := netip.ParseAddr(strings.TrimSpace(xRealIP)); err == nil {
			if !ip.IsPrivate() && !ip.IsLoopback() {
				return ip.String(), nil
			}
		}
	}

	// The Ultimate Fallback: Our immediate proxy/caller
	// Used if the chain only contained private IPs, or headers were empty.
	return remoteIPStr, nil
}
