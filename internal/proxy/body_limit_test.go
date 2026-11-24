package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	configpkg "github.com/4nass/idmux-proxy/internal/config"
)

func TestProxyRejectsDeclaredRequestBodyOverLimit(t *testing.T) {
	upstreamRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamRequests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.MaxRequestBodyBytes = 4
	handler, err := New(upstreamURL, config)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://proxy.test/upload", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want=%d body=%q", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
	if upstreamRequests != 0 {
		t.Fatalf("oversized request reached upstream %d times", upstreamRequests)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", response.Header().Get("Cache-Control"))
	}
}

func TestProxyRejectsChunkedRequestBodyOverLimit(t *testing.T) {
	upstreamReadError := make(chan error, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		upstreamReadError <- err
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.MaxRequestBodyBytes = 4
	handler, err := New(upstreamURL, config)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://proxy.test/upload", strings.NewReader("12345"))
	request.ContentLength = -1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want=%d body=%q", response.Code, http.StatusRequestEntityTooLarge, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", response.Header().Get("Cache-Control"))
	}
	if err := <-upstreamReadError; !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("upstream body error=%v, want unexpected EOF", err)
	}
}

func TestNewRejectsRequestBodyLimitAboveMaximum(t *testing.T) {
	upstream, err := url.Parse("https://idp.example.test")
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.MaxRequestBodyBytes = configpkg.MaxRequestBodyBytes + 1
	if _, err := New(upstream, config); err == nil {
		t.Fatal("expected oversized request body limit to be rejected")
	}
}

func TestProxyAllowsRequestBodyAtLimit(t *testing.T) {
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		upstreamBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.MaxRequestBodyBytes = 4
	handler, err := New(upstreamURL, config)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://proxy.test/upload", strings.NewReader("1234"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want=%d body=%q", response.Code, http.StatusNoContent, response.Body.String())
	}
	if upstreamBody != "1234" {
		t.Fatalf("upstream body=%q, want=1234", upstreamBody)
	}
}
