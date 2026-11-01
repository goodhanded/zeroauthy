package grpcstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeroauthy/zeroauthy/internal/store/sqlstore"
	"github.com/zeroauthy/zeroauthy/internal/testutil/sqlitetemplate"
)

// sqliteTemplate is the migrated SQLite database this package's tests copy
// instead of migrating their own (see internal/testutil/sqlitetemplate).
var sqliteTemplate = sqlitetemplate.New(func(path string) error {
	ctx := context.Background()
	s, err := sqlstore.Open(ctx, "sqlite://"+path)
	if err != nil {
		return err
	}
	if err := s.Migrate(ctx); err != nil {
		_ = s.Close()
		return err
	}
	return s.Close()
})

// migratedSQLiteDSN returns the DSN of a fresh, migrated SQLite database for
// one test: a copy of the template in the test's own temporary directory.
func migratedSQLiteDSN(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "z.db")
	sqliteTemplate.CopyTo(t, path)
	return "sqlite://" + path
}

func TestMain(m *testing.M) {
	code := m.Run()
	sqliteTemplate.Remove()
	os.Exit(code)
}
