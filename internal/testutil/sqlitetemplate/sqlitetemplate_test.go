package sqlitetemplate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyToBuildsOnceAndCopies(t *testing.T) {
	builds := 0
	tp := New(func(path string) error {
		builds++
		return os.WriteFile(path, []byte("migrated"), 0o600)
	})
	defer tp.Remove()
	for range 3 {
		dst := filepath.Join(t.TempDir(), "z.db")
		tp.CopyTo(t, dst)
		got, err := os.ReadFile(dst)
		if err != nil || !bytes.Equal(got, []byte("migrated")) {
			t.Fatalf("copy = %q, %v", got, err)
		}
	}
	if builds != 1 {
		t.Fatalf("builds = %d, want 1", builds)
	}
	dir := tp.dir
	tp.Remove()
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("template dir after Remove: %v", err)
	}
}

func TestBuildRefusesAFailedMigration(t *testing.T) {
	tp := New(func(string) error { return errors.New("boom") })
	defer tp.Remove()
	tp.once.Do(tp.build)
	if tp.err == nil || tp.path != "" {
		t.Fatalf("err = %v, path = %q", tp.err, tp.path)
	}
}

func TestBuildRefusesALogLeftWithContent(t *testing.T) {
	tp := New(func(path string) error {
		if err := os.WriteFile(path, []byte("main"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(path+"-wal", []byte("uncheckpointed"), 0o600)
	})
	defer tp.Remove()
	tp.once.Do(tp.build)
	if tp.err == nil || tp.path != "" {
		t.Fatalf("err = %v, path = %q", tp.err, tp.path)
	}
}

func TestBuildAcceptsAnEmptyLog(t *testing.T) {
	tp := New(func(path string) error {
		if err := os.WriteFile(path, []byte("main"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(path+"-wal", nil, 0o600)
	})
	defer tp.Remove()
	tp.once.Do(tp.build)
	if tp.err != nil || tp.path == "" {
		t.Fatalf("err = %v, path = %q", tp.err, tp.path)
	}
}
