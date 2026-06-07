package config

import (
	"testing"
	"time"
)

func TestProductionRequiresAuth(t *testing.T) {
	cfg := Default()
	cfg.Environment = EnvProd
	cfg.AuthMode = AuthOff
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config without auth to fail")
	}
}

func TestTokenAuthRequiresToken(t *testing.T) {
	cfg := Default()
	cfg.AuthMode = AuthToken
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected token auth without token to fail")
	}
	cfg.AuthToken = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid token auth: %v", err)
	}
}

func TestDurationEnvOverrides(t *testing.T) {
	t.Setenv("SHIP_SIM_HTTP_READ_TIMEOUT", "7s")
	t.Setenv("SHIP_SIM_HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("SHIP_SIM_HTTP_WRITE_TIMEOUT", "11s")
	t.Setenv("SHIP_SIM_HTTP_IDLE_TIMEOUT", "45s")
	t.Setenv("SHIP_SIM_SHUTDOWN_TIMEOUT", "20s")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTPReadTimeout != 7*time.Second ||
		cfg.HTTPReadHeaderTimeout != 3*time.Second ||
		cfg.HTTPWriteTimeout != 11*time.Second ||
		cfg.HTTPIdleTimeout != 45*time.Second ||
		cfg.ShutdownTimeout != 20*time.Second {
		t.Fatalf("unexpected duration config: %+v", cfg)
	}
}

func TestInvalidDurationEnvFails(t *testing.T) {
	t.Setenv("SHIP_SIM_HTTP_READ_TIMEOUT", "soon")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid duration to fail")
	}
}

func TestTimeoutsMustBePositive(t *testing.T) {
	cfg := Default()
	cfg.ShutdownTimeout = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected zero shutdown timeout to fail")
	}
}
