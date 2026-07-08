package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsForDevelopment(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("JWT_TTL_HOURS", "")
	t.Setenv("REQUEST_BODY_LIMIT_BYTES", "")
	t.Setenv("HTTP_READ_TIMEOUT", "")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("HTTP_WRITE_TIMEOUT", "")
	t.Setenv("HTTP_IDLE_TIMEOUT", "")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	t.Setenv("ANALYTICS_HTTP_TIMEOUT", "")
	t.Setenv("DATABASE_AUTO_MIGRATE", "")
	t.Setenv("SEED_DEMO_DATA", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Environment != EnvDevelopment {
		t.Fatalf("expected development environment, got %q", cfg.Environment)
	}
	if !cfg.DatabaseAutoMigrate {
		t.Fatal("expected development auto migrate default to be enabled")
	}
	if !cfg.SeedDemoData {
		t.Fatal("expected development demo seed default to be enabled")
	}
	if cfg.RequestBodyLimit != 1<<20 {
		t.Fatalf("expected default request body limit 1048576, got %d", cfg.RequestBodyLimit)
	}
	if cfg.HTTPReadTimeout != 15*time.Second {
		t.Fatalf("expected default read timeout 15s, got %s", cfg.HTTPReadTimeout)
	}
	if cfg.HTTPHeaderTimeout != 5*time.Second {
		t.Fatalf("expected default header timeout 5s, got %s", cfg.HTTPHeaderTimeout)
	}
	if cfg.HTTPWriteTimeout != 30*time.Second {
		t.Fatalf("expected default write timeout 30s, got %s", cfg.HTTPWriteTimeout)
	}
	if cfg.HTTPIdleTimeout != 60*time.Second {
		t.Fatalf("expected default idle timeout 60s, got %s", cfg.HTTPIdleTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("expected default shutdown timeout 10s, got %s", cfg.ShutdownTimeout)
	}
	if cfg.AnalyticsHTTPTimeout != 10*time.Second {
		t.Fatalf("expected default analytics HTTP timeout 10s, got %s", cfg.AnalyticsHTTPTimeout)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults should validate: %v", err)
	}
}

func TestValidateRejectsProductionDefaults(t *testing.T) {
	cfg := Config{
		Environment:         EnvProduction,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		GoAPIToken:          "strong-go-api-callback-token",
		JWTSecret:           defaultJWTSecret,
		CORSOrigins:         []string{"http://localhost:3000"},
		AdminPassword:       defaultAdminPassword,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production defaults to be rejected")
	}
}

func TestValidateRejectsWildcardCORSInProduction(t *testing.T) {
	cfg := Config{
		Environment:         EnvProduction,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		GoAPIToken:          "strong-go-api-callback-token",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"*"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard production CORS to be rejected")
	}
}

func TestValidateRejectsProductionAutoMigrateAndDemoSeed(t *testing.T) {
	cfg := Config{
		Environment:         EnvProduction,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		DatabaseAutoMigrate: true,
		SeedDemoData:        true,
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		GoAPIToken:          "strong-go-api-callback-token",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"https://ships.example.com"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production auto migration and demo seed to be rejected")
	}
}

func TestLoadRejectsInvalidBoolean(t *testing.T) {
	t.Setenv("DATABASE_AUTO_MIGRATE", "maybe")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid boolean to be rejected")
	}
}

func TestLoadRejectsInvalidRequestBodyLimit(t *testing.T) {
	t.Setenv("REQUEST_BODY_LIMIT_BYTES", "abc")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid request body limit to be rejected")
	}
}

func TestLoadRejectsInvalidHTTPReadTimeout(t *testing.T) {
	t.Setenv("HTTP_READ_TIMEOUT", "bad-timeout")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid HTTP read timeout to be rejected")
	}
}

func TestLoadRejectsInvalidAnalyticsHTTPTimeout(t *testing.T) {
	t.Setenv("ANALYTICS_HTTP_TIMEOUT", "bad-timeout")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid analytics HTTP timeout to be rejected")
	}
}

