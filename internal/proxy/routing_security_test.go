package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProxyCanonicalizesRoutingInputBeforeUpstream(t *testing.T) {
	var upstreamAuthUsers []string
	var upstreamCookie string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthUsers = append([]string(nil), r.URL.Query()["authuser"]...)
		upstreamCookie = r.Header.Get("Cookie")
		if r.URL.Query().Get("issue") != "" {
			w.Header().Add("Set-Cookie", "idp=seed-session; Path=/")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstreamURL)

	seedRequest := httptest.NewRequest(http.MethodGet, "http://proxy.test/login?authuser=new&issue=seed", nil)
	seedResponse := httptest.NewRecorder()
	handler.ServeHTTP(seedResponse, seedRequest)

	var composite *http.Cookie
	for _, cookie := range seedResponse.Result().Cookies() {
		if cookie.Name == "IDMUX_SESSION" {
			composite = cookie
			break
		}
	}
	if composite == nil {
		t.Fatal("seed request did not create a composite cookie")
	}

	request := httptest.NewRequest(http.MethodGet, "http://proxy.test/check?authuser=0&authuser=new&scope=openid", nil)
	request.AddCookie(&http.Cookie{
		Name:     composite.Name,
		Value:    composite.Value,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("request: status=%d", response.Code)
	}
	if len(upstreamAuthUsers) != 1 || upstreamAuthUsers[0] != "0" {
		t.Fatalf("upstream received ambiguous routing values: %q", upstreamAuthUsers)
	}
	if upstreamCookie != "idp=seed-session" {
		t.Fatalf("upstream received the wrong session cookie: %q", upstreamCookie)
	}
}

func TestFreshLoginPathUsesTheSelectedSession(t *testing.T) {
	routes := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			w.Header().Set("Set-Cookie", "idp=seed-session; Path=/")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		routes <- r.URL.Query().Get("authuser") + "|" + r.Header.Get("Cookie")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := secureTestConfig()
	cfg.FreshLoginPath = "/form"
	handler, err := New(upstreamURL, cfg)
	if err != nil {
		t.Fatal(err)
	}
	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed?authuser=new", nil))
	cookies := seed.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cfg.SessionCookieName {
		t.Fatal("seed request did not create a composite cookie")
	}

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"form submission", http.MethodPost, "/form", "new|"},
		{"form submission with explicit index", http.MethodPost, "/form?authuser=0", "0|idp=seed-session"},
		{"form view", http.MethodGet, "/form", "0|idp=seed-session"},
		{"different submission path", http.MethodPost, "/form/other", "0|idp=seed-session"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			req.AddCookie(cookies[0])
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
			}
			select {
			case got := <-routes:
				if got != test.want {
					t.Fatalf("upstream route and cookie = %q, want %q", got, test.want)
				}
			default:
				t.Fatal("upstream did not receive the request")
			}
		})
	}
}
