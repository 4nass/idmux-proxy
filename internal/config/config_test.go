package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUsesPrefixedEnvironmentAndPreviousKeys(t *testing.T) {
	active := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	previous := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	t.Setenv("IDMUX_ENV", "production")
	t.Setenv("IDMUX_UPSTREAM_URL", "https://idp.example.test")
	t.Setenv("IDMUX_ENCRYPTION_KEY", active)
	t.Setenv("IDMUX_ENCRYPTION_KEY_FILE", "")
	t.Setenv("IDMUX_ENCRYPTION_KEY_PREVIOUS", previous)
	t.Setenv("IDMUX_COOKIE_SECURE", "true")
	t.Setenv("IDMUX_MAX_REQUEST_BODY_BYTES", "")
	t.Setenv("IDMUX_UPSTREAM_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ListenAddr != ":9000" || cfg.SessionCookieName != "IDMUX_SESSION" || len(cfg.CookieKeys) != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.MaxRequestBodyBytes != DefaultMaxRequestBodyBytes {
		t.Fatalf("MaxRequestBodyBytes=%d, want=%d", cfg.MaxRequestBodyBytes, DefaultMaxRequestBodyBytes)
	}
	if cfg.UpstreamTimeout != DefaultUpstreamTimeout {
		t.Fatalf("UpstreamTimeout=%s, want=%s", cfg.UpstreamTimeout, DefaultUpstreamTimeout)
	}
}

func TestLoadRejectsInvalidUpstreamTimeout(t *testing.T) {
	for _, value := range []string{"0", "500ms", "invalid", "5m1s"} {
		t.Run(value, func(t *testing.T) {
			setValidConfigEnvironment(t)
			t.Setenv("IDMUX_UPSTREAM_TIMEOUT", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted invalid upstream timeout %q", value)
			}
		})
	}
}

func TestLoadAcceptsConfiguredUpstreamTimeout(t *testing.T) {
	setValidConfigEnvironment(t)
	t.Setenv("IDMUX_UPSTREAM_TIMEOUT", "2m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.UpstreamTimeout != 2*time.Minute {
		t.Fatalf("UpstreamTimeout=%s, want=2m0s", cfg.UpstreamTimeout)
	}
}

func TestLoadRejectsInvalidRequestBodyLimit(t *testing.T) {
	for _, value := range []string{"0", "-1", "invalid", "1073741825", "9223372036854775808"} {
		t.Run(value, func(t *testing.T) {
			setValidConfigEnvironment(t)
			t.Setenv("IDMUX_MAX_REQUEST_BODY_BYTES", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted invalid request body limit %q", value)
			}
		})
	}
}

func TestLoadAcceptsConfiguredRequestBodyLimit(t *testing.T) {
	setValidConfigEnvironment(t)
	t.Setenv("IDMUX_MAX_REQUEST_BODY_BYTES", "16777216")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.MaxRequestBodyBytes != 16<<20 {
		t.Fatalf("MaxRequestBodyBytes=%d, want=%d", cfg.MaxRequestBodyBytes, 16<<20)
	}
}

func TestReadKeysFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idmux-key")
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IDMUX_ENCRYPTION_KEY", "")
	t.Setenv("IDMUX_ENCRYPTION_KEY_FILE", path)
	t.Setenv("IDMUX_SECRET_KEY", "")
	t.Setenv("IDMUX_ENCRYPTION_KEY_PREVIOUS", "")

	keys, err := readKeys()
	if err != nil {
		t.Fatalf("read keys: %v", err)
	}
	if len(keys) != 1 || string(keys[0]) != "01234567890123456789012345678901" {
		t.Fatalf("unexpected keys: count=%d", len(keys))
	}
}

func TestReadKeysRejectsEnvironmentAndFileTogether(t *testing.T) {
	t.Setenv("IDMUX_ENCRYPTION_KEY", "not-logged")
	t.Setenv("IDMUX_ENCRYPTION_KEY_FILE", filepath.Join(t.TempDir(), "key"))
	if _, err := readKeys(); err == nil {
		t.Fatal("expected conflicting key sources to be rejected")
	}
}

func setValidConfigEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("IDMUX_ENV", "production")
	t.Setenv("IDMUX_UPSTREAM_URL", "https://idp.example.test")
	t.Setenv("IDMUX_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")))
	t.Setenv("IDMUX_ENCRYPTION_KEY_FILE", "")
	t.Setenv("IDMUX_COOKIE_SECURE", "true")
	t.Setenv("IDMUX_COOKIE_SAMESITE", "lax")
	t.Setenv("IDMUX_SESSION_TTL", "12h")
	t.Setenv("IDMUX_MAX_SESSIONS", "8")
	t.Setenv("IDMUX_UPSTREAM_TIMEOUT", "")
}

func TestLoadRejectsInsecureProduction(t *testing.T) {
	t.Setenv("IDMUX_ENV", "production")
	t.Setenv("IDMUX_UPSTREAM_URL", "https://idp.example.test")
	t.Setenv("IDMUX_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")))
	t.Setenv("IDMUX_ENCRYPTION_KEY_FILE", "")
	t.Setenv("IDMUX_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("expected insecure production configuration to be rejected")
	}
}

func TestLoadRejectsHTTPUpstreamInProduction(t *testing.T) {
	setValidConfigEnvironment(t)
	t.Setenv("IDMUX_UPSTREAM_URL", "http://idp.example.test")
	if _, err := Load(); err == nil {
		t.Fatal("expected production HTTP upstream to be rejected")
	}
}

func TestLoadAllowsHTTPUpstreamInDevelopment(t *testing.T) {
	setValidConfigEnvironment(t)
	t.Setenv("IDMUX_ENV", "development")
	t.Setenv("IDMUX_UPSTREAM_URL", "http://idp.example.test")
	t.Setenv("IDMUX_COOKIE_SECURE", "false")

	if _, err := Load(); err != nil {
		t.Fatalf("load development config: %v", err)
	}
}

func TestLoadDoesNotUseLegacyEnvironmentNames(t *testing.T) {
	for _, name := range []string{
		"UPSTREAM_URL",
		"COOKIE_KEY_BASE64",
		"IDP_COOKIE_NAME",
	} {
		_ = os.Unsetenv(name)
	}
	t.Setenv("IDMUX_UPSTREAM_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing prefixed upstream configuration to fail")
	}
}
