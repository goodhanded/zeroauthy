package sqlstore

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/zeroauthy/zeroauthy/internal/testutil/sqlitetemplate"
)

// sqliteTemplate is the migrated SQLite database this package's tests copy
// instead of migrating their own (see internal/testutil/sqlitetemplate).
var sqliteTemplate = sqlitetemplate.New(func(path string) error {
	ctx := context.Background()
	s, err := Open(ctx, "sqlite://"+path)
	if err != nil {
		return err
	}
	if err := s.Migrate(ctx); err != nil {
		_ = s.Close()
		return err
	}
	return s.Close()
})

// migrated fills the SQLite database a sqlite:// DSN names with a copy of
// the migrated template and returns the DSN unchanged, so any parameters
// dsnsUnderTest puts on it still apply.
func migrated(t *testing.T, dsn string) string {
	t.Helper()
	path, _, _ := strings.Cut(strings.TrimPrefix(dsn, "sqlite://"), "?")
	sqliteTemplate.CopyTo(t, path)
	return dsn
}

func TestMain(m *testing.M) {
	code := m.Run()
	sqliteTemplate.Remove()
	os.Exit(code)
}
