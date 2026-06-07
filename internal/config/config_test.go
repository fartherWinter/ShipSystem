package config

import "testing"

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
