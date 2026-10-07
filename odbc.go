// Package odbc is a database/sql driver for ODBC, written in pure Go.
//
// It loads the ODBC driver manager of the system at run time with purego, so
// it needs no cgo and no C compiler. It targets Windows, macOS and Linux with
// the same code.
//
// Import it for its side effect and open a database by name:
//
//	import _ "github.com/xo/odbc"
//
//	db, err := sql.Open("odbc", "odbc+PostgreSQL+Unicode://user:pass@localhost/db")
//
// The data source name is described by [ParseDSN]. A value has the Go type
// that dbimp gives its kind, so a decimal is a *apd.Decimal and a date is a
// dbimp.Date.
package odbc

import (
	"database/sql"
	"database/sql/driver"
	"errors"
)

// Name is the name the driver registers under.
const Name = "odbc"

func init() {
	sql.Register(Name, &Driver{})
}

// Driver is the database/sql driver.
type Driver struct{}

var (
	_ driver.Driver        = (*Driver)(nil)
	_ driver.DriverContext = (*Driver)(nil)
)

// Open refuses, because a connection needs a context. database/sql calls
// [Driver.OpenConnector] instead.
func (*Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("opening a connection: use OpenConnector, which takes a context")
}

// OpenConnector parses the data source name and returns a connector. It loads
// the driver manager and allocates the environment, so a failure shows here
// and not at the first query.
func (*Driver) OpenConnector(dsn string) (driver.Connector, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return NewConnector(cfg)
}
