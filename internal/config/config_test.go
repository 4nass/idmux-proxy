package config

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestLoadUsesPrefixedEnvironmentAndPreviousKeys(t *testing.T) {
	active := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	previous := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	t.Setenv("IDMUX_ENV", "production")
	t.Setenv("IDMUX_UPSTREAM_URL", "https://idp.example.test")
	t.Setenv("IDMUX_ENCRYPTION_KEY", active)
	t.Setenv("IDMUX_ENCRYPTION_KEY_PREVIOUS", previous)
	t.Setenv("IDMUX_COOKIE_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ListenAddr != ":9000" || cfg.SessionCookieName != "IDMUX_SESSION" || len(cfg.CookieKeys) != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsInsecureProduction(t *testing.T) {
	t.Setenv("IDMUX_ENV", "production")
	t.Setenv("IDMUX_UPSTREAM_URL", "https://idp.example.test")
	t.Setenv("IDMUX_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")))
	t.Setenv("IDMUX_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("expected insecure production configuration to be rejected")
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
