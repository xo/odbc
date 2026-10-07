package odbc

import (
	"errors"
	"fmt"
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
