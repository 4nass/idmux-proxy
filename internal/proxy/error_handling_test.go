package proxy

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyUpstreamErrorDoesNotLeakDetails(t *testing.T) {
	upstreamURL, err := url.Parse("http://upstream.test")
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	handler, err := New(upstreamURL, Config{
		IDPCookieName:     "idp",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSecure:      true,
		SessionTTL:        time.Hour,
		MaxSessions:       2,
		Logger:            slog.New(slog.NewJSONHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/callback?code=secret-code", nil)
	response := httptest.NewRecorder()
	response.Header().Set("Set-Cookie", "IDMUX_SESSION=must-not-survive")
	response.Header().Set("X-IdMux-Internal-Role", "admin")
	handler.proxy.ErrorHandler(response, request, errors.New("idp=secret-session"))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected bad gateway, got %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store response, got %q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Set-Cookie") != "" || response.Header().Get("X-IdMux-Internal-Role") != "" {
		t.Fatalf("unsafe error headers survived: %+v", response.Header())
	}
	if strings.Contains(response.Body.String(), "secret-session") || strings.Contains(logs.String(), "secret-session") {
		t.Fatal("upstream error details leaked to the client or logs")
	}
}

func TestProxyRejectsMalformedUpstreamIdentityCookie(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "idp")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstreamURL)
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/login?authuser=new", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected malformed upstream cookie to fail safely, got %d", response.Code)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store response, got %q", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("malformed cookie was forwarded: %q", response.Header().Get("Set-Cookie"))
	}
	if strings.Contains(response.Body.String(), "idp") {
		t.Fatalf("error response exposed upstream details: %q", response.Body.String())
	}
}
