package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/4nass/idmux-proxy/internal/config"
	"github.com/4nass/idmux-proxy/internal/proxy"
)

const (
	serverReadTimeout   = 30 * time.Second
	serverWriteGrace    = 10 * time.Second
	serverHeaderTimeout = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	handler, err := proxy.New(cfg.UpstreamURL, proxy.Config{
		IDPCookieName:       cfg.IDPCookieName,
		SessionCookieName:   cfg.SessionCookieName,
		CookieKeys:          cfg.CookieKeys,
		CookieSecure:        cfg.CookieSecure,
		CookieSameSite:      cfg.CookieSameSite,
		SessionTTL:          cfg.SessionTTL,
		MaxSessions:         cfg.MaxSessions,
		MaxRequestBodyBytes: cfg.MaxRequestBodyBytes,
		UpstreamTimeout:     cfg.UpstreamTimeout,
		LogoutPathPrefix:    cfg.LogoutPathPrefix,
		ControlPathPrefix:   cfg.ControlPathPrefix,
		TrustedOrigins:      cfg.TrustedOrigins,
		Logger:              logger,
	})
	if err != nil {
		logger.Error("proxy setup failed", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: serverHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      cfg.UpstreamTimeout + serverReadTimeout + serverWriteGrace,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	contextValue, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("proxy started", "listen_addr", cfg.ListenAddr, "upstream_host", cfg.UpstreamURL.Host)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("proxy stopped unexpectedly", "error", err)
			stop()
		}
	}()

	<-contextValue.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := server.Shutdown(shutdownContext); err != nil {
		cancel()
		stop()
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	cancel()
	stop()
	logger.Info("proxy stopped")
}
