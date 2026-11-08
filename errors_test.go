package odbc

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestErrorClasses(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		state string
		is    []ClassError
		isNot []ClassError
	}{
		{"23505", []ClassError{ErrIntegrity}, []ClassError{ErrConnection, ErrSyntax}},
		{"08S01", []ClassError{ErrConnection}, []ClassError{ErrIntegrity}},
		{"40001", []ClassError{ErrRollback}, []ClassError{ErrTimeout}},
		{"HYT00", []ClassError{ErrTimeout}, []ClassError{ErrRollback}},
		{"42S02", []ClassError{ErrSyntax}, []ClassError{ErrData}},
		{"22003", []ClassError{ErrData}, []ClassError{ErrSyntax}},
	} {
		// a wrapped error, as a caller gets it from database/sql
		err := fmt.Errorf("querying: %w", &Error{Op: "executing", State: tt.state})
		for _, c := range tt.is {
			if !errors.Is(err, c) {
				t.Errorf("%s: errors.Is(err, %q) is false", tt.state, c)
			}
		}
		for _, c := range tt.isNot {
			if errors.Is(err, c) {
				t.Errorf("%s: errors.Is(err, %q) is true", tt.state, c)
			}
		}
	}
}

// refusal is the contract of usql for a refused credential: a method on the error
// type of a driver. It walks the whole tree, as usql does, because errors.As stops at
// the first error that has the method, even when that one answers false.
type refusal interface{ AuthenticationRefused() bool }

func refused(err error) bool {
	for err != nil {
		if e, ok := err.(refusal); ok && e.AuthenticationRefused() {
			return true
		}
		switch u := err.(type) { //nolint:errorlint // same reason
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

func TestAuthenticationRefused(t *testing.T) {
	t.Parallel()
	pg := `connection to server at "127.0.0.1", port 5432 failed: FATAL:  password authentication failed for user "x"`
	for name, tt := range map[string]struct {
		err  *Error
		want bool
	}{
		"MariaDB wrong password":      {&Error{State: "28000", Native: 1045, Message: "Access denied for user 'root'@'h' (using password: YES)"}, true},
		"SQL Server login failed":     {&Error{State: "28000", Native: 18456, Message: "Login failed for user 'sa'."}, true},
		"PostgreSQL 28P01":            {&Error{State: "28P01", Message: "password authentication failed"}, true},
		"psqlODBC wrong password":     {&Error{State: "08001", Native: 101, Message: pg}, true},
		"psqlODBC no password":        {&Error{State: "08001", Native: 101, Message: `fe_sendauth: no password supplied`}, true},
		"first record is information": {&Error{State: "01000", Message: "changed", Next: []Error{{State: "28000", Native: 18456}}}, true},
		"locked account":              {&Error{State: "28000", Native: 18486}, false},
		"expired password":            {&Error{State: "28000", Native: 18487}, false},
		"must change password":        {&Error{State: "28000", Native: 18488}, false},
		"MySQL locked account":        {&Error{State: "28000", Native: 3118}, false},
		"MySQL expired password":      {&Error{State: "28000", Native: 1862}, false},
		"PostgreSQL no privilege":     {&Error{State: "42501", Message: "permission denied for table t"}, false},
		"MySQL no privilege":          {&Error{State: "42000", Native: 1142}, false},
		"server rejected connection":  {&Error{State: "08004", Message: "Server rejected the connection"}, false},
		"network failure":             {&Error{State: "08001", Native: 2002, Message: "Can't connect to server on '127.0.0.1' (10061)"}, false},
		"psqlODBC refused connection": {&Error{State: "08001", Native: 101, Message: `connection to server at "127.0.0.1", port 1 failed: Connection refused`}, false},
		"login timeout":               {&Error{State: "HYT00", Message: "Login timeout expired"}, false},
		"unknown driver":              {&Error{State: "IM002", Message: "Data source name not found"}, false},
		"information only":            {&Error{State: "01000"}, false},
	} {
		if got := tt.err.AuthenticationRefused(); got != tt.want {
			t.Errorf("%s: AuthenticationRefused is %t, want %t", name, got, tt.want)
		}
		// a caller wraps the error with %w, or joins it with another
		if got := refused(fmt.Errorf("connecting: %w", tt.err)); got != tt.want {
			t.Errorf("%s wrapped: got %t, want %t", name, got, tt.want)
		}
		if got := refused(errors.Join(errors.New("other"), fmt.Errorf("x: %w", tt.err))); got != tt.want {
			t.Errorf("%s joined: got %t, want %t", name, got, tt.want)
		}
		// %v loses the method, as usql says
		if refused(fmt.Errorf("connecting: %v", tt.err)) { //nolint:errorlint // %v is the point of this check: it loses the method
			t.Errorf("%s wrapped with %%v is still found", name)
		}
	}
}

// TestAuthenticationRefusedWalksNext checks that a caller who uses errors.As, and
// so gets the first record only, still gets the answer that the later records
// give, because the method looks at every record of the call.
func TestAuthenticationRefusedWalksNext(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("connecting: %w", &Error{State: "01000", Next: []Error{{State: "01S02"}, {State: "28000", Native: 1045}}})
	var e *Error
	if !errors.As(err, &e) || !e.AuthenticationRefused() {
		t.Error("the first record does not say that a later record refused the credential")
	}
}

func TestErrVerbose(t *testing.T) {
	t.Parallel()
	for name, tt := range map[string]struct {
		err  *Error
		want string
	}{
		"one record": {
			&Error{Op: "executing", State: "42S02", Native: 1146, Message: "Table 'x' does not exist"},
			"ERROR:  42S02: Table 'x' does not exist\nDETAIL:  native error 1146\nCONTEXT:  in executing",
		},
		"no native code and no call": {
			&Error{State: "HY000", Message: "bad"},
			"ERROR:  HY000: bad",
		},
		"severity from the driver": {
			&Error{Op: "executing", State: "42P01", Native: 1, Message: `ERROR: relation "nope" does not exist`},
			"ERROR:  42P01: relation \"nope\" does not exist\nDETAIL:  native error 1\nCONTEXT:  in executing",
		},
		"further records": {
			&Error{
				Op: "connecting", State: "01000", Message: "note",
				Next: []Error{
					{Op: "connecting", State: "28000", Native: 18456, Message: "login failed"},
					{Op: "connecting", State: "08001", Message: "no route"},
				},
			},
			"ERROR:  01000: note\nDETAIL:  28000 (native 18456): login failed\nDETAIL:  08001: no route\nCONTEXT:  in connecting",
		},
	} {
		if got := tt.err.ErrVerbose("select 1"); got != tt.want {
			t.Errorf("%s: ErrVerbose is %q, want %q", name, got, tt.want)
		}
	}
}
