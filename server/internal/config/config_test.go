package config

import (
	"testing"
)

func TestLoadDerivesHTTPAddrFromPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/cloud?sslmode=disable")
	t.Setenv("LOCAL_CLOUD_DATA_DIR", "/data")
	t.Setenv("LOCAL_CLOUD_JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("LOCAL_CLOUD_PORT", "9000")
	t.Setenv("LOCAL_CLOUD_HTTP_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
}

func TestLoadDefaultPort8090(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/cloud?sslmode=disable")
	t.Setenv("LOCAL_CLOUD_DATA_DIR", "/data")
	t.Setenv("LOCAL_CLOUD_JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("LOCAL_CLOUD_PORT", "")
	t.Setenv("LOCAL_CLOUD_HTTP_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
}

func TestLoadDefaultAdminCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/cloud?sslmode=disable")
	t.Setenv("LOCAL_CLOUD_DATA_DIR", "/data")
	t.Setenv("LOCAL_CLOUD_JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("LOCAL_CLOUD_ADMIN_USERNAME", "")
	t.Setenv("LOCAL_CLOUD_ADMIN_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminUsername != "admin" || cfg.AdminPassword != "admin" {
		t.Fatalf("defaults = %q / %q", cfg.AdminUsername, cfg.AdminPassword)
	}
	if !cfg.BootstrapConfigured() {
		t.Fatal("expected bootstrap configured with defaults")
	}
}
