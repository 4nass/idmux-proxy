package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/4nass/idmux-proxy/internal/session"
)

type Config struct {
	IDPCookieName     string
	SessionCookieName string
	CookieKeys        [][]byte
	CookieSecure      bool
	CookieSameSite    string
	SessionTTL        time.Duration
	MaxSessions       int
	LogoutPathPrefix  string
	ControlPathPrefix string
	TrustedOrigins    []string
	Logger            *slog.Logger
}

type Handler struct {
	cfg    Config
	sealer *session.Sealer
	proxy  *httputil.ReverseProxy
	logger *slog.Logger
}

type requestState struct {
	state   session.State
	target  session.Target
	logout  bool
	changed bool
}

type requestStateKey struct{}

func New(upstream *url.URL, cfg Config) (*Handler, error) {
	if upstream == nil || upstream.Host == "" {
		return nil, errors.New("upstream URL is required")
	}
	if cfg.IDPCookieName == "" || cfg.SessionCookieName == "" {
		return nil, errors.New("cookie names are required")
	}
	if cfg.IDPCookieName == cfg.SessionCookieName {
		return nil, errors.New("IdP and composite cookie names must be different")
	}
	if strings.ContainsAny(cfg.IDPCookieName, " ;,\t\r\n") || strings.ContainsAny(cfg.SessionCookieName, " ;,\t\r\n") {
		return nil, errors.New("cookie names contain invalid characters")
	}
	if cfg.MaxSessions < 1 {
		return nil, errors.New("maximum session count must be positive")
	}
	if cfg.SessionTTL <= 0 {
		return nil, errors.New("session lifetime must be positive")
	}
	sealer, err := session.NewSealer(cfg.CookieKeys...)
	if err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &Handler{cfg: cfg, sealer: sealer, logger: logger}
	reverseProxy := &httputil.ReverseProxy{}
	reverseProxy.Rewrite = func(proxyRequest *httputil.ProxyRequest) {
		proxyRequest.SetURL(upstream)
		req := proxyRequest.Out
		value, _ := req.Context().Value(requestStateKey{}).(*requestState)
		if value == nil {
			return
		}

		clientCookies := req.Cookies()
		forwardCookies := make([]string, 0, len(clientCookies)+1)
		for _, cookie := range clientCookies {
			if cookie.Name == h.cfg.IDPCookieName || cookie.Name == h.cfg.SessionCookieName {
				continue
			}
			forwardCookies = append(forwardCookies, cookie.Name+"="+cookie.Value)
		}
		if !value.target.New {
			if entry, ok := value.state.Entry(value.target.Index); ok {
				forwardCookies = append(forwardCookies, h.cfg.IDPCookieName+"="+entry.IDPSessionID)
			}
		}
		if len(forwardCookies) == 0 {
			req.Header.Del("Cookie")
		} else {
			req.Header.Set("Cookie", strings.Join(forwardCookies, "; "))
		}
		canonicalizeRoutingQuery(req.URL, value.target)
		removeSensitiveBoundaryHeaders(req.Header)
	}
	reverseProxy.ModifyResponse = h.modifyResponse
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		h.logger.Error("upstream request failed", "path", r.URL.Path, "error", err.Error())
		http.Error(w, "upstream request failed", http.StatusBadGateway)
	}
	h.proxy = reverseProxy
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.serveControl(w, r) {
		return
	}

	state, err := h.readState(r)
	if err != nil {
		h.logger.Warn("invalid session cookie", "path", r.URL.Path, "error", err.Error())
		h.invalidCookieResponse(w, r)
		return
	}

	requestedTarget, err := session.ResolveTarget(r)
	if err != nil {
		h.logger.Warn("invalid session routing input", "path", r.URL.Path, "error", err.Error())
		requestedTarget = session.Target{Index: 0}
	}
	target, fallback := fallbackTarget(requestedTarget, state)
	if fallback {
		h.logger.Info("used deterministic session fallback", "path", r.URL.Path, "index", target.Index, "new", target.New)
	}
	if err := validateTarget(target, state, h.cfg.MaxSessions); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request := &requestState{
		state:  state,
		target: target,
		logout: h.isLogoutPath(r.URL.Path),
	}
	if !target.New && state.ActiveIndex != target.Index {
		request.state.ActiveIndex = target.Index
		request.changed = true
	}

	requestContext := context.WithValue(r.Context(), requestStateKey{}, request)
	h.proxy.ServeHTTP(w, r.Clone(requestContext))
}

