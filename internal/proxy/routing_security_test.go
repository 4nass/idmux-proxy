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
	request.AddCookie(&http.Cookie{Name: composite.Name, Value: composite.Value})
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
