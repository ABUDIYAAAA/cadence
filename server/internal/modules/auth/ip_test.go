package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
	}{
		{
			name:       "standard IP with port",
			remoteAddr: "192.0.2.1:54321",
			wantIP:     "192.0.2.1",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			wantIP:     "2001:db8::1",
		},
		{
			name:       "bare IP without port",
			remoteAddr: "198.51.100.42",
			wantIP:     "198.51.100.42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			got := getClientIP(req)
			if got != tt.wantIP {
				t.Errorf("getClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}
