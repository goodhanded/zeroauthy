// internal/testutil/sqlitetemplate/sqlitetemplate.go

// Package sqlitetemplate gives tests a fresh, migrated SQLite database by
// copying one migrated per test process, instead of migrating each one.
// Under -race, running the migrations on modernc's pure-Go SQLite costs
// about 1.7 s per database; copying the file costs milliseconds.
//
// It does not import the store, so sqlstore's in-package tests can use it
// too: the caller supplies the migration.
package sqlitetemplate

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Template is one migrated database, built on first use and copied for each
// test that asks.
type Template struct {
	migrate func(path string) error

	once sync.Once
	dir  string
	path string
	err  error
}

// New returns a template that migrate builds: migrate must open the SQLite
// database at path, run every migration and close it again, leaving no open
// connection.
func New(migrate func(path string) error) *Template {
	return &Template{migrate: migrate}
}

// CopyTo writes a copy of the migrated template to dst, building the
// template first if no test has yet. A failed build fails every caller.
func (tp *Template) CopyTo(t testing.TB, dst string) {
	t.Helper()
	tp.once.Do(tp.build)
	if tp.err != nil {
		t.Fatal(tp.err)
	}
	body, err := os.ReadFile(tp.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Remove deletes the template's directory. Call it from TestMain after
// m.Run.
func (tp *Template) Remove() {
	if tp.dir != "" {
		_ = os.RemoveAll(tp.dir)
	}
}

func (tp *Template) build() {
	dir, err := os.MkdirTemp("", "zeroauthy-sqlite-template-")
	if err != nil {
		tp.err = err
		return
	}
	tp.dir = dir
	path := filepath.Join(dir, "template.db")
	if err := tp.migrate(path); err != nil {
		tp.err = fmt.Errorf("sqlite template: %w", err)
		return
	}
	// The store runs in WAL mode. Closing its last connection checkpoints the
	// log into the main file, so the main file alone is then the whole
	// database. A log with content left behind would make a copy of the main
	// file a partial one, so refuse it rather than copy it.
	if fi, err := os.Stat(path + "-wal"); err == nil && fi.Size() > 0 {
		tp.err = fmt.Errorf("sqlite template: %s-wal still holds %d bytes after close", path, fi.Size())
		return
	}
	tp.path = path
}
