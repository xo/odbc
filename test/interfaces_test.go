package test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/dbimp"
	"github.com/xo/odbc"
)

// TestColumnTypeLengthUnbounded checks the contract of ColumnType.Length: a type
// with no limit reports math.MaxInt64, and a bounded type reports its length (D27).
func TestColumnTypeLengthUnbounded(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		unbounded := map[string]string{
			"postgres": "text", "mariadb": "longtext", "mysql": "longtext", "sqlserver": "nvarchar(max)",
		}[p.name]
		if unbounded == "" {
			t.Skipf("%s has no type that is known to be unbounded", p.name)
		}
		query := "SELECT CAST('x' AS " + unbounded + ") AS u, CAST('x' AS varchar(10)) AS b"
		if p.name == "mariadb" || p.name == "mysql" {
			// MySQL has no longtext to cast to, so read a column
			ctx := t.Context()
			_, _ = db.ExecContext(ctx, "DROP TABLE odbc_len")
			if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_len (u longtext, b varchar(10))"); err != nil {
				t.Fatal(err)
			}
			defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_len") }()
			query = "SELECT u, b FROM odbc_len"
		}
		rows, err := db.QueryContext(t.Context(), query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		cts, err := rows.ColumnTypes()
		if err != nil {
			t.Fatal(err)
		}
		if l, ok := cts[0].Length(); !ok || l != math.MaxInt64 {
			t.Errorf("%s: the unbounded column has length %d, %v, want math.MaxInt64", p.name, l, ok)
		}
		if l, ok := cts[1].Length(); !ok || l != 10 {
			t.Errorf("%s: varchar(10) has length %d, %v, want 10", p.name, l, ok)
		}
	})
}

// TestUint64Argument sends the largest uint64, which database/sql refuses by
// itself, and reads it back (D27).
func TestUint64Argument(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		if p.name == "sqlite" {
			t.Skip("SQLite has no decimal type")
		}
		ctx := t.Context()
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_u64")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_u64 (v decimal(20,0))"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_u64") }()
		for _, v := range []any{uint64(math.MaxUint64), uint(math.MaxUint64), uint64(7)} {
			if _, err := db.ExecContext(ctx, "DELETE FROM odbc_u64"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO odbc_u64 VALUES (?)", v); err != nil {
				t.Fatalf("%T %v: %v", v, v, err)
			}
			var got string
			text := "varchar(30)"
			if p.name == "mysql" || p.name == "mariadb" {
				text = "char"
			}
			if err := db.QueryRowContext(ctx, "SELECT CAST(v AS "+text+") FROM odbc_u64").Scan(&got); err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprint(v); got != want {
				t.Errorf("%s: %T went in as %v and came back as %q", p.name, v, v, got)
			}
		}
	})
}

// TestLastInsertID reads the id of an insert with WithLastInsertID, on the
// databases that have a safe statement for it, and checks the error on the
// others (D27).
func TestLastInsertID(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		auto := map[string]string{
			"mariadb": "integer PRIMARY KEY AUTO_INCREMENT", "mysql": "integer PRIMARY KEY AUTO_INCREMENT",
			"sqlite":   "integer PRIMARY KEY AUTOINCREMENT",
			"postgres": "serial PRIMARY KEY", "sqlserver": "integer IDENTITY(1,1) PRIMARY KEY",
			"duckdb": "integer",
		}[p.name]
		_, _ = db.ExecContext(ctx, "DROP TABLE odbc_ins")
		if _, err := db.ExecContext(ctx, "CREATE TABLE odbc_ins (id "+auto+", note varchar(10))"); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = db.ExecContext(ctx, "DROP TABLE odbc_ins") }()
		insert := "INSERT INTO odbc_ins (note) VALUES (?)"
		// without the option the driver says how to get the id
		res, err := db.ExecContext(ctx, insert, "a")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := res.LastInsertId(); !errors.Is(err, dbimp.ErrNotSupported) {
			t.Errorf("%s without the option: got %v, want dbimp.ErrNotSupported", p.name, err)
		}
		switch p.name {
		case "mariadb", "mysql", "sqlite":
			var ids []int64
			for i := range 3 {
				arg := fmt.Sprint("n", i)
				res, err := db.ExecContext(ctx, insert, arg, odbc.WithLastInsertID(true))
				if err != nil {
					t.Fatal(err)
				}
				id, err := res.LastInsertId()
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			if ids[1] != ids[0]+1 || ids[2] != ids[1]+1 || ids[0] < 2 {
				t.Errorf("%s: the ids are %v, and each must follow the one before", p.name, ids)
			}
		default:
			_, err := db.ExecContext(ctx, insert, "b", odbc.WithLastInsertID(true))
			if !errors.Is(err, dbimp.ErrNotSupported) {
				t.Errorf("%s with the option: got %v, want dbimp.ErrNotSupported", p.name, err)
			}
		}
	})
}

// TestPingReachesTheServer kills the session of a connection from another
// connection, and checks that Ping on the killed connection fails at once and not
// at its next query (D27).
func TestPingReachesTheServer(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		if p.name != "mariadb" && p.name != "mysql" {
			t.Skip("the test kills a session by the id that MySQL and MariaDB give")
		}
		ctx := t.Context()
		victim, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = victim.Close() }()
		var id int64
		if err := victim.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := victim.Raw(func(dc any) error { return dc.(driver.Pinger).Ping(ctx) }); err != nil { //nolint:forcetypeassert // the connection is the driver of this test
			t.Fatalf("a Ping before the kill: %v", err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("KILL %d", id)); err != nil {
			t.Fatal(err)
		}
		err = victim.Raw(func(dc any) error { return dc.(driver.Pinger).Ping(ctx) }) //nolint:forcetypeassert // see above
		if err == nil {
			t.Error("a Ping after the kill worked")
		} else {
			t.Logf("%s: a Ping after the kill says %v", p.name, err)
		}
	})
}

// TestWithDatabaseInATransaction checks that a statement of a transaction cannot
// name another catalog, because the transaction runs in one (D22).
func TestWithDatabaseInATransaction(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		other := map[string]string{"mariadb": "information_schema", "mysql": "information_schema", "sqlserver": "tempdb"}[p.name]
		query := map[string]string{"mariadb": "SELECT DATABASE()", "mysql": "SELECT DATABASE()", "sqlserver": "SELECT DB_NAME()"}[p.name]
		if other == "" {
			t.Skipf("%s does not honor WithDatabase", p.name)
		}
		ctx := t.Context()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var name string
		err = tx.QueryRowContext(ctx, query, odbc.WithDatabase(other)).Scan(&name)
		if !errors.Is(err, dbimp.ErrNotSupported) {
			t.Errorf("%s: got %q and %v, want dbimp.ErrNotSupported", p.name, name, err)
		}
		// without the option the transaction works, and after it ends the option works
		if err := tx.QueryRowContext(ctx, query).Scan(&name); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		err = db.QueryRowContext(ctx, query, odbc.WithDatabase(other)).Scan(&name)
		if err != nil || !strings.EqualFold(name, other) {
			t.Errorf("%s after the transaction: got %q and %v, want %q", p.name, name, err, other)
		}
	})
}
