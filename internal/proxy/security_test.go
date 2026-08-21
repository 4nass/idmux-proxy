package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestProxyStripsCompositeIdentityAndInternalRequestHeaders(t *testing.T) {
	var upstreamRequest *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequest = r.Clone(r.Context())
		if r.URL.Query().Get("issue") != "" {
			w.Header().Add("Set-Cookie", "idp=real-session; Path=/")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstreamURL)

	seed := httptest.NewRequest(http.MethodGet, "http://proxy.test/login?authuser=new&issue=real", nil)
	seedResponse := httptest.NewRecorder()
	handler.ServeHTTP(seedResponse, seed)
	if seedResponse.Code != http.StatusNoContent {
		t.Fatalf("seed request: status=%d", seedResponse.Code)
	}
	var composite *http.Cookie
	for _, cookie := range seedResponse.Result().Cookies() {
		if cookie.Name == "IDMUX_SESSION" {
			composite = cookie
			break
		}
	}
	if composite == nil || composite.Value == "" {
		t.Fatal("seed request did not create a composite session cookie")
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/check?authuser=0", nil)
	request.Header.Add("Cookie", "idp=attacker-one")
	request.Header.Add("Cookie", "idp=attacker-two")
	request.AddCookie(&http.Cookie{Name: "IDMUX_SESSION", Value: composite.Value})
	request.AddCookie(&http.Cookie{Name: "keep", Value: "yes"})
	request.Header.Set("X-Auth-User-Index", "0")
	request.Header.Set("X-IdMux-Internal-Role", "admin")
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	request.Header.Set("X-Forwarded-Host", "attacker.example")
	request.Header.Set("X-Forwarded-Proto", "http")
	request.Header.Set("X-Real-IP", "203.0.113.10")
	request.Header.Set("Forwarded", "for=203.0.113.10")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("request: status=%d", response.Code)
	}
	if upstreamRequest == nil {
		t.Fatal("upstream did not receive the request")
	}

	forwarded := upstreamRequest.Cookies()
	identityCount := 0
	for _, cookie := range forwarded {
		switch cookie.Name {
		case "idp":
			identityCount++
			if cookie.Value != "real-session" {
				t.Fatalf("unexpected IdP cookie value: %q", cookie.Value)
			}
		case "IDMUX_SESSION":
			t.Fatal("composite session cookie crossed the trust boundary")
		case "keep":
			if cookie.Value != "yes" {
				t.Fatalf("unexpected application cookie value: %q", cookie.Value)
			}
		}
	}
	if identityCount != 1 {
		t.Fatalf("expected exactly one IdP cookie, got %d: %q", identityCount, upstreamRequest.Header.Get("Cookie"))
	}

	for _, name := range []string{
		"X-Auth-User-Index",
		"X-IdMux-Internal-Role",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Real-IP",
		"Forwarded",
	} {
		if value := upstreamRequest.Header.Get(name); value != "" {
			t.Fatalf("sensitive request header %s crossed the trust boundary: %q", name, value)
		}
	}
}

func TestProxyRemovesSensitiveResponseHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Auth-User-Index", "1")
		w.Header().Set("X-IdMux-Internal-Role", "admin")
		w.Header().Set("X-Forwarded-For", "203.0.113.10")
		w.Header().Set("X-Forwarded-Host", "idp.example")
		w.Header().Set("X-Forwarded-Proto", "https")
		w.Header().Set("X-Real-IP", "203.0.113.10")
		w.Header().Set("Forwarded", "for=203.0.113.10")
		w.Header().Add("Set-Cookie", "IDMUX_SESSION=forbidden; Path=/")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstreamURL)
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/check", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("request: status=%d", response.Code)
	}

	for _, name := range []string{
		"X-Auth-User-Index",
		"X-IdMux-Internal-Role",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto",
		"X-Real-IP",
		"Forwarded",
	} {
		if value := response.Header().Get(name); value != "" {
			t.Fatalf("sensitive response header was exposed: %s=%q", name, value)
		}
	}
	for _, raw := range response.Header().Values("Set-Cookie") {
		if raw == "IDMUX_SESSION=forbidden; Path=/" {
			t.Fatal("upstream tried to overwrite the composite session cookie")
		}
	}
}

func newBoundaryTestHandler(t *testing.T, upstreamURL *url.URL) *Handler {
	t.Helper()
	handler, err := New(upstreamURL, Config{
		IDPCookieName:     "idp",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSecure:      true,
		CookieSameSite:    "lax",
		SessionTTL:        time.Hour,
		MaxSessions:       4,
		LogoutPathPrefix:  "/logout",
		ControlPathPrefix: "/control",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	return handler
}