func (h *Handler) readState(r *http.Request) (session.State, error) {
	cookie, err := r.Cookie(h.cfg.SessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return session.NewState(), nil
	}
	if err != nil {
		return session.State{}, err
	}
	return h.sealer.Open(cookie.Value, h.cfg.MaxSessions)
}

func fallbackTarget(target session.Target, state session.State) (session.Target, bool) {
	if target.New {
		return target, false
	}
	if _, ok := state.Entry(target.Index); ok {
		return target, false
	}
	if _, ok := state.Entry(0); ok {
		return session.Target{Index: 0}, true
	}
	return session.Target{New: true, Index: -1}, true
}

func validateTarget(target session.Target, state session.State, maxSessions int) error {
	if target.New {
		if state.ActiveCount() >= maxSessions {
			return errors.New("maximum session count reached; log out one session first")
		}
		return nil
	}
	if _, ok := state.Entry(target.Index); !ok {
		return errors.New("requested session does not exist")
	}
	return nil
}

func (h *Handler) modifyResponse(response *http.Response) error {
	if response.Request == nil {
		return errors.New("upstream response has no request")
	}
	request, _ := response.Request.Context().Value(requestStateKey{}).(*requestState)
	if request == nil {
		return nil
	}
	state := request.state.Clone()
	changed := request.changed
	if request.logout && !request.target.New {
		changed = state.Remove(request.target.Index) || changed
	}

	outgoing := make([]string, 0, len(response.Header.Values("Set-Cookie"))+1)
	for _, raw := range response.Header.Values("Set-Cookie") {
		name := rawCookieName(raw)
		if name == h.cfg.SessionCookieName {
			changed = true
			continue
		}
		if name != h.cfg.IDPCookieName {
			outgoing = append(outgoing, raw)
			continue
		}

		parsed, ok := parseSetCookie(raw)
		if !ok {
			return errors.New("upstream returned an invalid IdP Set-Cookie header")
		}
		changed = true
		if request.logout || isDeletionCookie(parsed) {
			if !request.logout && !request.target.New {
				state.Remove(request.target.Index)
			}
			continue
		}
		if request.target.New {
			if _, err := state.Add(parsed.Value, h.cfg.MaxSessions); err != nil {
				return fmt.Errorf("store new IdP session: %w", err)
			}
			request.target = session.Target{Index: state.ActiveIndex}
			continue
		}
		if err := state.Upsert(request.target.Index, parsed.Value, h.cfg.MaxSessions); err != nil {
			return fmt.Errorf("store IdP session: %w", err)
		}
	}

	removeSensitiveResponseHeaders(response.Header)
	if containsCookiesDirective(response.Header.Values("Clear-Site-Data")) {
		response.Header.Del("Clear-Site-Data")
	}
	if changed {
		if state.ActiveCount() == 0 {
			outgoing = append(outgoing, h.expireCookie().String())
		} else {
			now := time.Now()
			state.IssuedAt = now.Unix()
			state.ExpiresAt = now.Add(h.cfg.SessionTTL).Unix()
			value, err := h.sealer.Seal(state)
			if err != nil {
				return fmt.Errorf("seal session state: %w", err)
			}
			outgoing = append(outgoing, h.sessionCookie(value, now).String())
		}
	}
	response.Header.Del("Set-Cookie")
	for _, raw := range outgoing {
		response.Header.Add("Set-Cookie", raw)
	}
	return nil
}

func (h *Handler) serveControl(w http.ResponseWriter, r *http.Request) bool {
	prefix := strings.TrimRight(h.cfg.ControlPathPrefix, "/")
	switch r.URL.Path {
	case prefix + "/healthz", prefix + "/readyz":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
		return true
	case prefix + "/sessions":
		h.serveSessions(w, r)
		return true
	default:
		return false
	}
}

