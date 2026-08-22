package proxy

import (
	"net/url"
	"testing"
	"time"
)

func TestNewRejectsUnsafeSecurityConfiguration(t *testing.T) {
	validURL, err := url.Parse("https://idp.example.test")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		upstream *url.URL
		config   Config
	}{
		{
			name:     "unsupported upstream scheme",
			upstream: &url.URL{Scheme: "ftp", Host: "idp.example.test"},
			config:   secureTestConfig(),
		},
		{
			name:     "upstream credentials",
			upstream: &url.URL{Scheme: "https", Host: "idp.example.test", User: url.UserPassword("user", "secret")},
			config:   secureTestConfig(),
		},
		{
			name:     "invalid IdP cookie name",
			upstream: validURL,
			config: func() Config {
				config := secureTestConfig()
				config.IDPCookieName = "idp=invalid"
				return config
			}(),
		},
		{
			name:     "invalid SameSite value",
			upstream: validURL,
			config: func() Config {
				config := secureTestConfig()
				config.CookieSameSite = "unsafe"
				return config
			}(),
		},
		{
			name:     "insecure SameSite none",
			upstream: validURL,
			config: func() Config {
				config := secureTestConfig()
				config.CookieSecure = false
				config.CookieSameSite = "none"
				return config
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.upstream, test.config); err == nil {
				t.Fatal("expected configuration to be rejected")
			}
		})
	}
}

func TestNewNormalizesDefaultSameSite(t *testing.T) {
	upstream, err := url.Parse("https://idp.example.test")
	if err != nil {
		t.Fatal(err)
	}
	config := secureTestConfig()
	config.CookieSameSite = ""

	handler, err := New(upstream, config)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	if handler.cfg.CookieSameSite != "lax" {
		t.Fatalf("expected lax default, got %q", handler.cfg.CookieSameSite)
	}
}

func secureTestConfig() Config {
	return Config{
		IDPCookieName:     "idp",
		SessionCookieName: "IDMUX_SESSION",
		CookieKeys:        [][]byte{[]byte("01234567890123456789012345678901")},
		CookieSecure:      true,
		CookieSameSite:    "lax",
		SessionTTL:        time.Hour,
		MaxSessions:       2,
	}
}
