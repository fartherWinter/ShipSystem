package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shipsim/internal/api"
	"shipsim/internal/config"
	"shipsim/internal/model"
	"shipsim/internal/sim"
	"shipsim/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	var st store.Store
	if dsn := cfg.DatabaseURL; dsn != "" {
		pg, err := store.NewPostgres(context.Background(), dsn)
		if err != nil {
			logger.Error("postgres unavailable", "error", err)
			os.Exit(1)
		}
		status, err := pg.MigrationStatus(context.Background())
		if err != nil {
			logger.Error("postgres migration status unavailable", "error", err)
			pg.Close()
			os.Exit(1)
		}
		if err := status.Error(); err != nil {
			logger.Error("postgres migrations required", "current_version", status.Current, "required_version", status.Required, "error", err)
			pg.Close()
			os.Exit(1)
		}
		defer pg.Close()
		st = pg
		logger.Info("using postgres/postgis store", "migration_version", status.Current)
	} else {
		st = store.NewMemory()
		logger.Warn("DATABASE_URL not set; using memory store for local simulation demo")
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	manager := sim.NewManagerWithSnapshotWriteTimeout(st, logger, cfg.SnapshotWriteTimeout)
	if count, err := manager.LoadScenarioDir(cfg.ScenarioDir); err != nil {
		logger.Error("load scenarios failed", "dir", cfg.ScenarioDir, "error", err)
		os.Exit(1)
	} else {
		logger.Info("scenarios loaded", "dir", cfg.ScenarioDir, "count", count)
	}
	policy := modelRetentionPolicy(cfg, time.Now().UTC())
	if retentionPolicyEmpty(policy) {
		logger.Info("retention prune skipped; no retention policy configured")
	} else {
		if _, err := runRetentionPrune(context.Background(), manager, logger, policy); err != nil {
			logger.Error("retention prune failed", "error", err)
			os.Exit(1)
		}
	}
	if cfg.RetentionInterval > 0 && !retentionPolicyEmpty(policy) {
		startRetentionWorker(signalCtx, manager, logger, cfg)
	} else if cfg.RetentionInterval > 0 {
		logger.Info("retention worker skipped; interval configured without retention policy")
	} else {
		logger.Info("retention worker disabled", "interval", cfg.RetentionInterval.String())
	}
	server := api.NewServerWithConfig(manager, logger, cfg)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Routes(),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("ship simulation server starting", "addr", cfg.Addr, "auth_mode", cfg.AuthMode, "env", cfg.Environment)
		serverErr <- httpServer.ListenAndServe()
	}()

	shutdownCompleted := false
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		logger.Info("shutdown signal received", "timeout", cfg.ShutdownTimeout.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("http shutdown failed", "error", err)
			_ = httpServer.Close()
		}
		if err := manager.Shutdown(shutdownCtx); err != nil {
			logger.Error("simulation shutdown completed with errors", "error", err)
			os.Exit(1)
		}
		shutdownCompleted = true
		logger.Info("ship simulation server stopped")
	}
	if !shutdownCompleted {
		if err := manager.Shutdown(context.Background()); err != nil {
			logger.Error("simulation shutdown completed with errors", "error", err)
			os.Exit(1)
		}
	}
}

func startRetentionWorker(ctx context.Context, manager *sim.Manager, logger *slog.Logger, cfg config.Config) {
	go func() {
		ticker := time.NewTicker(cfg.RetentionInterval)
		defer ticker.Stop()
		logger.Info("retention worker started", "interval", cfg.RetentionInterval.String())
		for {
			select {
			case <-ctx.Done():
				logger.Info("retention worker stopped")
				return
			case now := <-ticker.C:
				policy := modelRetentionPolicy(cfg, now.UTC())
				if retentionPolicyEmpty(policy) {
					continue
				}
				if _, err := runRetentionPrune(ctx, manager, logger, policy); err != nil {
					logger.Error("retention worker prune failed", "error", err)
				}
			}
		}
	}()
}

func runRetentionPrune(ctx context.Context, manager *sim.Manager, logger *slog.Logger, policy model.RetentionPolicy) (model.RetentionResult, error) {
	pruned, err := manager.Prune(ctx, policy)
	if err != nil {
		return model.RetentionResult{}, err
	}
	logger.Info("retention prune completed",
		"runs_matched", pruned.RunsMatched,
		"events_deleted", pruned.EventsDeleted,
		"track_points_deleted", pruned.TrackPointsDeleted,
		"contacts_deleted", pruned.ContactsDeleted,
		"snapshots_deleted", pruned.SnapshotsDeleted,
	)
	return pruned, nil
}

func modelRetentionPolicy(cfg config.Config, now time.Time) model.RetentionPolicy {
	policy := model.RetentionPolicy{
		MaxTrackPointsPerRun: cfg.MaxTrackPointsPerRun,
		MaxEventsPerRun:      cfg.MaxEventsPerRun,
		MaxSnapshotsPerRun:   cfg.MaxSnapshotsPerRun,
	}
	if cfg.RetentionDays > 0 {
		policy.Cutoff = now.Add(-time.Duration(cfg.RetentionDays) * 24 * time.Hour)
	}
	return policy
}

func retentionPolicyEmpty(policy model.RetentionPolicy) bool {
	return policy.Cutoff.IsZero() &&
		policy.EndedBefore.IsZero() &&
		policy.MaxTrackPointsPerRun <= 0 &&
		policy.MaxEventsPerRun <= 0 &&
		policy.MaxSnapshotsPerRun <= 0
}
