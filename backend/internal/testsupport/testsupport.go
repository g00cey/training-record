// Package testsupport provides shared test fixtures: a migrated in-tmp
// SQLite database and a fully-wired service / HTTP router.
//
// Every consumer package lives at backend/internal/<pkg>, so os.DirFS("../..")
// resolves to backend/ (which contains migrations/) regardless of which
// package's tests are running.
package testsupport

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"training-record/internal/database"
	"training-record/internal/httpapi"
	"training-record/internal/service"
	"training-record/internal/store"
)

// NewDB returns a fresh migrated database in a temp dir.
func NewDB(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// NewStore returns a Store over a fresh migrated database.
func NewStore(t testing.TB) *store.Store {
	return store.New(NewDB(t))
}

// NewService returns a Service over a fresh migrated database.
func NewService(t testing.TB) (*service.Service, *store.Store) {
	st := NewStore(t)
	return service.New(st), st
}

// NewRouter returns the HTTP handler wired to a fresh migrated database,
// using apiKey for auth.
func NewRouter(t testing.TB, apiKey string) (http.Handler, *store.Store) {
	svc, st := NewService(t)
	return httpapi.NewRouter(svc, apiKey), st
}

// LegacyDBPath is the on-disk legacy training.db, relative to a test
// package directory (backend/internal/<pkg>).
const LegacyDBPath = "../../../skill/.hermes/home/.hermes/training-logs/training.db"

// NewBootstrappedDB returns a migrated DB seeded from the legacy training.db
// (bootstrap + preset fixup applied). The test is skipped when the legacy
// file is not present.
func NewBootstrappedDB(t testing.TB) *sql.DB {
	t.Helper()
	if _, err := os.Stat(LegacyDBPath); err != nil {
		t.Skipf("legacy DB not present: %v", err)
	}
	db := NewDB(t)
	if _, err := database.MaybeBootstrap(db, LegacyDBPath, true); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := database.EnsureRoutinePresets(db); err != nil {
		t.Fatalf("ensure presets: %v", err)
	}
	return db
}

// NewBootstrappedRouter is NewRouter over a bootstrapped database.
func NewBootstrappedRouter(t testing.TB, apiKey string) (http.Handler, *store.Store) {
	st := store.New(NewBootstrappedDB(t))
	return httpapi.NewRouter(service.New(st), apiKey), st
}
