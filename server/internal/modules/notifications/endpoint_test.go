package notifications

import (
	"net"
	"testing"
)

func TestValidatePushEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid FCM https endpoint",
			url:     "https://fcm.googleapis.com/fcm/send/sample-token",
			wantErr: false,
		},
		{
			name:    "valid Mozilla push endpoint",
			url:     "https://updates.push.services.mozilla.com/wpush/v2/sample-token",
			wantErr: false,
		},
		{
			name:    "insecure http scheme",
			url:     "http://fcm.googleapis.com/fcm/send/sample-token",
			wantErr: true,
		},
		{
			name:    "loopback IPv4 address",
			url:     "https://127.0.0.1/notify",
			wantErr: true,
		},
		{
			name:    "loopback localhost hostname",
			url:     "https://localhost/notify",
			wantErr: true,
		},
		{
			name:    "cloud metadata link-local IP",
			url:     "https://169.254.169.254/latest/meta-data/",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 10.0.0.0/8",
			url:     "https://10.0.1.5:443/push",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 172.16.0.0/12",
			url:     "https://172.20.0.2/push",
			wantErr: true,
		},
		{
			name:    "private RFC 1918 192.168.0.0/16",
			url:     "https://192.168.1.100/push",
			wantErr: true,
		},
		{
			name:    "custom port probe",
			url:     "https://fcm.googleapis.com:8080/push",
			wantErr: true,
		},
		{
			name:    "internal service custom port",
			url:     "https://127.0.0.1:15672/api",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePushEndpoint(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePushEndpoint(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsRestrictedIP(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		restricted bool
	}{
		{"IPv4 Loopback", "127.0.0.1", true},
		{"IPv6 Loopback", "::1", true},
		{"Cloud metadata", "169.254.169.254", true},
		{"Link-local unicast", "169.254.1.1", true},
		{"Private class A", "10.10.10.10", true},
		{"Private class B", "172.16.1.1", true},
		{"Private class C", "192.168.1.1", true},
		{"Carrier grade NAT", "100.64.0.1", true},
		{"Unspecified IPv4", "0.0.0.0", true},
		{"Unspecified IPv6", "::", true},
		{"Public Cloudflare DNS", "1.1.1.1", false},
		{"Public Google DNS", "8.8.8.8", false},
		{"Public Quad9 DNS", "9.9.9.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := net.ParseIP(tt.ip)
			if parsed == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			got := IsRestrictedIP(parsed)
			if got != tt.restricted {
				t.Errorf("IsRestrictedIP(%s) = %v, want %v", tt.ip, got, tt.restricted)
			}
		})
	}
}
