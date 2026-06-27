package database

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

func TestEmbeddedMigrationsAreVersionedSQL(t *testing.T) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
		if !strings.HasSuffix(entry.Name(), ".sql") {
			t.Fatalf("migration %s must be a .sql file", entry.Name())
		}
		if len(entry.Name()) < len("001_") || entry.Name()[3] != '_' {
			t.Fatalf("migration %s must start with a numeric version prefix", entry.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("expected at least one embedded migration")
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("migration names should be lexically sortable: %#v", names)
	}
}

func TestListMigrationsReturnsVersionedPaths(t *testing.T) {
	migrations, err := ListMigrations()
	if err != nil {
		t.Fatalf("ListMigrations returned error: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected migrations")
	}
	if !sort.SliceIsSorted(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	}) {
		t.Fatalf("expected migrations sorted by name: %#v", migrations)
	}
	first := migrations[0]
	if first.Version == "" || first.Name == "" || first.Path == "" {
		t.Fatalf("expected versioned migration metadata: %#v", first)
	}
	if !strings.HasPrefix(first.Path, "migrations/") {
		t.Fatalf("expected embedded migration path, got %s", first.Path)
	}
}

func TestBattleMenuMigrationIsIdempotent(t *testing.T) {
	data, err := migrationFiles.ReadFile("migrations/002_seed_battle_menu.sql")
	if err != nil {
		t.Fatalf("read battle menu migration: %v", err)
	}
	sql := string(data)
	for _, required := range []string{
		"INSERT INTO menus",
		"'/battle'",
		"ON CONFLICT (path) DO UPDATE",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("expected battle menu migration to contain %q", required)
		}
	}
}

func TestSeedMenuPathsCoverFrontendRoutes(t *testing.T) {
	menus := defaultMenus()
	paths := make(map[string]struct{}, len(menus))
	for _, menu := range menus {
		if menu.Path == "" {
			t.Fatalf("menu path must not be empty: %#v", menu)
		}
		paths[menu.Path] = struct{}{}
	}

	for _, path := range []string{"/dashboard", "/ships", "/monitor", "/battle", "/tracks", "/alarms", "/dispatch", "/rbac"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("expected seed menu path %s", path)
		}
	}
}

func TestSeedMenuSortOrderIsUnique(t *testing.T) {
	seen := map[int]string{}
	for _, menu := range defaultMenus() {
		if existing, ok := seen[menu.Sort]; ok {
			t.Fatalf("menu sort %d reused by %s and %s", menu.Sort, existing, menu.Path)
		}
		seen[menu.Sort] = menu.Path
	}
}

func TestPendingMigrationsReturnsOnlyUnappliedItems(t *testing.T) {
	statuses := []MigrationStatus{
		{Migration: Migration{Name: "001_init.sql"}, Applied: true},
		{Migration: Migration{Name: "002_seed.sql"}, Applied: false},
		{Migration: Migration{Name: "003_add_index.sql"}, Applied: false},
	}

	pending := PendingMigrations(statuses)

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending migrations, got %d", len(pending))
	}
	if pending[0].Name != "002_seed.sql" || pending[1].Name != "003_add_index.sql" {
		t.Fatalf("unexpected pending migrations: %#v", pending)
	}
}

func TestMigrationGateErrorIncludesPendingNamesAndCommand(t *testing.T) {
	err := (&MigrationGateError{
		Pending: []Migration{
			{Name: "002_seed.sql"},
			{Name: "003_add_index.sql"},
		},
	}).Error()

	for _, want := range []string{
		"002_seed.sql",
		"003_add_index.sql",
		"go run ./cmd/migrate -action=up",
		"DATABASE_AUTO_MIGRATE=false",
	} {
		if !strings.Contains(err, want) {
			t.Fatalf("expected error to contain %q, got %q", want, err)
		}
	}
}
