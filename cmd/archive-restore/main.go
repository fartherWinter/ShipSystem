package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"shipsim/internal/runarchive"
	"shipsim/internal/store"
)

func main() {
	var archivePath string
	var databaseURL string
	var allowAppend bool
	var jsonOutput bool
	flag.StringVar(&archivePath, "archive", "", "Path to a run archive directory or compressed .zip bundle")
	flag.StringVar(&databaseURL, "database-url", firstEnv("SHIP_SIM_DATABASE_URL", "DATABASE_URL"), "PostgreSQL/PostGIS database URL")
	flag.BoolVar(&allowAppend, "allow-append", false, "Allow restoring into a run that already has replay data")
	flag.BoolVar(&jsonOutput, "json", false, "Print restore summary as JSON")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if strings.TrimSpace(archivePath) == "" {
		logger.Error("archive path is required", "flag", "-archive")
		os.Exit(2)
	}
	if strings.TrimSpace(databaseURL) == "" {
		logger.Error("database URL is required", "env", "SHIP_SIM_DATABASE_URL or DATABASE_URL")
		os.Exit(2)
	}

	ctx := context.Background()
	pg, err := store.NewPostgres(ctx, databaseURL)
	if err != nil {
		logger.Error("postgres unavailable", "error", err)
		os.Exit(1)
	}
	defer pg.Close()
	status, err := pg.MigrationStatus(ctx)
	if err != nil {
		logger.Error("postgres migration status unavailable", "error", err)
		os.Exit(1)
	}
	if err := status.Error(); err != nil {
		logger.Error("postgres migrations required", "current_version", status.Current, "required_version", status.Required, "error", err)
		os.Exit(1)
	}

	summary, err := runarchive.Restore(ctx, pg, archivePath, runarchive.RestoreOptions{AllowAppend: allowAppend})
	if err != nil {
		logger.Error("archive restore failed", "archive", archivePath, "error", err)
		os.Exit(1)
	}
	if jsonOutput {
		body, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			logger.Error("encode summary failed", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(body))
		return
	}
	fmt.Printf("Restored training archive run %s: %d events, %d snapshots, %d annotations, %d audit logs\n",
		summary.RunID, summary.Events, summary.Snapshots, summary.Annotations, summary.AuditLogs)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
