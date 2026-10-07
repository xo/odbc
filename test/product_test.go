package test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/duckdb"
	dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"
	_ "github.com/xo/dbmeta/models/mysql"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	_ "github.com/xo/dbmeta/models/postgres"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
	_ "github.com/xo/dbmeta/models/sqlite3"
	sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"
	_ "github.com/xo/dbmeta/models/sqlserver"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
	"github.com/xo/odbc"
)

// step is one statement of a fixture, with its name.
type step struct{ name, query string }

// product is one database that the driver is tested against.
type product struct {
	// name names the product in the test name.
	name string
	// env is the environment variable that holds the data source name.
	// env+"_URL" holds the URL that dbrun prints for the server, and
	// env+"_DRIVER" holds the name or the path of the ODBC driver. See dsnOf.
	env string
	// scheme is the name of the ODBC driver when no variable names one.
	scheme string
	// embedded is true for a database that is a library and a file, and has
	// no server and no URL.
	embedded bool
	dialect  dbmeta.Dialect
	// schema is the schema that the fixture builds.
	schema string
	// fixture returns the statements that build and drop the fixture.
	fixture func(dbmeta.VersionSet) (up, down []step, err error)
	// unicode is the type of a column that holds any text.
	unicode string
	// binary is the type of a column that holds bytes.
	binary string
}

func convert[R interface {
	~struct {
		Name    string
		Query   string
		Skipped bool
		Reason  string
	}
}](rs []R) []step {
	var out []step
	for _, r := range rs {
		s := struct {
			Name    string
			Query   string
			Skipped bool
			Reason  string
		}(r)
		if !s.Skipped {
			out = append(out, step{s.Name, s.Query})
		}
	}
	return out
}

var products = []product{{
	name: "postgres", env: "ODBC_POSTGRES", scheme: "PostgreSQL+Unicode", dialect: dbmeta.PostgreSQL,
	schema:  pgfixture.Everything.Schema,
	unicode: "varchar(200)", binary: "bytea",
	fixture: func(v dbmeta.VersionSet) (up, down []step, err error) {
		u, err := pgfixture.Everything.ResolveSetup(v)
		if err != nil {
			return nil, nil, err
		}
		d, err := pgfixture.Everything.ResolveTeardown(v)
		return convert(u), convert(d), err
	},
}, {
	name: "mariadb", env: "ODBC_MARIADB", scheme: "MariaDB", dialect: dbmeta.MySQL,
	schema:  myfixture.Everything.Schema,
	unicode: "varchar(200)", binary: "varbinary(200)",
	fixture: myFixture,
}, {
	name: "mysql", env: "ODBC_MYSQL", scheme: "MariaDB", dialect: dbmeta.MySQL,
	schema:  myfixture.Everything.Schema,
	unicode: "varchar(200)", binary: "varbinary(200)",
	fixture: myFixture,
}, {
	name: "sqlserver", env: "ODBC_SQLSERVER", scheme: "ODBC+Driver+18+for+SQL+Server", dialect: dbmeta.SQLServer,
	schema:  msfixture.Everything.Schema,
	unicode: "nvarchar(200)", binary: "varbinary(200)",
	fixture: func(v dbmeta.VersionSet) (up, down []step, err error) {
		u, err := msfixture.Everything.ResolveSetup(v)
		if err != nil {
			return nil, nil, err
		}
		d, err := msfixture.Everything.ResolveTeardown(v)
		return convert(u), convert(d), err
	},
}, {
	name: "duckdb", env: "ODBC_DUCKDB", embedded: true, scheme: "DuckDB Driver", dialect: dbmeta.DuckDB,
	schema:  dkfixture.Everything.Schema,
	unicode: "varchar", binary: "blob",
	fixture: func(v dbmeta.VersionSet) (up, down []step, err error) {
		u, err := dkfixture.Everything.ResolveSetup(v)
		if err != nil {
			return nil, nil, err
		}
		d, err := dkfixture.Everything.ResolveTeardown(v)
		return convert(u), convert(d), err
	},
}, {
	name: "sqlite", env: "ODBC_SQLITE", embedded: true, scheme: "SQLite3", dialect: dbmeta.SQLite3,
	schema:  sqfixture.Everything.Schema,
	unicode: "varchar(200)", binary: "blob",
	fixture: func(v dbmeta.VersionSet) (up, down []step, err error) {
		u, err := sqfixture.Everything.ResolveSetup(v)
		if err != nil {
			return nil, nil, err
		}
		d, err := sqfixture.Everything.ResolveTeardown(v)
		return convert(u), convert(d), err
	},
}}

