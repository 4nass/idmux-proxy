package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func FuzzParseSetCookieDoesNotPanic(f *testing.F) {
	f.Add("idp=session; Path=/")
	f.Add("idp=; Max-Age=0; Path=/")
	f.Add("")
	f.Add("not-a-cookie")
	f.Add("=missing-name")
	f.Fuzz(func(t *testing.T, raw string) {
		cookie, ok := parseSetCookie(raw)
		if ok && (cookie == nil || cookie.Name == "") {
			t.Fatalf("parser accepted a cookie without a name: %#v", cookie)
		}
	})
}

func TestProxyRejectsOversizedCompositeCookie(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("oversized session cookie reached upstream")
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstreamURL)
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/check", nil)
	request.Header.Set("Cookie", "IDMUX_SESSION="+strings.Repeat("A", 3801))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected safe redirect, got status=%d", response.Code)
	}
	if response.Header().Get("Location") == "" {
		t.Fatal("invalid cookie response did not include a redirect")
	}
}

func TestProxyCookieDoesNotExposeIdpSessionID(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "idp=clear-session-id; Path=/")
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

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "IDMUX_SESSION" && strings.Contains(cookie.Value, "clear-session-id") {
			t.Fatal("composite cookie contains a clear IdP session ID")
		}
	}
}
