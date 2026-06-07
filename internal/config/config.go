package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	AuthOff     = "off"
	AuthToken   = "token"
	AuthProxy   = "proxy"
	EnvDev      = "development"
	EnvProd     = "production"
	DefaultAddr = ":8080"
)

type Config struct {
	Addr                  string
	DatabaseURL           string
	AllowedOrigins        []string
	Environment           string
	AuthMode              string
	AuthToken             string
	AuthUserHeader        string
	ScenarioDir           string
	StaticDir             string
	RequestBodyLimit      int64
	RetentionDays         int
	RetentionInterval     time.Duration
	MaxTrackPointsPerRun  int
	MaxEventsPerRun       int
	MaxSnapshotsPerRun    int
	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	ShutdownTimeout       time.Duration
	SnapshotWriteTimeout  time.Duration
}

func Default() Config {
	return Config{
		Addr:                  DefaultAddr,
		Environment:           EnvDev,
		AuthMode:              AuthOff,
		AuthUserHeader:        "X-Forwarded-User",
		ScenarioDir:           "scenarios",
		RequestBodyLimit:      1 << 20,
		RetentionDays:         0,
		RetentionInterval:     0,
		MaxTrackPointsPerRun:  0,
		MaxEventsPerRun:       0,
		MaxSnapshotsPerRun:    0,
		HTTPReadTimeout:       10 * time.Second,
		HTTPReadHeaderTimeout: 5 * time.Second,
		HTTPWriteTimeout:      30 * time.Second,
		HTTPIdleTimeout:       60 * time.Second,
		ShutdownTimeout:       15 * time.Second,
		SnapshotWriteTimeout:  5 * time.Second,
	}
}

