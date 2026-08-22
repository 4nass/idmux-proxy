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