func myFixture(v dbmeta.VersionSet) (up, down []step, err error) {
	u, err := myfixture.Everything.ResolveSetup(v)
	if err != nil {
		return nil, nil, err
	}
	d, err := myfixture.Everything.ResolveTeardown(v)
	return convert(u), convert(d), err
}

// each runs fn for every product whose environment variable is set, and
// skips the rest.
func each(t *testing.T, fn func(t *testing.T, p product, db *sql.DB)) {
	t.Helper()
	for _, p := range products {
		t.Run(p.name, func(t *testing.T) {
			dsn := dsnOf(t, p)
			if dsn == "" {
				t.Skipf("%s is not set", p.env)
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
			fn(t, p, db)
		})
	}
}

// dsnOf returns the data source name of a product, or "" when the environment
// names none. Three variables can name it, and the first that is set wins:
//
//   - env holds the data source name itself.
//   - env_URL holds the URL that dbrun prints for the server. The driver is the
//     one in env_DRIVER, which is a name or a path, or else the default name of
//     the product.
//   - env_DRIVER alone names the driver of an embedded database, which gets a
//     file in the directory of the test.
func dsnOf(t *testing.T, p product) string {
	t.Helper()
	if v := os.Getenv(p.env); v != "" {
		return v
	}
	driver := os.Getenv(p.env + "_DRIVER")
	switch u := os.Getenv(p.env + "_URL"); {
	case u != "":
		dsn, err := fromURL(p, u, driver)
		if err != nil {
			t.Fatalf("%s_URL: %v", p.env, err)
		}
		return dsn
	case p.embedded && driver != "":
		file := filepath.Join(t.TempDir(), "odbc.db")
		return fmt.Sprintf("DRIVER={%s};Database=%s", driver, file)
	}
	return ""
}

// fromURL turns the URL that dbrun prints for a server into a data source name
// for this driver. The database is the path of the URL, or the database
// parameter that a SQL Server URL has, or mysql for a URL with neither.
func fromURL(p product, raw, driver string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	db := strings.Trim(u.Path, "/")
	q := u.Query()
	if v := q.Get("database"); v != "" {
		db = v
	}
	out := url.Values{}
	switch p.name {
	case "postgres":
		out.Set("sslmode", "disable")
	case "mariadb", "mysql":
		if db == "" {
			db = "mysql"
		}
	case "sqlserver":
		out.Set("TrustServerCertificate", "yes")
	default:
		return "", fmt.Errorf("%s has no server", p.name)
	}
	if driver != "" {
		out.Set("driver", driver)
	}
	dsn := "odbc+" + p.scheme + "://" + u.User.String() + "@" + u.Host + "/" + db
	if len(out) > 0 {
		dsn += "?" + out.Encode()
	}
	return dsn, nil
}

// openWith opens the database of a product through a connector whose Config the
// caller changes, and skips the test when the environment names no database.
func openWith(t *testing.T, p product, change func(*odbc.Config)) *sql.DB {
	t.Helper()
	dsn := dsnOf(t, p)
	if dsn == "" {
		t.Skipf("%s is not set", p.env)
	}
	cfg, err := odbc.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	change(&cfg)
	c, err := odbc.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
