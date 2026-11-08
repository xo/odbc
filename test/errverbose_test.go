package test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/xo/odbc"
)

// TestErrVerbose asks each database for an error in a table that does not exist,
// and checks the verbose form of it against the layout of psql.
func TestErrVerbose(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		_, err := db.ExecContext(t.Context(), "SELECT a FROM odbc_no_such_table")
		if err == nil {
			t.Fatalf("%s: a missing table gave no error", p.name)
		}
		var e *odbc.Error
		if !errors.As(err, &e) {
			t.Fatalf("%s: %v is not an *odbc.Error", p.name, err)
		}
		got := e.ErrVerbose("SELECT a FROM odbc_no_such_table")
		t.Logf("%s:\n%s", p.name, got)
		if !strings.HasPrefix(got, "ERROR:  "+e.State+": ") || !strings.Contains(got, "\nCONTEXT:  in ") || strings.HasSuffix(got, "\n") {
			t.Errorf("%s: the verbose form is not in the layout of psql:\n%s", p.name, got)
		}
	})
}