func TestValidateRejectsTooSmallRequestBodyLimit(t *testing.T) {
	cfg := Config{
		Environment:         EnvDevelopment,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		RequestBodyLimit:    512,
		HTTPReadTimeout:     15 * time.Second,
		HTTPHeaderTimeout:   5 * time.Second,
		HTTPWriteTimeout:    30 * time.Second,
		HTTPIdleTimeout:     60 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		AnalyticsHTTPTimeout: 10 * time.Second,
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		GoAPIToken:          "strong-go-api-callback-token",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"http://localhost:3000"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected too-small request body limit to be rejected")
	}
}

func TestValidateRejectsNonPositiveHTTPTimeouts(t *testing.T) {
	cfg := Config{
		Environment:         EnvDevelopment,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		RequestBodyLimit:    1 << 20,
		HTTPReadTimeout:     0,
		HTTPHeaderTimeout:   5 * time.Second,
		HTTPWriteTimeout:    30 * time.Second,
		HTTPIdleTimeout:     60 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		AnalyticsHTTPTimeout: 10 * time.Second,
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		GoAPIToken:          "strong-go-api-callback-token",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"http://localhost:3000"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-positive timeout to be rejected")
	}
}

func TestValidateRejectsNonPositiveAnalyticsHTTPTimeout(t *testing.T) {
	cfg := Config{
		Environment:          EnvDevelopment,
		Port:                 "8080",
		DatabaseDSN:          "host=db user=shipsystem password=secret dbname=shipsystem",
		RequestBodyLimit:     1 << 20,
		HTTPReadTimeout:      15 * time.Second,
		HTTPHeaderTimeout:    5 * time.Second,
		HTTPWriteTimeout:     30 * time.Second,
		HTTPIdleTimeout:      60 * time.Second,
		ShutdownTimeout:      10 * time.Second,
		AnalyticsHTTPTimeout: 0,
		AnalyticsBaseURL:     "http://analytics:8090",
		AnalyticsAdminToken:  "strong-analytics-admin-token",
		GoAPIToken:           "strong-go-api-callback-token",
		JWTSecret:            "0123456789abcdef0123456789abcdef",
		CORSOrigins:          []string{"http://localhost:3000"},
		AdminPassword:        "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-positive analytics HTTP timeout to be rejected")
	}
}

func TestValidateRejectsProductionWithoutAnalyticsToken(t *testing.T) {
	cfg := Config{
		Environment:      EnvProduction,
		Port:             "8080",
		DatabaseDSN:      "host=db user=shipsystem password=secret dbname=shipsystem",
		AnalyticsBaseURL: "http://analytics:8090",
		JWTSecret:        "0123456789abcdef0123456789abcdef",
		GoAPIToken:       "strong-go-api-callback-token",
		CORSOrigins:      []string{"https://ships.example.com"},
		AdminPassword:    "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing analytics token to be rejected")
	}
}

func TestValidateRejectsProductionWithoutGoAPIToken(t *testing.T) {
	cfg := Config{
		Environment:         EnvProduction,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "strong-analytics-admin-token",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"https://ships.example.com"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing Go API token to be rejected")
	}
}

func TestValidateRejectsSharedServiceTokensInProduction(t *testing.T) {
	cfg := Config{
		Environment:         EnvProduction,
		Port:                "8080",
		DatabaseDSN:         "host=db user=shipsystem password=secret dbname=shipsystem",
		AnalyticsBaseURL:    "http://analytics:8090",
		AnalyticsAdminToken: "shared-service-token-value",
		GoAPIToken:          "shared-service-token-value",
		JWTSecret:           "0123456789abcdef0123456789abcdef",
		CORSOrigins:         []string{"https://ships.example.com"},
		AdminPassword:       "change-this-admin-password",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected shared service tokens to be rejected")
	}
}

func TestSplitCSVTrimsEmptyValues(t *testing.T) {
	items := splitCSV(" http://localhost:3000, ,http://localhost:5173 ")
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %#v", items)
	}
	if items[0] != "http://localhost:3000" || items[1] != "http://localhost:5173" {
		t.Fatalf("unexpected items: %#v", items)
	}
}
