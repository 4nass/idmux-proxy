package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment       string
	ListenAddr        string
	UpstreamURL       *url.URL
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
}

func Load() (Config, error) {
	environment := strings.ToLower(readString("IDMUX_ENV", "production"))
	if environment != "production" && environment != "development" {
		return Config{}, errors.New("IDMUX_ENV must be production or development")
	}

	upstreamValue := strings.TrimSpace(os.Getenv("IDMUX_UPSTREAM_URL"))
	if upstreamValue == "" {
		return Config{}, errors.New("IDMUX_UPSTREAM_URL is required")
	}
	upstream, err := url.Parse(upstreamValue)
	if err != nil || upstream.Host == "" || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		return Config{}, errors.New("IDMUX_UPSTREAM_URL must be an http or https URL with a host")
	}

	keys, err := readKeys()
	if err != nil {
		return Config{}, err
	}
	secure, err := readBool("IDMUX_COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}
	if !secure && environment != "development" {
		return Config{}, errors.New("IDMUX_COOKIE_SECURE=false is allowed only in development")
	}
	sameSite := strings.ToLower(readString("IDMUX_COOKIE_SAMESITE", "lax"))
	if sameSite != "lax" && sameSite != "strict" && sameSite != "none" {
		return Config{}, errors.New("IDMUX_COOKIE_SAMESITE must be lax, strict, or none")
	}
	if sameSite == "none" && !secure {
		return Config{}, errors.New("IDMUX_COOKIE_SAMESITE=none requires IDMUX_COOKIE_SECURE=true")
	}
	ttl, err := time.ParseDuration(readString("IDMUX_SESSION_TTL", "12h"))
	if err != nil || ttl <= 0 {
		return Config{}, errors.New("IDMUX_SESSION_TTL must be a positive duration")
	}
	maxSessions, err := readInt("IDMUX_MAX_SESSIONS", 8)
	if err != nil || maxSessions < 1 || maxSessions > 32 {
		return Config{}, errors.New("IDMUX_MAX_SESSIONS must be between 1 and 32")
	}

	return Config{
		Environment:       environment,
		ListenAddr:        readString("IDMUX_LISTEN_PORT", ":9000"),
		UpstreamURL:       upstream,
		IDPCookieName:     readString("IDMUX_COOKIE_NAME", "KEYCLOAK_IDENTITY"),
		SessionCookieName: readString("IDMUX_SESSION_COOKIE_NAME", "IDMUX_SESSION"),
		CookieKeys:        keys,
		CookieSecure:      secure,
		CookieSameSite:    sameSite,
		SessionTTL:        ttl,
		MaxSessions:       maxSessions,
		LogoutPathPrefix:  readString("IDMUX_LOGOUT_PATH_PREFIX", "/protocol/openid-connect/logout"),
		ControlPathPrefix: readString("IDMUX_CONTROL_PATH_PREFIX", "/__idmux"),
		TrustedOrigins:    readList("IDMUX_TRUSTED_ORIGINS"),
	}, nil
}

func readKeys() ([][]byte, error) {
	active := strings.TrimSpace(os.Getenv("IDMUX_ENCRYPTION_KEY"))
	if active == "" {
		active = strings.TrimSpace(os.Getenv("IDMUX_SECRET_KEY"))
	}
	if active == "" {
		return nil, errors.New("IDMUX_ENCRYPTION_KEY is required")
	}

	values := []string{active}
	for _, value := range strings.Split(os.Getenv("IDMUX_ENCRYPTION_KEY_PREVIOUS"), ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	keys := make([][]byte, 0, len(values))
	for index, value := range values {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("encryption key %d must decode to exactly 32 bytes", index)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate cookie key: %w", err)
	}
	return key, nil
}

func readString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func readBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func readInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func readList(name string) []string {
	values := strings.Split(os.Getenv(name), ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
