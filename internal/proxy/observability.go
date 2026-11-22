package proxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
)

type requestIDKey struct{}

func requestIDFrom(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

func newRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func safeMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

func safeLogPath(r *http.Request, cfg Config) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	control := strings.TrimRight(cfg.ControlPathPrefix, "/")
	switch r.URL.Path {
	case control + "/healthz":
		return "/__idmux/healthz"
	case control + "/readyz":
		return "/__idmux/readyz"
	case control + "/sessions":
		return "/__idmux/sessions"
	case control + "/metrics":
		return "/__idmux/metrics"
	}
	if prefix := strings.TrimRight(cfg.LogoutPathPrefix, "/"); prefix != "" &&
		(r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/")) {
		return "/logout"
	}
	return "/"
}

type observedResponseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *observedResponseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if !w.wrote {
		w.status, w.wrote = status, true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *observedResponseWriter) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *observedResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *observedResponseWriter) Flush() {
	_ = w.FlushError()
}

func (w *observedResponseWriter) FlushError() error {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *observedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *observedResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}
