package lib_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anuragthepathak/subscription-management/internal/lib"
)

// makeReq is a helper that builds a minimal *http.Request with the given
// RemoteAddr and optional headers, so each test case stays concise.
func makeReq(remoteAddr string, headers map[string]string) *http.Request {
	r := &http.Request{
		RemoteAddr: remoteAddr,
		Header:     make(http.Header),
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name    string
		r       *http.Request
		want    string
		wantErr bool
	}{
		// ── Direct connections (no proxy headers involved) ────────────────────

		// RemoteAddr is a public IP → trust it directly, ignore any headers.
		{
			name: "Direct public IP — no headers",
			r:    makeReq("203.0.113.5:54321", nil),
			want: "203.0.113.5",
		},
		// A public RemoteAddr with a (potentially spoofed) XFF header present.
		// The function must still return RemoteAddr and NOT trust the header.
		{
			name: "Direct public IP — XFF present but must be ignored (spoofing guard)",
			r: makeReq("203.0.113.5:54321", map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			}),
			want: "203.0.113.5",
		},
		// Private RemoteAddr, no headers at all → fall through to the ultimate
		// fallback and return RemoteAddr itself.
		{
			name: "Private RemoteAddr — no proxy headers (ultimate fallback)",
			r:    makeReq("10.0.0.1:54321", nil),
			want: "10.0.0.1",
		},

		// ── X-Forwarded-For traversal (right-to-left) ─────────────────────────

		// Single public IP in XFF.
		{
			name: "XFF — single public IP",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			}),
			want: "203.0.113.1",
		},
		// Classic proxy chain: client → public proxy → internal proxy → server.
		// The rightmost public IP in XFF is the true client.
		{
			name: "XFF — chain with private IPs appended by internal proxies",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.2, 172.16.0.1",
			}),
			want: "203.0.113.1",
		},
		// Attacker prepends a fake IP; traversal from the right skips it and
		// finds the rightmost public IP, which is the first trustworthy one.
		{
			name: "XFF — spoofed leftmost IP is skipped, rightmost public IP wins",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Forwarded-For": "1.3.3.7, 203.0.113.1, 10.0.0.2",
			}),
			want: "203.0.113.1",
		},
		// All IPs in XFF are private → XFF yields nothing, fall through.
		// X-Real-IP is also absent, so ultimate fallback (RemoteAddr) applies.
		{
			name: "XFF — all private IPs, no X-Real-IP (ultimate fallback)",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Forwarded-For": "10.0.0.2, 172.16.0.1",
			}),
			want: "10.0.0.1",
		},

		// ── X-Real-IP fallback ────────────────────────────────────────────────

		// XFF absent, X-Real-IP present with a public IP.
		{
			name: "X-Real-IP — used when XFF is absent",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Real-IP": "203.0.113.99",
			}),
			want: "203.0.113.99",
		},
		// XFF is all-private, X-Real-IP has a public IP → X-Real-IP wins.
		{
			name: "X-Real-IP — used when XFF contains only private IPs",
			r: makeReq("10.0.0.1:54321", map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-IP":       "203.0.113.42",
			}),
			want: "203.0.113.42",
		},

		// ── Error path ────────────────────────────────────────────────────────

		// Malformed RemoteAddr (missing port) → net.SplitHostPort fails.
		{
			name:    "Malformed RemoteAddr returns error",
			r:       makeReq("not-an-ip", nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lib.ClientIP(tt.r)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
