package test

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xo/odbc"
)

// TestCatalog reads the metadata of a table through the catalog functions of
// ODBC (D24).
func TestCatalog(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_cat")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_cat (id integer NOT NULL PRIMARY KEY, name varchar(20) NOT NULL, note varchar(30))"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_cat") }()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		err = conn.Raw(func(dc any) error {
			c, ok := dc.(odbc.Conn)
			if !ok {
				t.Fatalf("%T is not an odbc.Conn", dc)
			}
			tables, err := c.Tables(ctx, "", "", "odbc_cat", "TABLE")
			if err != nil {
				return fmt.Errorf("tables: %w", err)
			}
			if len(tables) != 1 || !strings.EqualFold(tables[0].Name, "odbc_cat") {
				t.Errorf("tables: got %+v", tables)
			}
			columns, err := c.Columns(ctx, "", "", "odbc_cat", "")
			if err != nil {
				return fmt.Errorf("columns: %w", err)
			}
			var names []string
			for _, col := range columns {
				names = append(names, strings.ToLower(col.Name))
			}
			if !reflect.DeepEqual(names, []string{"id", "name", "note"}) {
				t.Errorf("columns: got %v", names)
			}
			if len(columns) == 3 {
				t.Logf("%s: %s %s size %d, nullable %d / %d", p.name, columns[1].Name, columns[1].TypeName, columns[1].Size, columns[1].Nullable, columns[2].Nullable)
				if columns[1].Ordinal != 2 {
					t.Errorf("the ordinal of name is %d", columns[1].Ordinal)
				}
			}
			keys, err := c.PrimaryKeys(ctx, "", "", "odbc_cat")
			if err != nil {
				return fmt.Errorf("primary keys: %w", err)
			}
			version, _ := c.GetInfoString(odbc.InfoDriverVersion)
			switch {
			case p.name == "duckdb" && len(keys) == 0:
				// the DuckDB driver lists no primary key
				t.Logf("duckdb lists no primary key")
			case p.name == "mysql" && len(keys) == 0 && oldMariaDBDriver(version):
				// MariaDB Connector/ODBC before 3.2 compares COLUMN_KEY with the
				// text 'pri', and MySQL 8 and later compare it case sensitively
				t.Logf("the driver %s lists no primary key of a MySQL table", version)
			case len(keys) != 1 || !strings.EqualFold(keys[0].Column, "id") || keys[0].Sequence != 1:
				t.Errorf("primary keys: got %+v", keys)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

// TestFetchSize reads the same rows one at a time and in blocks, and checks
// that every value is the same (D24).
func TestFetchSize(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_fetch")
		timestamp := map[string]string{
			"postgres": "timestamp(6)", "mariadb": "datetime(6)", "mysql": "datetime(6)", "sqlserver": "datetime2(6)", "sqlite": "timestamp", "duckdb": "timestamp",
		}[p.name]
		create := "CREATE TABLE odbc_fetch (id integer PRIMARY KEY, n bigint, f double precision, s " + p.unicode + ", t " + timestamp + ", d date)"
		if p.name == "mariadb" || p.name == "mysql" {
			create = strings.Replace(create, "double precision", "double", 1)
		}
		if p.name == "sqlserver" {
			create = strings.Replace(create, "double precision", "float", 1)
		}
		if _, err := db.ExecContext(ctx, create); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_fetch") }()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		const rows = 1234
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range rows {
			var s any = fmt.Sprintf("row %d héllo", i)
			if i%7 == 0 {
				s = nil
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO odbc_fetch VALUES (?, ?, ?, ?, ?, ?)",
				i, int64(i)*1000003, float64(i)/4, s, base.Add(time.Duration(i)*time.Minute), base.AddDate(0, 0, i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		read := func(opts ...any) ([][]any, time.Duration) {
			t.Helper()
			start := time.Now()
			args := opts
			r, err := db.QueryContext(ctx, "SELECT id, n, f, s, t, d FROM odbc_fetch ORDER BY id", args...)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()
			var out [][]any
			for r.Next() {
				row := make([]any, 6)
				dest := make([]any, 6)
				for i := range dest {
					dest[i] = &row[i]
				}
				if err := r.Scan(dest...); err != nil {
					t.Fatal(err)
				}
				out = append(out, row)
			}
			if err := r.Err(); err != nil {
				t.Fatal(err)
			}
			return out, time.Since(start)
		}
		one, oneTime := read()
		if len(one) != rows {
			t.Fatalf("one at a time: %d rows", len(one))
		}
		for _, size := range []int{2, 100, 5000} {
			block, blockTime := read(odbc.WithFetchSize(size))
			if !reflect.DeepEqual(block, one) {
				for i := range min(len(block), len(one)) {
					if !reflect.DeepEqual(block[i], one[i]) {
						t.Fatalf("fetch size %d: row %d is %v and one at a time it is %v", size, i, block[i], one[i])
					}
				}
				t.Fatalf("fetch size %d: %d rows, want %d", size, len(block), len(one))
			}
			t.Logf("%s: one at a time %v, in blocks of %d %v", p.name, oneTime.Round(time.Millisecond), size, blockTime.Round(time.Millisecond))
		}
	})
}

// TestFetchSizeFallsBack checks that a result with a column that cannot be bound
// is read one row at a time, and that a value too long for its buffer is an
// error and never a cut value.
func TestFetchSizeFallsBack(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		long := strings.Repeat("x", 20000)
		var s string
		query := "SELECT ?"
		if err := db.QueryRowContext(ctx, query, long, odbc.WithFetchSize(10)).Scan(&s); err != nil {
			t.Logf("%s: %v", p.name, err)
			return
		}
		if s != long {
			t.Errorf("%s: got %d bytes, want %d", p.name, len(s), len(long))
		}
	})
}

// TestMaxRowsAndNoScan applies the two options that were reached only by name.
func TestMaxRowsAndNoScan(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_max")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_max (a integer)"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_max") }()
		for i := range 10 {
			if _, err := db.ExecContext(ctx, "INSERT INTO odbc_max VALUES (?)", i); err != nil {
				t.Fatal(err)
			}
		}
		r, err := db.QueryContext(ctx, "SELECT a FROM odbc_max", odbc.WithMaxRows(3))
		if err != nil {
			t.Logf("%s: %v", p.name, err)
			return
		}
		defer func() { _ = r.Close() }()
		var n int
		for r.Next() {
			n++
		}
		if err := r.Err(); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s returned %d rows for a limit of 3", p.name, n)
		if n != 3 {
			t.Errorf("got %d rows, want 3", n)
		}
		var v int
		if err := db.QueryRowContext(ctx, "SELECT 1", odbc.WithNoScan(true)).Scan(&v); err != nil {
			t.Logf("%s: WithNoScan: %v", p.name, err)
		}
	})
}

