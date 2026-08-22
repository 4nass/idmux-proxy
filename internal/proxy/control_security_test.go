package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSessionsRejectsAmbiguousOriginHeaders(t *testing.T) {
	upstreamURL, err := url.Parse("https://idp.example.test")
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.ControlPathPrefix = "/control"
	config.TrustedOrigins = []string{"https://app.example.test"}
	handler, err := New(upstreamURL, config)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	tests := []struct {
		name       string
		origins    []string
		wantStatus int
	}{
		{
			name:       "one trusted origin",
			origins:    []string{"https://app.example.test"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "two trusted origin headers",
			origins:    []string{"https://app.example.test", "https://app.example.test"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "trusted and untrusted origin headers",
			origins:    []string{"https://app.example.test", "https://attacker.example.test"},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://proxy.test/control/sessions", nil)
			for _, origin := range test.origins {
				request.Header.Add("Origin", origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want=%d", response.Code, test.wantStatus)
			}
			if test.wantStatus != http.StatusOK && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unsafe cache policy: %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}
