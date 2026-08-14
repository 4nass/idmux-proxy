package session

import (
	"net/http"
	"net/url"
	"testing"
)

func TestResolveTargetPrecedenceAndDefaults(t *testing.T) {
	request := &http.Request{URL: &url.URL{RawQuery: "authuser=2"}, Header: make(http.Header)}
	target, err := ResolveTarget(request)
	if err != nil || target.New || target.Index != 2 {
		t.Fatalf("resolve query target: %+v err=%v", target, err)
	}
	request = &http.Request{URL: &url.URL{}, Header: http.Header{"X-Auth-User-Index": []string{"1"}}}
	target, err = ResolveTarget(request)
	if err != nil || target.Index != 1 {
		t.Fatalf("resolve header target: %+v err=%v", target, err)
	}
	request = &http.Request{URL: &url.URL{}, Header: make(http.Header)}
	target, err = ResolveTarget(request)
	if err != nil || target.Index != 0 {
		t.Fatalf("resolve default target: %+v err=%v", target, err)
	}
}

func TestResolveTargetRejectsConflictingInputs(t *testing.T) {
	request := &http.Request{URL: &url.URL{RawQuery: "authuser=1"}, Header: http.Header{"X-Auth-User-Index": []string{"2"}}}
	if _, err := ResolveTarget(request); err == nil {
		t.Fatal("expected conflicting target inputs to be rejected")
	}
}
