package proxy

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestObservabilityRedactsSecretsAndTracksProxyBehavior(t *testing.T) {
	const (
		querySecret = "private-query-secret"
		idpSecret   = "private-idp-session" // #nosec G101 -- Synthetic marker for the redaction test.
		cookieKey   = "01234567890123456789012345678901"
	)
	var upstreamRequestIDs []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		upstreamRequestIDs = append(upstreamRequestIDs, requestID)
		if requestID == "" || requestID == "attacker-id" {
			t.Error("upstream did not receive a generated request ID")
		}
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{
				Name: "idp", Value: idpSecret, Path: "/", Secure: true,
				HttpOnly: true, SameSite: http.SameSiteLaxMode,
			})
		case "/error":
			http.Error(w, "upstream failed", http.StatusServiceUnavailable)
		case "/malformed-error":
			w.Header().Add("Set-Cookie", "idp")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	cfg := secureTestConfig()
	cfg.CookieKeys = [][]byte{[]byte(cookieKey)}
	cfg.LogoutPathPrefix = "/logout"
	cfg.ControlPathPrefix = "/__idmux"
	cfg.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	handler, err := New(upstreamURL, cfg)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	login := observeRequest(t, client, server.URL+"/login?code="+querySecret, nil, "attacker-id")
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", login.StatusCode)
	}
	var composite *http.Cookie
	for _, cookie := range login.Cookies() {
		if cookie.Name == "IDMUX_SESSION" {
			composite = cookie
		}
		if cookie.Name == "idp" {
			t.Fatal("native identity cookie reached the browser")
		}
	}
	if composite == nil || login.Header.Get("X-Request-ID") == "attacker-id" {
		t.Fatal("proxy did not issue a composite cookie and a new request ID")
	}
	if upstreamRequestIDs[0] != login.Header.Get("X-Request-ID") {
		t.Fatal("the browser response and upstream log cannot be correlated")
	}

	for _, test := range []struct {
		target string
		status int
	}{
		{server.URL + "/account?token=" + querySecret, http.StatusOK},
		{server.URL + "/error?access_token=" + querySecret, http.StatusServiceUnavailable},
		{server.URL + "/malformed-error?access_token=" + querySecret, http.StatusBadGateway},
		{server.URL + "/fallback?authuser=7", http.StatusOK},
		{server.URL + "/logout?authuser=new", http.StatusOK},
	} {
		var cookie *http.Cookie
		if strings.Contains(test.target, "/account?") {
			cookie = composite
		}
		response := observeRequest(t, client, test.target, cookie, "attacker-id")
		if response.StatusCode != test.status {
			t.Errorf("%s status = %d, want %d", test.target, response.StatusCode, test.status)
		}
		lastID := upstreamRequestIDs[len(upstreamRequestIDs)-1]
		if response.Header.Get("X-Request-ID") != lastID {
			t.Fatal("the browser response and upstream log cannot be correlated")
		}
	}

	invalid := &http.Cookie{
		Name: "IDMUX_SESSION", Value: "private-cookie-secret", Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
	invalidResponse := observeRequest(t, client, server.URL+"/invalid?code="+querySecret, invalid, "attacker-id")
	if invalidResponse.StatusCode != http.StatusSeeOther {
		t.Errorf("invalid cookie recovery status = %d", invalidResponse.StatusCode)
	}

	metricsResponse, err := client.Get(server.URL + "/__idmux/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer metricsResponse.Body.Close()
	var metrics bytes.Buffer
	if _, err := metrics.ReadFrom(metricsResponse.Body); err != nil {
		t.Fatal(err)
	}
	if metricsResponse.StatusCode != http.StatusOK {
		t.Fatalf("metrics status %d", metricsResponse.StatusCode)
	}
	for _, name := range []string{
		"idmux_requests_total ",
		"idmux_request_duration_seconds_count ",
		"idmux_upstream_requests_total ",
		"idmux_upstream_errors_total ",
		"idmux_upstream_duration_seconds_count ",
		"idmux_fallback_total ",
		"idmux_invalid_cookies_total ",
		"idmux_logout_total ",
	} {
		if !strings.Contains(metrics.String(), name) {
			t.Errorf("metrics are missing %q", name)
		}
	}
	for _, line := range strings.Split(metrics.String(), "\n") {
		if strings.Contains(line, "{") && !strings.Contains(line, "_bucket{le=") {
			t.Errorf("metric has an unexpected label: %q", line)
		}
	}
	for name, want := range map[string]uint64{
		"idmux_requests_total":                  7,
		"idmux_request_duration_seconds_count":  7,
		"idmux_upstream_requests_total":         6,
		"idmux_upstream_errors_total":           2,
		"idmux_upstream_duration_seconds_count": 6,
		"idmux_fallback_total":                  4,
		"idmux_invalid_cookies_total":           1,
		"idmux_logout_total":                    1,
	} {
		if got := metricValue(t, metrics.String(), name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	for _, secret := range []string{querySecret, idpSecret, cookieKey, "private-cookie-secret", "attacker-id"} {
		if strings.Contains(logs.String(), secret) || strings.Contains(metrics.String(), secret) {
			t.Errorf("sensitive value leaked to logs or metrics: %q", secret)
		}
	}
}

func TestMetricsEndpointAllowsOnlyGet(t *testing.T) {
	handler := newBoundaryTestHandler(t, mustObservabilityUpstream(t))
	request := httptest.NewRequest(http.MethodPost, "http://proxy.test/control/metrics", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("metrics endpoint accepted a non-GET request: status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
	if len(response.Header().Get("X-Request-ID")) != 32 {
		t.Fatal("request ID missing")
	}
}

func metricValue(t *testing.T, exposition, name string) uint64 {
	t.Helper()
	for _, line := range strings.Split(exposition, "\n") {
		if value, ok := strings.CutPrefix(line, name+" "); ok {
			var result uint64
			if _, err := fmt.Sscan(value, &result); err != nil {
				t.Fatal(err)
			}
			return result
		}
	}
	t.Fatalf("metric %q is missing", name)
	return 0
}

func observeRequest(t *testing.T, client *http.Client, target string, cookie *http.Cookie, requestID string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Request-ID", requestID)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if len(response.Header.Get("X-Request-ID")) != 32 {
		t.Errorf("missing safe correlation ID: %q", response.Header.Get("X-Request-ID"))
	}
	return response
}

func TestObservabilityLogsUseSafeRouteAndRequestMetadata(t *testing.T) {
	var logs bytes.Buffer
	handler := newBoundaryTestHandler(t, mustObservabilityUpstream(t))
	handler.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/user/private-id?token=private-query", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if strings.Contains(logs.String(), "private-id") || strings.Contains(logs.String(), "private-query") {
		t.Fatal("request path or query secret appeared in logs")
	}
	if !strings.Contains(logs.String(), `"status":200`) || !strings.Contains(logs.String(), `"duration_ms"`) {
		t.Fatalf("request completion log is missing status or duration: %s", logs.String())
	}
	if got := fmt.Sprint(response.Code); got != "200" {
		t.Fatalf("request status %s", got)
	}
}

func TestSafeLogPathUsesFixedCategories(t *testing.T) {
	cfg := Config{
		ControlPathPrefix: "/control/private-prefix",
		LogoutPathPrefix:  "/protocol/openid-connect/logout",
	}
	for _, test := range []struct {
		path string
		want string
	}{
		{"/control/private-prefix/metrics", "/__idmux/metrics"},
		{"/protocol/openid-connect/logout/private-user", "/logout"},
		{"/users/private-user", "/"},
	} {
		request := httptest.NewRequest(http.MethodGet, "http://proxy.test"+test.path, nil)
		if got := safeLogPath(request, cfg); got != test.want {
			t.Errorf("safeLogPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func mustObservabilityUpstream(t *testing.T) *url.URL {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
