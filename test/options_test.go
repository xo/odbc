package test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/xo/dbimp"
	"github.com/xo/odbc"
)

// TestOptions applies each option to a statement of each database. An option
// that the database driver cannot honor must fail with dbimp.ErrNotSupported,
// and never run the statement as if it held (D22). The test logs which
// databases honor which option.
func TestOptions(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		honored := func(name string, err error) {
			t.Helper()
			switch {
			case err == nil:
				t.Logf("%s honors %s", p.name, name)
			case errors.Is(err, dbimp.ErrNotSupported):
				t.Logf("%s does not honor %s: %v", p.name, name, err)
			default:
				t.Errorf("%s: %v", name, err)
			}
		}
		var n int
		honored("WithTimeout", db.QueryRowContext(ctx, "SELECT 1", odbc.WithTimeout(10*time.Second)).Scan(&n))
		honored("WithParameter", db.QueryRowContext(ctx, "SELECT 1", odbc.WithParameter("query_timeout", 10)).Scan(&n))

		// an option that is not valid fails before it reaches the database
		for name, opt := range map[string]odbc.Option{
			"a negative timeout":    odbc.WithTimeout(-time.Second),
			"an unknown parameter":  odbc.WithParameter("no_such_attribute", 1),
			"a parameter with text": odbc.WithParameter("max_rows", "ten"),
		} {
			if err := db.QueryRowContext(ctx, "SELECT 1", opt).Scan(&n); !errors.Is(err, dbimp.ErrInvalidValue) {
				t.Errorf("%s: got %v, want dbimp.ErrInvalidValue", name, err)
			}
		}

		// options from the context apply to each statement of it
		octx := odbc.WithOptions(ctx, odbc.WithTimeout(10*time.Second))
		honored("WithOptions", db.QueryRowContext(octx, "SELECT 1").Scan(&n))

		// an option argument does not count as a parameter of the statement
		if err := db.QueryRowContext(ctx, "SELECT ?", odbc.WithTimeout(10*time.Second), 7).Scan(&n); err != nil && !errors.Is(err, dbimp.ErrNotSupported) {
			t.Errorf("an argument after an option: %v", err)
		} else if err == nil && n != 7 {
			t.Errorf("an argument after an option: got %d, want 7", n)
		}
	})
}

// TestWithDatabase runs a statement in the catalog that the option names, on the
// databases that have catalogs, and checks that the connection returns to the
// catalog it had.
func TestWithDatabase(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		query := map[string]string{
			"postgres":  "SELECT current_database()",
			"mariadb":   "SELECT DATABASE()",
			"mysql":     "SELECT DATABASE()",
			"sqlserver": "SELECT DB_NAME()",
		}[p.name]
		other := map[string]string{
			"postgres": "template1", "mariadb": "information_schema", "mysql": "information_schema", "sqlserver": "tempdb",
		}[p.name]
		if query == "" {
			t.Skipf("%s has no catalog to name", p.name)
		}
		ctx := t.Context()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		var before, during, after string
		if err := conn.QueryRowContext(ctx, query).Scan(&before); err != nil {
			t.Fatal(err)
		}
		err = conn.QueryRowContext(ctx, query, odbc.WithDatabase(other)).Scan(&during)
		switch {
		case errors.Is(err, dbimp.ErrNotSupported):
			t.Skipf("%s does not honor WithDatabase: %v", p.name, err)
		case err != nil:
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(ctx, query).Scan(&after); err != nil {
			t.Fatal(err)
		}
		t.Logf("before %q, during %q, after %q", before, during, after)
		if !equalFold(during, other) || after != before {
			t.Errorf("before %q, during %q, after %q, want %q during and the first catalog after", before, during, after, other)
		}
	})
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// TestReadonlyIsRefused checks that WithReadonly(true) fails with
// dbimp.ErrNotSupported and never writes. PostgreSQL, SQL Server and DuckDB
// accept the ODBC access mode and still write, so the driver does not use it
// (D22).
func TestReadonlyIsRefused(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_ro")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_ro (a integer)"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_ro") }()
		if _, err := db.ExecContext(ctx, "INSERT INTO odbc_ro VALUES (1)", odbc.WithReadonly(true)); !errors.Is(err, dbimp.ErrNotSupported) {
			t.Errorf("got %v, want dbimp.ErrNotSupported", err)
		}
		var n int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM odbc_ro").Scan(&n); err != nil || n != 0 {
			t.Errorf("the table holds %d rows after a refused write, and the error is %v", n, err)
		}
		// false asks for nothing
		if _, err := db.ExecContext(ctx, "INSERT INTO odbc_ro VALUES (2)", odbc.WithReadonly(false)); err != nil {
			t.Errorf("WithReadonly(false): %v", err)
		}
	})
}

// TestTimeoutIsEnforced checks that a database that accepts WithTimeout also
// stops a statement that runs longer. A driver that accepts the option and
// waits breaks the rule of D22.
func TestTimeoutIsEnforced(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		sleep := map[string]string{
			"postgres":  "SELECT pg_sleep(3)",
			"mariadb":   "SELECT SLEEP(3)",
			"mysql":     "SELECT SLEEP(3)",
			"sqlserver": "WAITFOR DELAY '00:00:03'",
		}[p.name]
		if sleep == "" {
			t.Skipf("%s has no statement that sleeps", p.name)
		}
		start := time.Now()
		_, err := db.ExecContext(t.Context(), sleep, odbc.WithTimeout(time.Second))
		took := time.Since(start)
		switch {
		case errors.Is(err, dbimp.ErrNotSupported):
			t.Logf("%s refuses WithTimeout: %v", p.name, err)
		case err == nil && took > 2500*time.Millisecond:
			t.Errorf("%s accepted WithTimeout and waited %v", p.name, took)
		default:
			t.Logf("%s stopped the statement after %v: %v", p.name, took.Round(time.Millisecond), err)
		}
	})
}
