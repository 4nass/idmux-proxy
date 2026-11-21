package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/4nass/idmux-proxy/internal/session"
)

func TestLoginAndLogoutRouting(t *testing.T) {
	// Two occupied slots make wrong fallbacks and accidental allocations visible.
	state := session.NewState()
	for _, value := range []string{"first", "second"} {
		if _, err := state.Add(value, 2); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{cfg: Config{FreshLoginPath: "/form", LogoutPathPrefix: "/logout"}}
	for _, test := range []struct {
		path    string
		headers []string
		index   int
		wantErr bool
	}{
		{"/form?authuser=1", nil, 1, false},
		{"/form", []string{"1"}, 1, false},
		{"/form?authuser=0&authuser=1", nil, 0, true},
		{"/form", []string{"0", "1"}, 0, true},
		{"/form?authuser=0", []string{"1"}, 0, true},
		{"/form?authuser=2", nil, 0, true},
		{"/logout?authuser=1", nil, 1, false},
		{"/logout?authuser=3", nil, 0, true},
		{"/logout", nil, 0, true},
	} {
		req := httptest.NewRequest(http.MethodPost, test.path, nil)
		for _, value := range test.headers {
			req.Header.Add("X-Auth-User-Index", value)
		}
		target, err := h.resolveTarget(req, state, h.isLogoutPath(req.URL.Path))
		if err == nil {
			err = validateTarget(target, state, 2)
		}
		if (err != nil) != test.wantErr || (err == nil && (target.New || target.Index != test.index)) {
			t.Errorf("%s headers %v: target %+v, error %v", test.path, test.headers, target, err)
		}
	}
}

func TestLogoutKeepsSlotUntilConfirmed(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/seed":
			w.Header().Set("Set-Cookie", "idp=first; Path=/")
		case "/logout/confirmation":
			w.WriteHeader(http.StatusOK)
		case "/logout/error":
			w.Header().Set("Set-Cookie", "idp=; Max-Age=0; Path=/")
			w.WriteHeader(http.StatusBadRequest)
		case "/logout/done":
			w.Header().Set("Set-Cookie", "idp=; Max-Age=0; Path=/")
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer origin.Close()
	upstream, err := url.Parse(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	handler := newBoundaryTestHandler(t, upstream)
	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, httptest.NewRequest(http.MethodGet, "/seed?authuser=new", nil))
	cookies := seed.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != handler.cfg.SessionCookieName {
		t.Fatal("missing seed cookie")
	}
	for _, test := range []struct {
		path    string
		status  int
		expired bool
	}{
		{"/logout/confirmation", 200, false},
		{"/logout/error", 400, false},
		{"/logout/done", 302, true},
	} {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, test.path+"?authuser=0", nil)
		req.AddCookie(cookies[0])
		handler.ServeHTTP(response, req)
		set := response.Result().Cookies()
		if response.Code != test.status {
			t.Fatalf("%s returned %d", test.path, response.Code)
		}
		if test.expired {
			if len(set) != 1 || set[0].Name != handler.cfg.SessionCookieName || set[0].MaxAge >= 0 {
				t.Errorf("%s did not expire the composite cookie", test.path)
			}
		} else if len(set) != 0 {
			t.Errorf("%s changed browser state before logout finished", test.path)
		}
	}
}
