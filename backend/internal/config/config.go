package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"

	defaultJWTSecret     = "dev-only-change-me"
	defaultAdminPassword = "Admin123!"
)

type Config struct {
	Environment         string
	Port                string
	DatabaseDSN         string
	RequestBodyLimit    int64
	HTTPReadTimeout     time.Duration
	HTTPHeaderTimeout   time.Duration
	HTTPWriteTimeout    time.Duration
	HTTPIdleTimeout     time.Duration
	ShutdownTimeout     time.Duration
	AnalyticsHTTPTimeout time.Duration
	DatabaseAutoMigrate bool
	SeedDemoData        bool
	AnalyticsBaseURL    string
	AnalyticsAdminToken string
	GoAPIToken          string
	JWTSecret           string
	JWTTTL              time.Duration
	CORSOrigins         []string
	AdminPassword       string
}

func Load() (Config, error) {
	environment := strings.ToLower(getenv("APP_ENV", EnvDevelopment))
	ttlHours, err := strconv.Atoi(getenv("JWT_TTL_HOURS", "24"))
	if err != nil || ttlHours <= 0 {
		return Config{}, fmt.Errorf("JWT_TTL_HOURS must be a positive integer")
	}
	autoMigrateDefault := "true"
	seedDemoDefault := "true"
	if environment == EnvProduction || environment == "prod" {
		autoMigrateDefault = "false"
		seedDemoDefault = "false"
	}
	autoMigrate, err := parseBoolEnv("DATABASE_AUTO_MIGRATE", autoMigrateDefault)
	if err != nil {
		return Config{}, err
	}
	requestBodyLimit, err := parseInt64Env("REQUEST_BODY_LIMIT_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	httpReadTimeout, err := parseDurationEnv("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpHeaderTimeout, err := parseDurationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpWriteTimeout, err := parseDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	httpIdleTimeout, err := parseDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := parseDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	analyticsHTTPTimeout, err := parseDurationEnv("ANALYTICS_HTTP_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	seedDemoData, err := parseBoolEnv("SEED_DEMO_DATA", seedDemoDefault)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Environment:         environment,
		Port:                getenv("PORT", "8080"),
		DatabaseDSN:         getenv("DATABASE_DSN", "host=localhost user=shipsystem password=shipsystem dbname=shipsystem port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		RequestBodyLimit:    requestBodyLimit,
		HTTPReadTimeout:     httpReadTimeout,
		HTTPHeaderTimeout:   httpHeaderTimeout,
		HTTPWriteTimeout:    httpWriteTimeout,
		HTTPIdleTimeout:     httpIdleTimeout,
		ShutdownTimeout:     shutdownTimeout,
		AnalyticsHTTPTimeout: analyticsHTTPTimeout,
		DatabaseAutoMigrate: autoMigrate,
		SeedDemoData:        seedDemoData,
		AnalyticsBaseURL:    strings.TrimRight(getenv("ANALYTICS_BASE_URL", "http://localhost:8090"), "/"),
		AnalyticsAdminToken: getenv("ANALYTICS_ADMIN_TOKEN", ""),
		GoAPIToken:          getenv("GO_API_TOKEN", ""),
		JWTSecret:           getenv("JWT_SECRET", defaultJWTSecret),
		JWTTTL:              time.Duration(ttlHours) * time.Hour,
		CORSOrigins:         splitCSV(getenv("CORS_ORIGINS", "http://localhost:5173")),
		AdminPassword:       getenv("ADMIN_PASSWORD", defaultAdminPassword),
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Port == "" {
		return errors.New("PORT is required")
	}
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if c.RequestBodyLimit < 1024 {
		return errors.New("REQUEST_BODY_LIMIT_BYTES must be at least 1024")
	}
	if c.HTTPReadTimeout <= 0 {
		return errors.New("HTTP_READ_TIMEOUT must be greater than zero")
	}
	if c.HTTPHeaderTimeout <= 0 {
		return errors.New("HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}
	if c.HTTPWriteTimeout <= 0 {
		return errors.New("HTTP_WRITE_TIMEOUT must be greater than zero")
	}
	if c.HTTPIdleTimeout <= 0 {
		return errors.New("HTTP_IDLE_TIMEOUT must be greater than zero")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("HTTP_SHUTDOWN_TIMEOUT must be greater than zero")
	}
	if c.AnalyticsHTTPTimeout <= 0 {
		return errors.New("ANALYTICS_HTTP_TIMEOUT must be greater than zero")
	}
	if len(c.CORSOrigins) == 0 {
		return errors.New("CORS_ORIGINS must include at least one origin")
	}
	if !c.IsProduction() {
		return nil
	}
	if c.DatabaseDSN == "" {
		return errors.New("DATABASE_DSN is required in production")
	}
	if c.DatabaseAutoMigrate {
		return errors.New("DATABASE_AUTO_MIGRATE must be disabled in production")
	}
	if c.SeedDemoData {
		return errors.New("SEED_DEMO_DATA must be disabled in production")
	}
	if c.AnalyticsBaseURL == "" {
		return errors.New("ANALYTICS_BASE_URL is required in production")
	}
	if c.AnalyticsAdminToken == "" || len(c.AnalyticsAdminToken) < 24 {
		return errors.New("ANALYTICS_ADMIN_TOKEN must be set to a strong value in production")
	}
	if c.GoAPIToken == "" || len(c.GoAPIToken) < 24 {
		return errors.New("GO_API_TOKEN must be set to a strong value in production")
	}
	if c.GoAPIToken == c.AnalyticsAdminToken {
		return errors.New("GO_API_TOKEN must be different from ANALYTICS_ADMIN_TOKEN in production")
	}
	if c.JWTSecret == defaultJWTSecret || c.JWTSecret == "please-change-me" || len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be changed to a strong value in production")
	}
	if c.AdminPassword == "" || c.AdminPassword == defaultAdminPassword || c.AdminPassword == "please-change-me" || len(c.AdminPassword) < 12 {
		return errors.New("ADMIN_PASSWORD must be changed to a strong value in production")
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			return errors.New("CORS_ORIGINS cannot include * in production")
		}
	}
	return nil
}

func (c Config) IsProduction() bool {
	return c.Environment == EnvProduction || c.Environment == "prod"
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func parseBoolEnv(key, fallback string) (bool, error) {
	value := getenv(key, fallback)
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return parsed, nil
}

func parseInt64Env(key string, fallback int64) (int64, error) {
	value := getenv(key, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := getenv(key, fallback.String())
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 5s, 1m, or 500ms", key)
	}
	return parsed, nil
}
