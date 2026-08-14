package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyVirtualizesMultipleSessionsAndIsolatesLogout(t *testing.T) {
	var forwardedCookie string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if issue := r.URL.Query().Get("issue"); issue != "" {
			w.Header().Add("Set-Cookie", "KEYCLOAK_IDENTITY="+issue+"; Path=/; HttpOnly")
		}
		if strings.HasPrefix(r.URL.Path, "/protocol/openid-connect/logout") {
			w.Header().Add("Set-Cookie", "KEYCLOAK_IDENTITY=; Max-Age=0; Path=/")
		}
		w.Header().Set("Content-Type", "text/plain")
		forwardedCookie = r.Header.Get("Cookie")
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	handler, err := New(upstreamURL, Config{
		IDPCookieName:     "KEYCLOAK_IDENTITY",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSameSite:    "lax",
		SessionTTL:        time.Hour,
		MaxSessions:       4,
		LogoutPathPrefix:  "/protocol/openid-connect/logout",
		ControlPathPrefix: "/__idmux",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	proxyServer := httptest.NewServer(handler)
	defer proxyServer.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	body := getBody(t, client, proxyServer.URL+"/login?authuser=new&issue=idp-A")
	if body != "" {
		t.Fatalf("new login forwarded a native IdP cookie: %q", body)
	}
	assertNoCookie(t, jar, proxyServer.URL, "KEYCLOAK_IDENTITY")

	getBody(t, client, proxyServer.URL+"/check?authuser=0")
	if forwardedCookie != "KEYCLOAK_IDENTITY=idp-A" {
		t.Fatalf("default session was not selected: %q", forwardedCookie)
	}

	body = getBody(t, client, proxyServer.URL+"/login?authuser=new&issue=idp-B")
	if body != "" {
		t.Fatalf("second login forwarded a native IdP cookie: %q", body)
	}
	getBody(t, client, proxyServer.URL+"/check?authuser=1")
	if forwardedCookie != "KEYCLOAK_IDENTITY=idp-B" {
		t.Fatalf("second session was not selected: %q", forwardedCookie)
	}

	getBody(t, client, proxyServer.URL+"/protocol/openid-connect/logout?authuser=1")
	if forwardedCookie != "KEYCLOAK_IDENTITY=idp-B" {
		t.Fatalf("logout did not target the selected session: %q", forwardedCookie)
	}
	getBody(t, client, proxyServer.URL+"/check?authuser=0")
	if forwardedCookie != "KEYCLOAK_IDENTITY=idp-A" {
		t.Fatalf("logout removed the wrong session: %q", forwardedCookie)
	}

	response, err := client.Get(proxyServer.URL + "/__idmux/sessions")
	if err != nil {
		t.Fatalf("read sessions endpoint: %v", err)
	}
	defer response.Body.Close()
	var public struct {
		Sessions []struct {
			Index        int    `json:"index"`
			IDPSessionID string `json:"idp_session_id"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&public); err != nil {
		t.Fatalf("decode sessions endpoint: %v", err)
	}
	if response.StatusCode != http.StatusOK || len(public.Sessions) != 1 || public.Sessions[0].Index != 0 || public.Sessions[0].IDPSessionID != "" {
		t.Fatalf("sessions endpoint leaked or kept wrong state: status=%d body=%+v", response.StatusCode, public)
	}
}

func TestProxyFallsBackOnConflictingRoutingInputs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	handler, err := New(upstreamURL, Config{
		IDPCookieName:     "idp",
		SessionCookieName: "multi",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		SessionTTL:        time.Hour,
		MaxSessions:       2,
		LogoutPathPrefix:  "/logout",
		ControlPathPrefix: "/control",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/?authuser=1", nil)
	request.Header.Set("X-Auth-User-Index", "2")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Fatalf("expected a fresh fallback request, got status=%d body=%q", response.Code, response.Body.String())
	}
}

func getBody(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status=%d body=%s", target, response.StatusCode, body)
	}
	return string(body)
}

func assertNoCookie(t *testing.T, jar http.CookieJar, rawURL, name string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range jar.Cookies(parsed) {
		if cookie.Name == name {
			t.Fatalf("jar contains forbidden cookie %s", name)
		}
	}
}

func ExampleHandler() {
	upstream, _ := url.Parse("https://idp.example.com")
	_, _ = New(upstream, Config{
		IDPCookieName:     "KEYCLOAK_IDENTITY",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSecure:      true,
		CookieSameSite:    "lax",
		SessionTTL:        12 * time.Hour,
		MaxSessions:       8,
	})
	fmt.Println("ready")
	// Output: ready
}

func TestProxyClearsInvalidCookieAndStartsFreshSession(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("invalid cookie request reached upstream")
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)
	handler, err := New(upstreamURL, Config{
		IDPCookieName:     "idp",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSecure:      true,
		CookieSameSite:    "lax",
		SessionTTL:        time.Hour,
		MaxSessions:       2,
		LogoutPathPrefix:  "/logout",
		ControlPathPrefix: "/control",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/start", nil)
	request.AddCookie(&http.Cookie{
		Name:     "IDMUX_SESSION",
		Value:    "corrupted",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", response.Code)
	}
	if !strings.Contains(response.Header().Get("Location"), "authuser=new") {
		t.Fatalf("redirect did not start a new session: %q", response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "IDMUX_SESSION" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" || cookies[0].MaxAge >= 0 {
		t.Fatalf("invalid cookie was not safely expired: %+v", cookies)
	}
}
