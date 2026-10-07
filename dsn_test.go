package odbc_test

import (
	"testing"

	"github.com/xo/odbc"
)

func TestParseDSN(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		dsn  string
		want odbc.Config
	}{{
		"DRIVER={SQLite3};Database=:memory:",
		odbc.Config{ConnString: "DRIVER={SQLite3};Database=:memory:"},
	}, {
		"odbc+PostgreSQL+Unicode://u:p@localhost:5432/db",
		odbc.Config{ConnString: "Driver={PostgreSQL Unicode};Server=localhost;Port=5432;Database=db;UID=u;PWD=p;"},
	}, {
		`odbc+ODBC+Driver+18+for+SQL+Server://sa:p%3Bw@host/inst/db?TrustServerCertificate=yes`,
		odbc.Config{ConnString: `Driver={ODBC Driver 18 for SQL Server};Server=host\inst;Database=db;UID=sa;PWD={p;w};TrustServerCertificate=yes;`},
	}, {
		"odbc+ODBC+Driver+18+for+SQL+Server://sa:p@host:1433/db",
		odbc.Config{ConnString: "Driver={ODBC Driver 18 for SQL Server};Server=host,1433;Database=db;UID=sa;PWD=p;"},
	}, {
		"odbc+MariaDB://u:p@h:3306/db?INITSTMT=SET+SESSION+sql_mode%3D%27ANSI%27",
		odbc.Config{ConnString: "Driver=MariaDB;Server=h;Port=3306;Database=db;UID=u;PWD=p;INITSTMT={SET SESSION sql_mode='ANSI'};"},
	}, {
		"odbc+x://h/db?driver=/usr/lib/x.so&manager=/lib/libodbc.so.2&wchar=4",
		odbc.Config{ConnString: "Driver=/usr/lib/x.so;Server=h;Database=db;", Manager: "/lib/libodbc.so.2", WChar: 4},
	}} {
		got, err := odbc.ParseDSN(tt.dsn)
		if err != nil {
			t.Errorf("%s: %v", tt.dsn, err)
			continue
		}
		if got.ConnString != tt.want.ConnString || got.Manager != tt.want.Manager || got.WChar != tt.want.WChar {
			t.Errorf("%s:\n got %+v\nwant %+v", tt.dsn, got, tt.want)
		}
	}
	for _, bad := range []string{"", "postgres://h/db", "odbc://h/db", "odbc+x://h/db?wchar=3"} {
		if _, err := odbc.ParseDSN(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}