func (h *Handler) serveSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !h.allowedOrigin(r) {
		http.Error(w, "origin is not trusted", http.StatusForbidden)
		return
	}
	state, err := h.readState(r)
	if err != nil {
		h.logger.Warn("invalid session cookie", "path", r.URL.Path, "error", err.Error())
		h.invalidCookieResponse(w, r)
		return
	}
	type publicEntry struct {
		Index       int    `json:"index"`
		UserID      string `json:"user_id,omitempty"`
		DisplayName string `json:"display_name,omitempty"`
		CreatedAt   int64  `json:"created_at"`
	}
	response := struct {
		ActiveIndex int           `json:"active_index"`
		Sessions    []publicEntry `json:"sessions"`
	}{ActiveIndex: state.ActiveIndex, Sessions: make([]publicEntry, 0, state.ActiveCount())}
	for _, entry := range state.Sessions {
		if entry.IDPSessionID == "" {
			continue
		}
		response.Sessions = append(response.Sessions, publicEntry{
			Index:       entry.Index,
			UserID:      entry.UserID,
			DisplayName: entry.DisplayName,
			CreatedAt:   entry.CreatedAt,
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *Handler) allowedOrigin(r *http.Request) bool {
	for _, header := range []string{"Origin", "Referer"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return false
		}
		origin := parsed.Scheme + "://" + parsed.Host
		trustedOrigin := false
		for _, trusted := range h.cfg.TrustedOrigins {
			if strings.TrimRight(trusted, "/") == origin {
				trustedOrigin = true
				break
			}
		}
		if !trustedOrigin {
			return false
		}
	}
	return true
}

func (h *Handler) invalidCookieResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Set-Cookie", h.expireCookie().String())
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		query := r.URL.Query()
		query.Set("authuser", "new")
		location := &url.URL{
			Path:     r.URL.Path,
			RawPath:  r.URL.RawPath,
			RawQuery: query.Encode(),
		}
		http.Redirect(w, r, location.String(), http.StatusSeeOther)
		return
	}
	http.Error(w, "invalid session cookie", http.StatusBadRequest)
}

func (h *Handler) sessionCookie(value string, now time.Time) *http.Cookie {
	cookie := &http.Cookie{ // #nosec G124 -- CookieSecure is disabled only for development over plain HTTP; production config rejects it.
		Name:     h.cfg.SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  now.Add(h.cfg.SessionTTL),
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
	}
	if !h.cfg.CookieSecure {
		cookie.Secure = false
	}
	cookie.SameSite = sameSite(h.cfg.CookieSameSite)
	return cookie
}

func (h *Handler) expireCookie() *http.Cookie {
	cookie := &http.Cookie{ // #nosec G124 -- CookieSecure is disabled only for development over plain HTTP; production config rejects it.
		Name:     h.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	}
	if !h.cfg.CookieSecure {
		cookie.Secure = false
	}
	cookie.SameSite = sameSite(h.cfg.CookieSameSite)
	return cookie
}

func sameSite(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func (h *Handler) isLogoutPath(path string) bool {
	prefix := strings.TrimRight(h.cfg.LogoutPathPrefix, "/")
	return prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/"))
}

func rawCookieName(raw string) string {
	part, _, _ := strings.Cut(raw, "=")
	return strings.TrimSpace(part)
}

func parseSetCookie(raw string) (*http.Cookie, bool) {
	response := &http.Response{Header: http.Header{"Set-Cookie": []string{raw}}}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		return nil, false
	}
	return cookies[0], true
}

func isDeletionCookie(cookie *http.Cookie) bool {
	return cookie.Value == "" || cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(time.Now()))
}

func containsCookiesDirective(values []string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), "cookies") {
			return true
		}
	}
	return false
}

func removeSensitiveResponseHeaders(header http.Header) {
	removeSensitiveBoundaryHeaders(header)
}

func removeSensitiveBoundaryHeaders(header http.Header) {
	for key := range header {
		lower := strings.ToLower(key)
		if lower == "x-auth-user-index" ||
			strings.HasPrefix(lower, "x-idmux-internal-") ||
			strings.HasPrefix(lower, "x-forwarded-") ||
			lower == "x-real-ip" ||
			lower == "forwarded" {
			delete(header, key)
		}
	}
}

func canonicalizeRoutingQuery(requestURL *url.URL, target session.Target) {
	query := requestURL.Query()
	value := strconv.Itoa(target.Index)
	if target.New {
		value = "new"
	}
	query.Set("authuser", value)
	requestURL.RawQuery = query.Encode()
}