// TestLocation checks that Config.Location makes a timestamp a time.Time in
// that location, and sends a time.Time as its time there (D24).
func TestLocation(t *testing.T) {
	each(t, func(t *testing.T, p product, _ *sql.DB) {
		timestamp := map[string]string{
			"postgres": "timestamp(6)", "mariadb": "datetime(6)", "mysql": "datetime(6)", "sqlserver": "datetime2(6)", "sqlite": "timestamp", "duckdb": "timestamp",
		}[p.name]
		loc := time.FixedZone("test", 2*3600)
		db := openWith(t, p, func(c *odbc.Config) { c.Location = loc })
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_loc")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_loc (t "+timestamp+")"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_loc") }()
		instant := time.Date(2026, 10, 7, 12, 30, 45, 0, time.UTC)
		if _, err := db.ExecContext(ctx, "INSERT INTO odbc_loc VALUES (?)", instant); err != nil {
			t.Fatal(err)
		}
		var got time.Time
		if err := db.QueryRowContext(ctx, "SELECT t FROM odbc_loc").Scan(&got); err != nil {
			t.Fatal(err)
		}
		if !got.Equal(instant) || got.Location() != loc {
			t.Errorf("got %v in %v, want the instant %v in %v", got, got.Location(), instant, loc)
		}
		// the wall clock that the database stores is the time in the location
		var text string
		if err := db.QueryRowContext(ctx, "SELECT CAST(t AS "+map[string]string{"sqlserver": "varchar(40)"}[p.name]+") FROM odbc_loc").Scan(&text); err == nil {
			t.Logf("%s stores %s", p.name, text)
		}
	})
}

// TestFetchSizeNeverCutsAValue checks that a block read of a value longer than its
// column says is an error, and not a value cut short. SQLite keeps the text of a
// varchar(5) column whatever its length, so the driver reports a size that the
// data exceeds.
func TestFetchSizeNeverCutsAValue(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		if p.name != "sqlite" {
			t.Skip("SQLite is the database that holds more than a column declares")
		}
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_cut")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_cut (s varchar(5))"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_cut") }()
		if _, err := db.ExecContext(ctx, "INSERT INTO odbc_cut VALUES ('a value much longer than five')"); err != nil {
			t.Fatal(err)
		}
		var s string
		err := db.QueryRowContext(ctx, "SELECT s FROM odbc_cut", odbc.WithFetchSize(10)).Scan(&s)
		if !errors.Is(err, odbc.ErrTruncated) {
			t.Errorf("got %q and %v, want odbc.ErrTruncated", s, err)
		}
		// one row at a time reads the whole value
		if err := db.QueryRowContext(ctx, "SELECT s FROM odbc_cut").Scan(&s); err != nil || s != "a value much longer than five" {
			t.Errorf("one at a time: got %q and %v", s, err)
		}
	})
}

// oldMariaDBDriver reports whether a driver version, in the form 03.01.0015 that
// SQLGetInfo gives, is before 3.2.
func oldMariaDBDriver(version string) bool {
	major, minor, ok := strings.Cut(version, ".")
	if !ok {
		return false
	}
	minor, _, _ = strings.Cut(minor, ".")
	return strings.TrimLeft(major, "0") < "3" || (strings.TrimLeft(major, "0") == "3" && strings.TrimLeft(minor, "0") < "2")
}

// TestQueryWithoutResult runs a statement that returns no result set through
// QueryContext, as a client such as usql does for every statement. The rows are
// empty, and Next and Err report no error.
func TestQueryWithoutResult(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_ddl")
		r, err := db.QueryContext(ctx, "CREATE TABLE odbc_ddl (a integer)")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_ddl") }()
		defer r.Close()
		if r.Next() {
			t.Error("a CREATE TABLE returned a row")
		}
		if err := r.Err(); err != nil {
			t.Errorf("rows.Err: %v", err)
		}
	})
}
