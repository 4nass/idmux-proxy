package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestQueuedBrowserFlow(t *testing.T) {
	type observed struct{ path, route, marker, cookie string }
	requests := make(chan observed, 2)
	const redirect = "https://app.example.test/callback?state=test&code=test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests <- observed{r.URL.Path, q.Get("authuser"), q.Get("__idmux_login_frame"), r.Header.Get("Cookie")}
		if r.URL.Path == "/form" {
			w.Header().Set("Set-Cookie", "idp=test; Path=/")
			w.Header().Set("Location", redirect)
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer upstream.Close()
	issuer, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := secureTestConfig()
	cfg.LoginStartPath, cfg.FreshLoginPath = "/auth", "/form"
	h, err := New(issuer, cfg)
	if err != nil {
		t.Fatal(err)
	}
	const auth = "/auth?authuser=new&response_type=code&redirect_uri=https%3A%2F%2Fapp.example.test%2Fcallback"
	start := httptest.NewRequest(http.MethodGet, auth, nil)
	start.Header.Set("Sec-Fetch-Dest", "document")
	waiting := httptest.NewRecorder()
	h.ServeHTTP(waiting, start)
	if waiting.Code != http.StatusOK || waiting.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(waiting.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || len(requests) != 0 {
		t.Fatal("the waiting page must be private and must not start an upstream login")
	}

	for _, step := range []struct {
		method, url, cookie string
		want                observed
	}{
		{http.MethodGet, auth + "&__idmux_login_frame=1", "transaction=old", observed{"/auth", "new", "", ""}},
		{http.MethodPost, "/form?__idmux_login_frame=1", "transaction=current", observed{"/form", "new", "", "transaction=current"}},
	} {
		req := httptest.NewRequest(step.method, step.url, nil)
		req.Header.Set("Sec-Fetch-Dest", "iframe")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Cookie", step.cookie)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != http.StatusOK || len(requests) != 1 {
			t.Fatalf("unexpected upstream request or status for %s", step.method)
		}
		if got := <-requests; got != step.want {
			t.Errorf("%s upstream state: got %+v, want %+v", step.method, got, step.want)
		}
		if step.method == http.MethodPost {
			cookies := res.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Name != cfg.SessionCookieName {
				t.Fatal("the session must be saved before completing the callback")
			}
			if res.Header().Get("Location") != "" || res.Header().Get("Cache-Control") != "no-store" ||
				!strings.Contains(res.Body.String(), "state=test&amp;code=test") {
				t.Fatal("expected a private, escaped callback for the waiting page")
			}
		}
	}
}

func TestLoginQueueOnlyHandlesBrowserCodeCallbacks(t *testing.T) {
	for _, test := range []struct {
		callback string
		mode     string
		want     bool
	}{
		{"https://app.example.test/callback", "query", true},
		{"https://app.example.test/callback", "form_post", false},
		{"myapp://login/callback", "query", false},
	} {
		query := url.Values{"response_type": {"code"}, "authuser": {"new"},
			"redirect_uri": {test.callback}, "response_mode": {test.mode}}
		req := httptest.NewRequest(http.MethodGet, "/auth?"+query.Encode(), nil)
		req.Header.Set("Sec-Fetch-Dest", "document")
		if queueNavigation(req) != test.want {
			t.Errorf("unexpected queue decision for %s with %s", test.callback, test.mode)
		}
	}
}

func TestLoginQueueRequiresDistinctConfiguredPaths(t *testing.T) {
	upstream := &url.URL{Scheme: "https", Host: "idp.example.test"}
	for _, paths := range [][2]string{{"/auth", ""}, {"/auth", "/auth"}, {"relative", "/form"}, {"/auth", "/form?q=x"}} {
		cfg := secureTestConfig()
		cfg.LoginStartPath, cfg.FreshLoginPath = paths[0], paths[1]
		if _, err := New(upstream, cfg); err == nil {
			t.Errorf("accepted invalid login paths %q", paths)
		}
	}
}
