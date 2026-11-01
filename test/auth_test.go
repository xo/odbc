package test

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/odbc"
)

// refused asks every error of the tree, as usql does, whether the database refused
// the credential.
func refused(err error) bool {
	type refusal interface{ AuthenticationRefused() bool }
	for err != nil {
		if e, ok := err.(refusal); ok && e.AuthenticationRefused() {
			return true
		}
		switch u := err.(type) { //nolint:errorlint // the walk asks every error of the tree, as usql does
		case interface{ Unwrap() []error }:
			return slices.ContainsFunc(u.Unwrap(), refused)
		case interface{ Unwrap() error }:
			err = u.Unwrap()
		default:
			return false
		}
	}
	return false
}

// withCredential opens the database of a product with another user and password,
// and returns the first error of a connection and a statement.
func withCredential(t *testing.T, p product, user, pass, statement string) error {
	t.Helper()
	cfg, err := odbc.ParseDSN(dsnOf(t, p))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(cfg.ConnString, ";")
	for i, s := range parts {
		switch {
		case strings.HasPrefix(s, "UID="):
			parts[i] = "UID=" + user
		case strings.HasPrefix(s, "PWD="):
			parts[i] = "PWD=" + pass
		}
	}
	cfg.ConnString = strings.Join(parts, ";")
	c, err := odbc.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(c)
	defer func() { _ = db.Close() }()
	if err := db.PingContext(t.Context()); err != nil {
		return err
	}
	_, err = db.ExecContext(t.Context(), statement)
	return err
}

// TestAuthenticationRefused asks each database that has users for three
// refusals: a wrong password, a user it does not know, and a refusal for lack of
// a permission. Only the first two are a refused credential (D25).
func TestAuthenticationRefused(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		if p.embedded {
			t.Skipf("%s has no users", p.name)
		}
		drop, create := limitedUser(p.name)
		ctx := t.Context()
		for _, s := range drop {
			_, _ = db.ExecContext(ctx, s)
		}
		for _, s := range create {
			if _, err := db.ExecContext(ctx, s); err != nil {
				t.Fatalf("%s: %v", s, err)
			}
		}
		t.Cleanup(func() {
			for _, s := range drop {
				_, _ = db.ExecContext(ctx, s)
			}
		})
		// The user connects with the right password. A statement that it has no
		// right to run is a refusal of a permission, not of the credential.
		denied := withCredential(t, p, limitedName, limitedPassword, "SELECT a FROM odbc_auth_t")
		if denied == nil {
			t.Fatalf("%s: the limited user read a table that it has no right to read", p.name)
		}
		if !errors.Is(denied, odbc.ErrSyntax) {
			// Class 42 is the class of a refused permission. Any other class means
			// that the statement did not get as far as the permission.
			t.Fatalf("%s: got %v, want a refusal of a permission in the class 42", p.name, denied)
		}
		if refused(denied) {
			t.Errorf("%s: a refusal of a permission counts as a refused credential: %v", p.name, denied)
		}
		for name, tt := range map[string]struct{ user, pass string }{
			"a wrong password": {limitedName, "not the password"},
			"an unknown user":  {"nobody_here", "x"},
		} {
			err := withCredential(t, p, tt.user, tt.pass, "SELECT 1")
			var e *odbc.Error
			if !errors.As(err, &e) {
				t.Errorf("%s with %s: got %v, want an *odbc.Error", p.name, name, err)
				continue
			}
			if !refused(err) {
				t.Errorf("%s with %s: not a refused credential: state %s, native %d: %v", p.name, name, e.State, e.Native, err)
			}
		}
	})
}

// TestAuthenticationNotRefusedForANetworkFailure checks that a server that is not
// there is not a refused credential. A client then does not ask for a password
// that cannot help.
func TestAuthenticationNotRefusedForANetworkFailure(t *testing.T) {
	each(t, func(t *testing.T, p product, _ *sql.DB) {
		if p.embedded {
			t.Skipf("%s has no server", p.name)
		}
		cfg, err := odbc.ParseDSN(dsnOf(t, p))
		if err != nil {
			t.Fatal(err)
		}
		// port 1 holds no server
		parts := strings.Split(cfg.ConnString, ";")
		for i, s := range parts {
			switch {
			case strings.HasPrefix(s, "PORT="), strings.HasPrefix(s, "Port="):
				parts[i] = "Port=1"
			case strings.HasPrefix(s, "Server=") && strings.Contains(s, ","):
				parts[i] = "Server=127.0.0.1,1"
			}
		}
		cfg.ConnString = strings.Join(parts, ";")
		c, err := odbc.NewConnector(cfg)
		if err != nil {
			t.Fatal(err)
		}
		db := sql.OpenDB(c)
		defer func() { _ = db.Close() }()
		err = db.PingContext(t.Context())
		if err == nil {
			t.Fatal("a connection to port 1 worked")
		}
		if refused(err) {
			t.Errorf("%s: a network failure counts as a refused credential: %v", p.name, err)
		}
	})
}

const (
	limitedName     = "odbc_lim"
	limitedPassword = "L1mited!pass9"
)

// limitedUser returns the statements that drop and make a user that can connect
// and can read nothing, with a table that the user has no right to read.
func limitedUser(product string) (drop, create []string) {
	switch product {
	case "postgres":
		return []string{"DROP TABLE IF EXISTS odbc_auth_t", "DROP ROLE IF EXISTS odbc_lim"},
			[]string{"CREATE TABLE odbc_auth_t (a integer)", "CREATE ROLE odbc_lim LOGIN PASSWORD '" + limitedPassword + "'"}
	case "mariadb", "mysql":
		return []string{"DROP TABLE IF EXISTS mysql.odbc_auth_t", "DROP USER IF EXISTS 'odbc_lim'@'%'"},
			[]string{"CREATE TABLE mysql.odbc_auth_t (a integer)", "CREATE USER 'odbc_lim'@'%' IDENTIFIED BY '" + limitedPassword + "'"}
	default:
		return []string{"DROP TABLE IF EXISTS odbc_auth_t", "IF EXISTS (SELECT 1 FROM sys.server_principals WHERE name = 'odbc_lim') DROP LOGIN odbc_lim"},
			[]string{"CREATE TABLE odbc_auth_t (a integer)", "CREATE LOGIN odbc_lim WITH PASSWORD = '" + limitedPassword + "', CHECK_POLICY = OFF"}
	}
}
