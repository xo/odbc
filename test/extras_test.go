package test

import (
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/xo/odbc"
)

// TestGetInfo asks each database about itself through sql.Conn.Raw (D23).
func TestGetInfo(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		err = conn.Raw(func(dc any) error {
			c, ok := dc.(odbc.Conn)
			if !ok {
				t.Fatalf("%T is not an odbc.Conn", dc)
			}
			name, err := c.GetInfoString(odbc.InfoDBMSName)
			if err != nil {
				return err
			}
			version, err := c.GetInfoString(odbc.InfoDBMSVersion)
			if err != nil {
				return err
			}
			quote, err := c.GetInfoString(odbc.InfoIdentifierQuoteChar)
			if err != nil {
				return err
			}
			t.Logf("%s reports %q, version %q, quote %q", p.name, name, version, quote)
			if name == "" || version == "" {
				t.Errorf("the name is %q and the version is %q", name, version)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}

// TestDrivers lists the drivers and the data sources that the manager knows. The
// list holds what the machine has, so the test checks its shape only.
func TestDrivers(t *testing.T) {
	t.Parallel()
	drivers, err := odbc.Drivers(odbc.Config{})
	if err != nil {
		t.Skipf("no driver manager: %v", err)
	}
	for _, d := range drivers {
		t.Logf("driver %q %v", d.Name, d.Attributes)
		if d.Name == "" {
			t.Error("a driver has no name")
		}
	}
	sources, err := odbc.DataSources(odbc.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sources {
		t.Logf("data source %q (%s)", s.Name, s.Description)
	}
}

// TestTrace turns the trace of the driver manager on and checks that it wrote.
func TestTrace(t *testing.T) {
	each(t, func(t *testing.T, p product, _ *sql.DB) {
		file := t.TempDir() + "/trace.log"
		db := openWith(t, p, func(c *odbc.Config) { c.TraceFile = file })
		var n int
		if err := db.QueryRowContext(t.Context(), "SELECT 1").Scan(&n); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
		info, err := os.Stat(file)
		if err != nil {
			t.Fatalf("the trace file: %v", err)
		}
		if info.Size() == 0 {
			t.Error("the trace file is empty")
		}
	})
}

// TestWarnings checks that an informational diagnostic reaches OnWarning. SQL
// Server returns the text of a PRINT statement that way.
func TestWarnings(t *testing.T) {
	each(t, func(t *testing.T, p product, _ *sql.DB) {
		statement := map[string]string{
			"sqlserver": "PRINT 'a message'",
			"postgres":  "DO $$ BEGIN RAISE NOTICE 'a message'; END $$",
		}[p.name]
		if statement == "" {
			t.Skipf("%s has no statement that returns a warning", p.name)
		}
		var mu sync.Mutex
		var got []string
		db := openWith(t, p, func(c *odbc.Config) {
			c.OnWarning = func(e *odbc.Error) {
				mu.Lock()
				defer mu.Unlock()
				got = append(got, e.Message)
			}
		})
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		if !strings.Contains(strings.Join(got, "\n"), "a message") {
			t.Errorf("OnWarning got %q", got)
		}
	})
}

// TestInitStmt checks the recipe of the README for ANSI SQL mode: the INITSTMT key
// of the MariaDB driver runs a statement when a connection opens.
func TestInitStmt(t *testing.T) {
	each(t, func(t *testing.T, p product, _ *sql.DB) {
		if p.name != "mariadb" && p.name != "mysql" {
			t.Skipf("%s has no sql_mode", p.name)
		}
		db := openWith(t, p, func(c *odbc.Config) {
			c.ConnString += ";INITSTMT={SET SESSION sql_mode='ANSI'}"
		})
		ctx := t.Context()
		var mode string
		if err := db.QueryRowContext(ctx, "SELECT @@sql_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(mode, "ANSI_QUOTES") {
			t.Errorf("sql_mode is %q, and it must hold ANSI_QUOTES", mode)
		}
		// double quotes name an identifier in ANSI mode
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "information_schema"."tables" WHERE "table_schema" = 'mysql'`).Scan(&n); err != nil || n == 0 {
			t.Errorf("a query with quoted identifiers: %d rows and %v", n, err)
		}
	})
}