func Load() (Config, error) {
	cfg := Default()
	cfg.Addr = env("SHIP_SIM_ADDR", cfg.Addr)
	cfg.DatabaseURL = firstEnv("SHIP_SIM_DATABASE_URL", "DATABASE_URL")
	cfg.AllowedOrigins = csvEnv("SHIP_SIM_ALLOWED_ORIGINS")
	cfg.Environment = env("SHIP_SIM_ENV", cfg.Environment)
	cfg.AuthMode = env("SHIP_SIM_AUTH_MODE", cfg.AuthMode)
	cfg.AuthToken = os.Getenv("SHIP_SIM_AUTH_TOKEN")
	cfg.AuthUserHeader = env("SHIP_SIM_AUTH_USER_HEADER", cfg.AuthUserHeader)
	cfg.ScenarioDir = env("SHIP_SIM_SCENARIO_DIR", cfg.ScenarioDir)
	cfg.StaticDir = os.Getenv("SHIP_SIM_STATIC_DIR")
	var parseErrs []string
	var err error
	if cfg.RequestBodyLimit, err = int64Env("SHIP_SIM_REQUEST_BODY_LIMIT", cfg.RequestBodyLimit); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RetentionDays, err = intEnv("SHIP_SIM_RETENTION_DAYS", cfg.RetentionDays); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.RetentionInterval, err = durationEnv("SHIP_SIM_RETENTION_INTERVAL", cfg.RetentionInterval); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.MaxTrackPointsPerRun, err = intEnv("SHIP_SIM_MAX_TRACK_POINTS_PER_RUN", cfg.MaxTrackPointsPerRun); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.MaxEventsPerRun, err = intEnv("SHIP_SIM_MAX_EVENTS_PER_RUN", cfg.MaxEventsPerRun); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.MaxSnapshotsPerRun, err = intEnv("SHIP_SIM_MAX_SNAPSHOTS_PER_RUN", cfg.MaxSnapshotsPerRun); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.HTTPReadTimeout, err = durationEnv("SHIP_SIM_HTTP_READ_TIMEOUT", cfg.HTTPReadTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.HTTPReadHeaderTimeout, err = durationEnv("SHIP_SIM_HTTP_READ_HEADER_TIMEOUT", cfg.HTTPReadHeaderTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.HTTPWriteTimeout, err = durationEnv("SHIP_SIM_HTTP_WRITE_TIMEOUT", cfg.HTTPWriteTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.HTTPIdleTimeout, err = durationEnv("SHIP_SIM_HTTP_IDLE_TIMEOUT", cfg.HTTPIdleTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.ShutdownTimeout, err = durationEnv("SHIP_SIM_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if cfg.SnapshotWriteTimeout, err = durationEnv("SHIP_SIM_SNAPSHOT_WRITE_TIMEOUT", cfg.SnapshotWriteTimeout); err != nil {
		parseErrs = append(parseErrs, err.Error())
	}
	if len(parseErrs) > 0 {
		return cfg, errors.New(strings.Join(parseErrs, "; "))
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	var details []string
	if strings.TrimSpace(c.Addr) == "" {
		details = append(details, "SHIP_SIM_ADDR is required")
	}
	if c.RequestBodyLimit < 1024 {
		details = append(details, "SHIP_SIM_REQUEST_BODY_LIMIT must be at least 1024 bytes")
	}
	if c.RetentionDays < 0 {
		details = append(details, "SHIP_SIM_RETENTION_DAYS must be zero or greater")
	}
	if c.RetentionInterval < 0 {
		details = append(details, "SHIP_SIM_RETENTION_INTERVAL must be zero or greater")
	}
	if c.MaxTrackPointsPerRun > 0 && c.MaxTrackPointsPerRun < 1000 {
		details = append(details, "SHIP_SIM_MAX_TRACK_POINTS_PER_RUN must be zero or at least 1000")
	}
	if c.MaxEventsPerRun < 0 {
		details = append(details, "SHIP_SIM_MAX_EVENTS_PER_RUN must be zero or greater")
	}
	if c.MaxSnapshotsPerRun > 0 && c.MaxSnapshotsPerRun < 1000 {
		details = append(details, "SHIP_SIM_MAX_SNAPSHOTS_PER_RUN must be zero or at least 1000")
	}
	if c.HTTPReadTimeout <= 0 {
		details = append(details, "SHIP_SIM_HTTP_READ_TIMEOUT must be greater than zero")
	}
	if c.HTTPReadHeaderTimeout <= 0 {
		details = append(details, "SHIP_SIM_HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}
	if c.HTTPWriteTimeout <= 0 {
		details = append(details, "SHIP_SIM_HTTP_WRITE_TIMEOUT must be greater than zero")
	}
	if c.HTTPIdleTimeout <= 0 {
		details = append(details, "SHIP_SIM_HTTP_IDLE_TIMEOUT must be greater than zero")
	}
	if c.ShutdownTimeout <= 0 {
		details = append(details, "SHIP_SIM_SHUTDOWN_TIMEOUT must be greater than zero")
	}
	if c.SnapshotWriteTimeout <= 0 {
		details = append(details, "SHIP_SIM_SNAPSHOT_WRITE_TIMEOUT must be greater than zero")
	}
	switch c.AuthMode {
	case AuthOff:
		if strings.EqualFold(c.Environment, EnvProd) {
			details = append(details, "SHIP_SIM_AUTH_MODE must not be off when SHIP_SIM_ENV=production")
		}
	case AuthToken:
		if strings.TrimSpace(c.AuthToken) == "" {
			details = append(details, "SHIP_SIM_AUTH_TOKEN is required when SHIP_SIM_AUTH_MODE=token")
		}
		if strings.EqualFold(c.Environment, EnvProd) && c.AuthToken == "change-this-before-deploying" {
			details = append(details, "SHIP_SIM_AUTH_TOKEN must be changed before production deployment")
		}
	case AuthProxy:
		if strings.TrimSpace(c.AuthUserHeader) == "" {
			details = append(details, "SHIP_SIM_AUTH_USER_HEADER is required when SHIP_SIM_AUTH_MODE=proxy")
		}
	default:
		details = append(details, "SHIP_SIM_AUTH_MODE must be one of off, token, proxy")
	}
	if strings.EqualFold(c.Environment, EnvProd) {
		if len(c.AllowedOrigins) == 0 {
			details = append(details, "SHIP_SIM_ALLOWED_ORIGINS is required when SHIP_SIM_ENV=production")
		}
		for _, origin := range c.AllowedOrigins {
			if origin == "*" {
				details = append(details, "SHIP_SIM_ALLOWED_ORIGINS must not include * in production")
			}
		}
	}
	if len(details) > 0 {
		return errors.New(strings.Join(details, "; "))
	}
	return nil
}

func (c Config) AuthEnabled() bool {
	return c.AuthMode != AuthOff
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func int64Env(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback, errors.New(key + " must be an integer")
	}
	return value, nil
}

func intEnv(key string, fallback int) (int, error) {
	value, err := int64Env(key, int64(fallback))
	return int(value), err
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback, errors.New(key + " must be a duration such as 5s, 1m, or 500ms")
	}
	return value, nil
}
