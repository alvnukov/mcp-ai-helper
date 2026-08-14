package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Browsers send cross-origin form posts as simple requests with no
// preflight, so a malicious page can drive the loopback task API with a
// text/plain body. Writes must require a JSON content type and, when the
// browser supplies an Origin, that it matches the loopback host.
func TestTaskUIDecodeRejectsCrossOriginWrites(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		origin      string
		host        string
		wantErr     bool
	}{
		{"same origin json", "application/json", "http://127.0.0.1:18067", "127.0.0.1:18067", false},
		{"no origin json", "application/json", "", "127.0.0.1:18067", false},
		{"foreign origin", "application/json", "http://evil.example", "127.0.0.1:18067", true},
		{"text/plain simple post", "text/plain", "", "127.0.0.1:18067", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var target struct {
				Status string `json:"status"`
			}
			r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/tasks/x/status", strings.NewReader(`{"status":"done"}`))
			r.Header.Set("Content-Type", tc.contentType)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			err := decodeTaskUIJSON(r, &target)
			if tc.wantErr && err == nil {
				t.Fatal("cross-origin write accepted")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("legitimate write rejected: %v", err)
			}
		})
	}
}
