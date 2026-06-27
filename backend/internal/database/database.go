package database

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/models"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migration struct {
	Version string
	Name    string
	Path    string
}

type MigrationStatus struct {
	Migration Migration
	Applied   bool
}

type MigrationGateError struct {
	Pending []Migration
}

func (e *MigrationGateError) Error() string {
	names := make([]string, 0, len(e.Pending))
	for _, migration := range e.Pending {
		names = append(names, migration.Name)
	}
	return fmt.Sprintf(
		"database migrations are not current: pending %s; run `go run ./cmd/migrate -action=up` before starting with DATABASE_AUTO_MIGRATE=false",
		strings.Join(names, ", "),
	)
}

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func MigrateAndSeed(db *gorm.DB, cfg config.Config) error {
	if cfg.DatabaseAutoMigrate {
		if err := RunMigrations(db); err != nil {
			return err
		}
		if err := autoMigrate(db); err != nil {
			return err
		}
	} else {
		if err := EnsureMigrationsCurrent(db); err != nil {
			return err
		}
	}
	return seed(db, cfg)
}

func ListMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	migrations := make([]Migration, 0, len(names))
	for _, name := range names {
		version, _, _ := strings.Cut(name, "_")
		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			Path:    "migrations/" + name,
		})
	}
	return migrations, nil
}

func MigrationStatuses(db *gorm.DB) ([]MigrationStatus, error) {
	migrations, err := ListMigrations()
	if err != nil {
		return nil, err
	}
	if err := ensureSchemaMigrations(db); err != nil {
		return nil, err
	}
	applied, err := appliedMigrationSet(db)
	if err != nil {
		return nil, err
	}
	statuses := make([]MigrationStatus, 0, len(migrations))
	for _, migration := range migrations {
		_, ok := applied[migration.Name]
		statuses = append(statuses, MigrationStatus{Migration: migration, Applied: ok})
	}
	return statuses, nil
}

func RunMigrations(db *gorm.DB) error {
	migrations, err := ListMigrations()
	if err != nil {
		return err
	}
	if err := ensureSchemaMigrations(db); err != nil {
		return err
	}
	for _, migration := range migrations {
		if err := applyMigration(db, migration); err != nil {
			return err
		}
	}
	return nil
}

func EnsureMigrationsCurrent(db *gorm.DB) error {
	statuses, err := MigrationStatuses(db)
	if err != nil {
		return err
	}
	pending := PendingMigrations(statuses)
	if len(pending) > 0 {
		return &MigrationGateError{Pending: pending}
	}
	return nil
}

func PendingMigrations(statuses []MigrationStatus) []Migration {
	pending := make([]Migration, 0, len(statuses))
	for _, status := range statuses {
		if status.Applied {
			continue
		}
		pending = append(pending, status.Migration)
	}
	return pending
}

func ensureSchemaMigrations(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`).Error; err != nil {
		return err
	}
	return nil
}

func appliedMigrationSet(db *gorm.DB) (map[string]struct{}, error) {
	type row struct {
		Version string
	}
	var rows []row
	if err := db.Raw(`SELECT version FROM schema_migrations`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	applied := make(map[string]struct{}, len(rows))
	for _, item := range rows {
		applied[item.Version] = struct{}{}
	}
	return applied, nil
}

func applyMigration(db *gorm.DB, migration Migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Raw(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, migration.Name).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		sql, err := migrationFiles.ReadFile(migration.Path)
		if err != nil {
			return err
		}
		if err := tx.Exec(string(sql)).Error; err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
		if err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, migration.Name).Error; err != nil {
			return err
		}
		return nil
	})
}

func autoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.Role{},
		&models.Menu{},
		&models.User{},
		&models.Ship{},
		&models.ShipLocation{},
		&models.Alarm{},
		&models.DispatchEvent{},
		&models.DispatchEventLog{},
		&models.BattleSession{},
		&models.RadarTarget{},
		&models.BattleUnit{},
		&models.BattleProjectile{},
		&models.BattleEvent{},
		&models.BattleSnapshot{},
	); err != nil {
		return err
	}

	if err := db.Exec(`ALTER TABLE ship_locations ADD COLUMN IF NOT EXISTS geom geography(Point, 4326)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_ship_locations_geom ON ship_locations USING GIST (geom)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_ship_locations_ship_reported ON ship_locations (ship_id, reported_at DESC)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_radar_targets_session_scan ON radar_targets (session_id, scan_time DESC)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_battle_events_session_time ON battle_events (session_id, occurred_at DESC)`).Error; err != nil {
		return err
	}

	return nil
}

func defaultMenus() []models.Menu {
	return []models.Menu{
		{Name: "首页态势", Path: "/dashboard", Icon: "dashboard", Sort: 10},
		{Name: "船舶管理", Path: "/ships", Icon: "ship", Sort: 20},
		{Name: "实时监控", Path: "/monitor", Icon: "radar", Sort: 30},
		{Name: "雷达对战", Path: "/battle", Icon: "radar", Sort: 35},
		{Name: "轨迹回放", Path: "/tracks", Icon: "route", Sort: 40},
		{Name: "告警中心", Path: "/alarms", Icon: "bell", Sort: 50},
		{Name: "调度事件", Path: "/dispatch", Icon: "send", Sort: 60},
		{Name: "权限管理", Path: "/rbac", Icon: "users", Sort: 70},
	}
}

func seed(db *gorm.DB, cfg config.Config) error {
	roles := []models.Role{
		{Name: "超级管理员", Code: "super_admin", Description: "拥有全部权限"},
		{Name: "管理员", Code: "admin", Description: "系统管理与船舶管理"},
		{Name: "调度员", Code: "dispatcher", Description: "监控、告警和调度处理"},
		{Name: "观察员", Code: "viewer", Description: "只读查看"},
	}
	for _, role := range roles {
		if err := db.FirstOrCreate(&role, models.Role{Code: role.Code}).Error; err != nil {
			return err
		}
	}

	menus := defaultMenus()
	for _, menu := range menus {
		if err := db.FirstOrCreate(&menu, models.Menu{Path: menu.Path}).Error; err != nil {
			return err
		}
	}

	var role models.Role
	if err := db.Where("code = ?", "super_admin").First(&role).Error; err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		DisplayName:  "系统管理员",
		RoleID:       role.ID,
		Status:       "enabled",
	}
	if err := db.Where(models.User{Username: admin.Username}).FirstOrCreate(&admin).Error; err != nil {
		return err
	}

	if !cfg.SeedDemoData {
		return nil
	}

	ships := []models.Ship{
		{Name: "东海巡航 01", MMSI: "412000001", ShipType: "巡逻船", Flag: "CN", LengthM: 72, WidthM: 11, Status: "active"},
		{Name: "沪航货运 88", MMSI: "412000088", ShipType: "货船", Flag: "CN", LengthM: 128, WidthM: 19, Status: "active"},
	}
	for _, ship := range ships {
		if err := db.Where(models.Ship{MMSI: ship.MMSI}).FirstOrCreate(&ship).Error; err != nil {
			return err
		}
		loc := models.ShipLocation{
			ShipID:     ship.ID,
			Longitude:  121.49,
			Latitude:   31.23,
			SpeedKnots: 8,
			Course:     95,
			ReportedAt: time.Now(),
		}
		var count int64
		if err := db.Model(&models.ShipLocation{}).Where("ship_id = ?", ship.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := db.Create(&loc).Error; err != nil {
				return err
			}
			if err := db.Exec(`UPDATE ship_locations SET geom = ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography WHERE id = ?`, loc.Longitude, loc.Latitude, loc.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
