package config

import "testing"

func TestLoadPrefersCanonicalEnvVars(t *testing.T) {
	t.Setenv("MANGASHELF_HTTP_PORT", "4100")
	t.Setenv("MANGASHELF_DATABASE_PATH", "/tmp/canonical.db")
	t.Setenv("MANGASHELF_JWT_SECRET", "canonical-secret")
	t.Setenv("MANGASHELF_CATBOX_USERHASH", "catbox-userhash")
	t.Setenv("MANGASHELF_DEV_WEB_URL", "http://localhost:5173/")
	t.Setenv("MANGASHELF_DEV_EXTENSION_URL", "http://localhost:38181/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.HTTPPort != "4100" {
		t.Fatalf("expected canonical HTTP port, got %q", cfg.HTTPPort)
	}
	if cfg.DatabasePath != "/tmp/canonical.db" {
		t.Fatalf("expected canonical database path, got %q", cfg.DatabasePath)
	}
	if cfg.JWTSecret != "canonical-secret" {
		t.Fatalf("expected canonical JWT secret, got %q", cfg.JWTSecret)
	}
	if cfg.JWTSecretGenerated {
		t.Fatal("expected configured JWT secret to not be marked as generated")
	}
	if cfg.CatboxUserHash != "catbox-userhash" {
		t.Fatalf("expected configured Catbox userhash, got %q", cfg.CatboxUserHash)
	}
	if cfg.DevWebURL != "http://localhost:5173" {
		t.Fatalf("expected trimmed dev web URL, got %q", cfg.DevWebURL)
	}
	if cfg.DevExtensionURL != "http://localhost:38181" {
		t.Fatalf("expected trimmed dev extension URL, got %q", cfg.DevExtensionURL)
	}
}

func TestLoadIgnoresLegacyEnvVars(t *testing.T) {
	t.Setenv("PORT", "4300")
	t.Setenv("DATABASE_PATH", "/tmp/legacy-only.db")
	t.Setenv("JWT_SECRET", "legacy-only-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.HTTPPort != "3001" {
		t.Fatalf("expected default HTTP port, got %q", cfg.HTTPPort)
	}
	if cfg.DatabasePath != "./data/mangashelf.db" {
		t.Fatalf("expected default database path, got %q", cfg.DatabasePath)
	}
	if cfg.JWTSecret == "legacy-only-secret" {
		t.Fatalf("expected legacy JWT secret to be ignored, got %q", cfg.JWTSecret)
	}
	if !cfg.JWTSecretGenerated {
		t.Fatal("expected generated JWT secret when canonical env var is unset")
	}
}

func TestLoadGeneratesJWTSecretWhenUnset(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.JWTSecret == "" {
		t.Fatal("expected generated JWT secret to be non-empty")
	}
	if !cfg.JWTSecretGenerated {
		t.Fatal("expected generated JWT secret to be marked as generated")
	}
}
