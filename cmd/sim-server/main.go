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
		defer pg.Close()
		st = pg
		logger.Info("using postgres/postgis store")
	} else {
		st = store.NewMemory()
		logger.Warn("DATABASE_URL not set; using memory store for local simulation demo")
	}

	manager := sim.NewManager(st, logger)
	if count, err := manager.LoadScenarioDir(cfg.ScenarioDir); err != nil {
		logger.Error("load scenarios failed", "dir", cfg.ScenarioDir, "error", err)
		os.Exit(1)
	} else {
		logger.Info("scenarios loaded", "dir", cfg.ScenarioDir, "count", count)
	}
	policy := modelRetentionPolicy(cfg.RetentionDays, cfg.MaxTrackPointsPerRun)
	if retentionPolicyEmpty(policy) {
		logger.Info("retention prune skipped; no retention policy configured")
	} else {
		pruned, err := manager.Prune(context.Background(), policy)
		if err != nil {
			logger.Error("retention prune failed", "error", err)
			os.Exit(1)
		}
		logger.Info("retention prune completed",
			"runs_matched", pruned.RunsMatched,
			"events_deleted", pruned.EventsDeleted,
			"track_points_deleted", pruned.TrackPointsDeleted,
			"contacts_deleted", pruned.ContactsDeleted,
			"snapshots_deleted", pruned.SnapshotsDeleted,
		)
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

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

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

func modelRetentionPolicy(days, maxTrackPoints int) model.RetentionPolicy {
	policy := model.RetentionPolicy{MaxTrackPointsPerRun: maxTrackPoints}
	if days > 0 {
		policy.Cutoff = time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	}
	return policy
}

func retentionPolicyEmpty(policy model.RetentionPolicy) bool {
	return policy.Cutoff.IsZero() && policy.EndedBefore.IsZero() && policy.MaxTrackPointsPerRun <= 0
}
