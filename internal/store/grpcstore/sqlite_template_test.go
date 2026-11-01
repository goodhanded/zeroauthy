package grpcstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/zeroauthy/zeroauthy/internal/store/sqlstore"
)

// sqliteTemplate is one migrated SQLite database per test process, copied for
// every test that wants a fresh migrated store. Under -race, running the
// migrations on modernc's pure-Go SQLite costs about 1.7 s per database;
// copying the file costs milliseconds. A copy of the helper in
// internal/store/sqlstore, whose in-package tests cannot import a shared one
// without an import cycle.
var sqliteTemplate struct {
	once sync.Once
	dir  string
	path string
	err  error
}

// migratedSQLiteDSN returns the DSN of a fresh, migrated SQLite database for
// one test: a copy of the template in the test's own temporary directory.
func migratedSQLiteDSN(t *testing.T) string {
	t.Helper()
	sqliteTemplate.once.Do(func() {
		sqliteTemplate.dir, sqliteTemplate.path, sqliteTemplate.err = buildSQLiteTemplate()
	})
	if sqliteTemplate.err != nil {
		t.Fatal(sqliteTemplate.err)
	}
	body, err := os.ReadFile(sqliteTemplate.path)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "z.db")
	if err := os.WriteFile(dst, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return "sqlite://" + dst
}

func buildSQLiteTemplate() (dir, path string, err error) {
	ctx := context.Background()
	if dir, err = os.MkdirTemp("", "zeroauthy-sqlite-template-"); err != nil {
		return "", "", err
	}
	path = filepath.Join(dir, "template.db")
	s, err := sqlstore.Open(ctx, "sqlite://"+path)
	if err != nil {
		return dir, "", err
	}
	if err = s.Migrate(ctx); err != nil {
		_ = s.Close()
		return dir, "", err
	}
	if err = s.Close(); err != nil {
		return dir, "", err
	}
	// The store runs in WAL mode. Closing its last connection checkpoints the
	// log into the main file, so the main file alone is then the whole
	// database. A log with content left behind would make a copy of the main
	// file a partial one, so refuse it rather than copy it.
	if fi, statErr := os.Stat(path + "-wal"); statErr == nil && fi.Size() > 0 {
		return dir, "", fmt.Errorf("sqlite template: %s-wal still holds %d bytes after close", path, fi.Size())
	}
	return dir, path, nil
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sqliteTemplate.dir != "" {
		_ = os.RemoveAll(sqliteTemplate.dir)
	}
	os.Exit(code)
}
