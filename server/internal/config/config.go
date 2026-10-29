package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort          = 8090
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin"
	minBootstrapPassword = 5
)

type Config struct {
	DatabaseURL         string
	HTTPAddr            string
	DataDir             string
	JWTSecret           string
	AdminUsername       string
	AdminPassword       string
	AdminQuotaBytes     int64
	LogLevel            string
	// TrustedProxyCIDRs is comma-separated CIDRs; X-Forwarded-For is honored only when RemoteAddr is in this list.
	TrustedProxyCIDRs string
	// TLSCertFile and TLSKeyFile enable HTTPS when both are set (recommended on LAN against passive sniffing).
	TLSCertFile string
	TLSKeyFile  string
	// OnlyOfficePublicURL is the browser-reachable Document Server origin (optional).
	OnlyOfficePublicURL string
	// InternalBaseURL is how other containers reach this API (e.g. http://localcloud-api:8090).
	InternalBaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DataDir:         strings.TrimSpace(os.Getenv("LOCAL_CLOUD_DATA_DIR")),
		JWTSecret:       strings.TrimSpace(os.Getenv("LOCAL_CLOUD_JWT_SECRET")),
		AdminUsername:   strings.TrimSpace(os.Getenv("LOCAL_CLOUD_ADMIN_USERNAME")),
		AdminPassword:   os.Getenv("LOCAL_CLOUD_ADMIN_PASSWORD"),
		LogLevel:        strings.TrimSpace(os.Getenv("LOCAL_CLOUD_LOG_LEVEL")),
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	httpAddr := strings.TrimSpace(os.Getenv("LOCAL_CLOUD_HTTP_ADDR"))
	if httpAddr == "" {
		port := defaultPort
		if p := strings.TrimSpace(os.Getenv("LOCAL_CLOUD_PORT")); p != "" {
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 || n > 65535 {
				return Config{}, fmt.Errorf("LOCAL_CLOUD_PORT: invalid port %q", p)
			}
			port = n
		}
		httpAddr = fmt.Sprintf(":%d", port)
	} else if !strings.Contains(httpAddr, ":") {
		return Config{}, fmt.Errorf("LOCAL_CLOUD_HTTP_ADDR: must include host or leading colon")
	}
	cfg.HTTPAddr = httpAddr

	quota := int64(107374182400)
	if q := strings.TrimSpace(os.Getenv("LOCAL_CLOUD_ADMIN_QUOTA_BYTES")); q != "" {
		n, err := strconv.ParseInt(q, 10, 64)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("LOCAL_CLOUD_ADMIN_QUOTA_BYTES: invalid value")
		}
		quota = n
	}
	cfg.AdminQuotaBytes = quota

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DataDir == "" {
		return Config{}, fmt.Errorf("LOCAL_CLOUD_DATA_DIR is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("LOCAL_CLOUD_JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("LOCAL_CLOUD_JWT_SECRET must be at least 32 characters")
	}

	if cfg.AdminUsername == "" {
		cfg.AdminUsername = defaultAdminUsername
	}
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = defaultAdminPassword
	}

	cfg.TrustedProxyCIDRs = strings.TrimSpace(os.Getenv("LOCAL_CLOUD_TRUSTED_PROXY_CIDRS"))
	cfg.TLSCertFile = strings.TrimSpace(os.Getenv("LOCAL_CLOUD_TLS_CERT"))
	cfg.TLSKeyFile = strings.TrimSpace(os.Getenv("LOCAL_CLOUD_TLS_KEY"))
	cfg.OnlyOfficePublicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("LOCAL_CLOUD_ONLYOFFICE_PUBLIC_URL")), "/")
	cfg.InternalBaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("LOCAL_CLOUD_INTERNAL_BASE_URL")), "/")

	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return Config{}, fmt.Errorf("LOCAL_CLOUD_TLS_CERT and LOCAL_CLOUD_TLS_KEY must both be set or both empty")
	}

	return cfg, nil
}

func (c Config) BootstrapConfigured() bool {
	return c.AdminUsername != "" && len(c.AdminPassword) >= minBootstrapPassword
}
