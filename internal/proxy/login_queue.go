package proxy

import (
	"crypto/rand"
	_ "embed"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

//go:embed login_queue.html
var loginQueuePage string

func (h *Handler) loginStart(r *http.Request) bool {
	return h.cfg.LoginStartPath != "" && r.Method == http.MethodGet && r.URL.Path == h.cfg.LoginStartPath
}

func queueNavigation(r *http.Request) bool {
	query := r.URL.Query()
	mode := query.Get("response_mode")
	callback, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || callback.Host == "" || callback.User != nil ||
		(callback.Scheme != "http" && callback.Scheme != "https") {
		return false
	}
	return r.Header.Get("Sec-Fetch-Dest") == "document" && query.Get("authuser") == "new" &&
		query.Get("response_type") == "code" && (mode == "" || mode == "query") &&
		query.Get("prompt") != "none"
}

func loginFrame(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Dest") == "iframe" && r.Header.Get("Sec-Fetch-Site") == "same-origin" &&
		r.URL.Query().Get("__idmux_login_frame") == "1"
}

func writeLoginQueue(w http.ResponseWriter, formPath string) {
	nonce := rand.Text()
	header := w.Header()
	header.Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; style-src 'nonce-"+nonce+"'; frame-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, strings.NewReplacer("IDMUX_NONCE", nonce, "IDMUX_FORM_PATH", html.EscapeString(formPath)).Replace(loginQueuePage))
}

// Let the coordinator finish the RP callback as a top-level navigation.
// The proxy has already processed the IdP cookies on this response.
func frameRedirect(response *http.Response) {
	if response.StatusCode != http.StatusFound && response.StatusCode != http.StatusSeeOther {
		return
	}
	location := response.Header.Get("Location")
	if location == "" {
		return
	}
	body := "<!doctype html><meta name=\"idmux-login-redirect\" content=\"" + html.EscapeString(location) + "\">"
	_ = response.Body.Close()
	response.Body = io.NopCloser(strings.NewReader(body))
	response.StatusCode = http.StatusOK
	response.Status = "200 OK"
	response.ContentLength = int64(len(body))
	for _, key := range []string{"Location", "Content-Encoding", "ETag", "Refresh", "Content-Disposition"} {
		response.Header.Del(key)
	}
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response.Header.Set("Content-Type", "text/html; charset=utf-8")
	response.Header.Set("Cache-Control", "no-store")
	response.Header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self'; base-uri 'none'")
	response.Header.Set("X-Frame-Options", "SAMEORIGIN")
	response.Header.Set("Referrer-Policy", "no-referrer")
	response.Header.Set("X-Content-Type-Options", "nosniff")
}
