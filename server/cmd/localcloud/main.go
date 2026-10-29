package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asb131/local-cloud/internal/bootstrap"
	"github.com/asb131/local-cloud/internal/config"
	"github.com/asb131/local-cloud/internal/db"
	"github.com/asb131/local-cloud/internal/httpapi"
	"github.com/asb131/local-cloud/internal/jobs"
	"github.com/asb131/local-cloud/internal/preview"
	"github.com/asb131/local-cloud/internal/storage"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		return 1
	}

	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	ctx := context.Background()

	if err := db.RunMigrations(db.MigrateURL(cfg.DatabaseURL)); err != nil {
		logger.Error("migrations failed", "err", err)
		return 1
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connect failed", "err", err)
		return 1
	}
	defer pool.Close()

	layout, err := storage.NewLayout(cfg.DataDir)
	if err != nil {
		logger.Error("data dir", "err", err)
		return 1
	}
	if err := layout.EnsureBaseDirs(); err != nil {
		logger.Error("data dir init", "err", err)
		return 1
	}

	if err := bootstrap.EnsureAdmin(ctx, pool, cfg, layout); err != nil {
		logger.Error("bootstrap", "err", err)
		if !cfg.BootstrapConfigured() {
			logger.Warn("readyz will fail until LOCAL_CLOUD_ADMIN_* is set and users table is empty")
		}
	}
	if err := bootstrap.SyncBootstrapAdminPassword(ctx, pool, cfg); err != nil {
		logger.Error("bootstrap admin password sync", "err", err)
		return 1
	}

	srv := httpapi.New(logger, cfg, pool, layout)
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	jobs.RunScheduler(runCtx, logger, pool, layout, srv.FilesStore())
	go (&preview.Worker{Pool: pool, Layout: layout, Hub: srv.Hub(), Log: logger}).Run(runCtx)
	go (&preview.OfficeWorker{Pool: pool, Layout: layout, Hub: srv.Hub(), Log: logger}).Run(runCtx)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if cfg.TLSCertFile != "" {
		tlsCfg, err := buildTLSConfig(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			logger.Error("tls config", "err", err)
			return 1
		}
		httpServer.TLSConfig = tlsCfg
	}

	go func() {
		logger.Info("security: LAN-only client filter enabled (always on)")
		if cfg.TLSCertFile != "" {
			logger.Info("listening with TLS", "addr", cfg.HTTPAddr, "cert", cfg.TLSCertFile)
		} else {
			logger.Info("listening", "addr", cfg.HTTPAddr)
		}
		var err error
		if cfg.TLSCertFile != "" {
			err = httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
		return 1
	}
	return 0
}

func buildTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
