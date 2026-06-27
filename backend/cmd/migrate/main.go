package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gorm.io/gorm"
	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/database"
)

func main() {
	action := flag.String("action", "status", "migration action: status, check, or up")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	db, err := database.Connect(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	switch *action {
	case "status":
		if err := printStatus(db); err != nil {
			log.Fatalf("migration status: %v", err)
		}
	case "check":
		if err := database.EnsureMigrationsCurrent(db); err != nil {
			log.Fatalf("migration check: %v", err)
		}
		fmt.Println("migrations are current")
	case "up":
		if err := database.RunMigrations(db); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
		if err := printStatus(db); err != nil {
			log.Fatalf("migration status: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unsupported action %q; use status, check, or up\n", *action)
		os.Exit(2)
	}
}

func printStatus(db *gorm.DB) error {
	statuses, err := database.MigrationStatuses(db)
	if err != nil {
		return err
	}
	for _, status := range statuses {
		state := "pending"
		if status.Applied {
			state = "applied"
		}
		fmt.Printf("%s\t%s\n", state, status.Migration.Name)
	}
	return nil
}
