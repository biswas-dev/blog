package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		remote  string
		want    string
	}{
		{
			name:    "CF-Connecting-IP header",
			headers: map[string]string{"CF-Connecting-IP": "1.2.3.4"},
			remote:  "5.6.7.8:1234",
			want:    "1.2.3.4",
		},
		{
			name:    "X-Real-IP header",
			headers: map[string]string{"X-Real-IP": "10.0.0.1"},
			remote:  "5.6.7.8:1234",
			want:    "10.0.0.1",
		},
		{
			name:    "X-Forwarded-For single IP",
			headers: map[string]string{"X-Forwarded-For": "172.16.0.1"},
			remote:  "5.6.7.8:1234",
			want:    "172.16.0.1",
		},
		{
			name:    "X-Forwarded-For multiple IPs",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.1, 172.16.0.1"},
			remote:  "5.6.7.8:1234",
			want:    "192.168.1.1",
		},
		{
			name:    "falls back to RemoteAddr",
			headers: map[string]string{},
			remote:  "203.0.113.50:8080",
			want:    "203.0.113.50",
		},
		{
			name:    "RemoteAddr without port",
			headers: map[string]string{},
			remote:  "203.0.113.50",
			want:    "203.0.113.50",
		},
		{
			name:    "CF-Connecting-IP takes priority over X-Real-IP",
			headers: map[string]string{"CF-Connecting-IP": "1.1.1.1", "X-Real-IP": "2.2.2.2"},
			remote:  "3.3.3.3:80",
			want:    "1.1.1.1",
		},
		{
			name:    "X-Real-IP takes priority over X-Forwarded-For",
			headers: map[string]string{"X-Real-IP": "2.2.2.2", "X-Forwarded-For": "4.4.4.4"},
			remote:  "3.3.3.3:80",
			want:    "2.2.2.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			got := ExtractIP(req)
			if got != tt.want {
				t.Errorf("ExtractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
