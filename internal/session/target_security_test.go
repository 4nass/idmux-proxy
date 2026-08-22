package session

import (
	"net/http"
	"net/url"
	"testing"
)

func TestResolveTargetRejectsAmbiguousValues(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
	}{
		{
			name: "repeated query value",
			request: &http.Request{
				URL:    &url.URL{RawQuery: "authuser=0&authuser=0"},
				Header: make(http.Header),
			},
		},
		{
			name: "repeated header value",
			request: &http.Request{
				URL:    &url.URL{},
				Header: http.Header{"X-Auth-User-Index": []string{"0", "0"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ResolveTarget(test.request); err == nil {
				t.Fatal("expected ambiguous routing input to be rejected")
			}
		})
	}
}

func TestResolveTargetRejectsIncompleteRequest(t *testing.T) {
	if _, err := ResolveTarget(nil); err == nil {
		t.Fatal("expected nil request to be rejected")
	}
	if _, err := ResolveTarget(&http.Request{}); err == nil {
		t.Fatal("expected request without URL to be rejected")
	}
}

func FuzzResolveTargetDoesNotPanic(f *testing.F) {
	f.Add("authuser=0", "")
	f.Add("authuser=new&authuser=0", "1")
	f.Add("%zz", "new")

	f.Fuzz(func(_ *testing.T, query, header string) {
		request := &http.Request{
			URL:    &url.URL{RawQuery: query},
			Header: make(http.Header),
		}
		if header != "" {
			request.Header.Add("X-Auth-User-Index", header)
			if len(header)%2 == 0 {
				request.Header.Add("X-Auth-User-Index", header)
			}
		}
		_, _ = ResolveTarget(request)
	})
}
