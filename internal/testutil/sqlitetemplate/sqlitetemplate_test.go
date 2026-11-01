package sqlitetemplate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fatalRecorder wraps the calling test so a failing CopyTo can be observed
// through the public API: Fatal and Fatalf record the message and end the
// calling goroutine, as the real ones do, without failing the real test.
// Every other method reaches the real test.
type fatalRecorder struct {
	testing.TB
	msg string
}

func (r *fatalRecorder) Fatal(args ...any) {
	r.msg = fmt.Sprint(args...)
	runtime.Goexit()
}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// copyFails runs CopyTo on its own goroutine and returns what it failed
// with ("" if it did not).
func copyFails(t *testing.T, tp *Template, dst string) string {
	r := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tp.CopyTo(r, dst)
	}()
	<-done
	return r.msg
}

// isolatedTemp points os.MkdirTemp at a directory of the test's own, so the
// template's directory can be observed and is cleaned up with the test.
func isolatedTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

func entries(t *testing.T, dir string) int {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

func TestCopyToBuildsOnceCopiesAndRemoves(t *testing.T) {
	tmp := isolatedTemp(t)
	builds := 0
	tp := New(func(path string) error {
		builds++
		return os.WriteFile(path, []byte("migrated"), 0o600)
	})
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
	if n := entries(t, tmp); n != 1 {
		t.Fatalf("template directories before Remove = %d, want 1", n)
	}
	tp.Remove()
	if n := entries(t, tmp); n != 0 {
		t.Fatalf("template directories after Remove = %d, want 0", n)
	}
}

func TestAFailedMigrationFailsEveryCaller(t *testing.T) {
	isolatedTemp(t)
	builds := 0
	tp := New(func(string) error {
		builds++
		return errors.New("boom")
	})
	defer tp.Remove()
	for i := range 2 {
		if msg := copyFails(t, tp, filepath.Join(t.TempDir(), "z.db")); !strings.Contains(msg, "boom") {
			t.Fatalf("caller %d failed with %q, want the migration error", i+1, msg)
		}
	}
	if builds != 1 {
		t.Fatalf("builds = %d, want 1", builds)
	}
}

func TestALogLeftWithContentIsRefused(t *testing.T) {
	isolatedTemp(t)
	tp := New(func(path string) error {
		if err := os.WriteFile(path, []byte("main"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(path+"-wal", []byte("uncheckpointed"), 0o600)
	})
	defer tp.Remove()
	dst := filepath.Join(t.TempDir(), "z.db")
	if msg := copyFails(t, tp, dst); !strings.Contains(msg, "-wal still holds") {
		t.Fatalf("CopyTo failed with %q, want the WAL refusal", msg)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused template was still copied: %v", err)
	}
}

func TestAnEmptyLogIsAccepted(t *testing.T) {
	isolatedTemp(t)
	tp := New(func(path string) error {
		if err := os.WriteFile(path, []byte("main"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(path+"-wal", nil, 0o600)
	})
	defer tp.Remove()
	if msg := copyFails(t, tp, filepath.Join(t.TempDir(), "z.db")); msg != "" {
		t.Fatalf("CopyTo failed with %q, want a copy", msg)
	}
}
