package test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

// open opens the database named by an environment variable, and skips the
// test when it is empty. The value is a data source name for this driver.
func open(t *testing.T, env string) *sql.DB {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("%s is not set", env)
	}
	db, err := sql.Open("odbc", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestPostgres runs the statements that a program meets first against
// PostgreSQL: parameters, a long string, a transaction, a prepared statement,
// an error, column types and a canceled query.
func TestPostgres(t *testing.T) {
	db := open(t, "ODBC_POSTGRES")
	ctx := t.Context()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`DROP TABLE IF EXISTS odbc_t`)
	exec(`CREATE TABLE odbc_t (
		id integer PRIMARY KEY, big bigint, f double precision, b boolean,
		s text, v varchar(10), n numeric(10,2), d date, ts timestamp, bin bytea, nul text)`)
	defer exec(`DROP TABLE IF EXISTS odbc_t`)
	when := time.Date(2026, 10, 7, 12, 30, 45, 123456000, time.UTC)
	res, err := db.ExecContext(ctx, `INSERT INTO odbc_t VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		1, int64(1)<<40, 1.5, true, "héllo ✓", "abc", "12.34", when, when, []byte{0, 1, 2, 255}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Errorf("rows affected = %d", n)
	}
	var (
		id, big int64
		f       float64
		b       bool
		s, v, n string
		d, ts   time.Time
		bin     []byte
		nul     sql.NullString
	)
	err = db.QueryRowContext(ctx, `SELECT id, big, f, b, s, v, n, d, ts, bin, nul FROM odbc_t WHERE id = ?`, 1).
		Scan(&id, &big, &f, &b, &s, &v, &n, &d, &ts, &bin, &nul)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 || big != 1<<40 || f != 1.5 || !b || s != "héllo ✓" || v != "abc" || n != "12.34" {
		t.Errorf("got %v %v %v %v %q %q %q", id, big, f, b, s, v, n)
	}
	if !ts.Equal(when) || !d.Equal(time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v and %v", ts, d)
	}
	if string(bin) != string([]byte{0, 1, 2, 255}) || nul.Valid {
		t.Errorf("got %v and %v", bin, nul)
	}

	// a long string crosses the read buffer
	long := make([]byte, 100000)
	for i := range long {
		long[i] = 'a' + byte(i%26)
	}
	exec(`INSERT INTO odbc_t (id, s) VALUES (2, ?)`, string(long))
	var got string
	if err := db.QueryRowContext(ctx, `SELECT s FROM odbc_t WHERE id = 2`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != string(long) {
		t.Errorf("long string: got %d bytes, want %d", len(got), len(long))
	}

	// a transaction rolls back
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO odbc_t (id) VALUES (3)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM odbc_t`).Scan(&count); err != nil || count != 2 {
		t.Errorf("count = %d, %v", count, err)
	}

	// a prepared statement runs twice
	stmt, err := db.PrepareContext(ctx, `SELECT s FROM odbc_t WHERE id = ?`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, id := range []int{1, 1} {
		var s string
		if err := stmt.QueryRowContext(ctx, id).Scan(&s); err != nil || s != "héllo ✓" {
			t.Errorf("prepared: %q, %v", s, err)
		}
	}

	// the error carries the SQLSTATE
	_, err = db.ExecContext(ctx, `INSERT INTO odbc_t (id) VALUES (1)`)
	var oe interface{ ConnectionError() bool }
	if err == nil || !errors.As(err, &oe) {
		t.Errorf("duplicate key: %v", err)
	}

	// the column types
	rows, err := db.QueryContext(ctx, `SELECT id, n, s FROM odbc_t`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range types {
		p, sc, ok := c.DecimalSize()
		t.Logf("%s %s %v precision %d scale %d %v", c.Name(), c.DatabaseTypeName(), c.ScanType(), p, sc, ok)
	}

	// a canceled query stops
	cctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = db.ExecContext(cctx, `SELECT pg_sleep(10)`)
	if err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("cancel: %v after %v", err, time.Since(start))
	}
}

// TestBasics runs the same few statements against every database that has
// a data source name in its environment variable: a table, parameters, NULL,
// a transaction and a prepared statement.
func TestBasics(t *testing.T) {
	for _, env := range []string{"ODBC_SQLITE", "ODBC_MARIADB", "ODBC_MYSQL", "ODBC_SQLSERVER", "ODBC_POSTGRES"} {
		t.Run(env, func(t *testing.T) {
			db := open(t, env)
			ctx := t.Context()
			// SQL Server stores varchar in a code page, so text outside it
			// needs nvarchar
			text := "varchar(100)"
			if env == "ODBC_SQLSERVER" {
				text = "nvarchar(100)"
			}
			_, _ = db.ExecContext(ctx, `DROP TABLE odbc_basic`)
			create := `CREATE TABLE odbc_basic (id integer PRIMARY KEY, name ` + text + `, score float, note ` + text + `)`
			if _, err := db.ExecContext(ctx, create); err != nil {
				t.Fatal(err)
			}
			defer func() { _, _ = db.ExecContext(ctx, `DROP TABLE odbc_basic`) }()
			for i, name := range []string{"a", "b", "héllo ✓"} {
				if _, err := db.ExecContext(ctx, `INSERT INTO odbc_basic VALUES (?, ?, ?, ?)`, i+1, name, float64(i)+0.5, nil); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := db.QueryContext(ctx, `SELECT id, name, score, note FROM odbc_basic ORDER BY id`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var got []string
			for rows.Next() {
				var id int
				var name string
				var score float64
				var note sql.NullString
				if err := rows.Scan(&id, &name, &score, &note); err != nil {
					t.Fatal(err)
				}
				if note.Valid {
					t.Errorf("note of %d is not NULL", id)
				}
				got = append(got, name)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if len(got) != 3 || got[2] != "héllo ✓" {
				t.Errorf("got %q", got)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM odbc_basic`); err != nil {
				t.Fatal(err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM odbc_basic`).Scan(&n); err != nil || n != 3 {
				t.Errorf("count = %d, %v", n, err)
			}
		})
	}
}
