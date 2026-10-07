package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/xo/dbimp"
)

func TestOptionsCheck(t *testing.T) {
	t.Parallel()
	for name, tt := range map[string]struct {
		opts options
		want error
	}{
		"empty":            {options{}, nil},
		"timeout":          {options{timeout: time.Second}, nil},
		"negative timeout": {options{timeout: -time.Second}, dbimp.ErrInvalidValue},
		"known parameter":  {options{params: map[string]any{"max_rows": 10}}, nil},
		"unknown":          {options{params: map[string]any{"nothing": 1}}, dbimp.ErrInvalidValue},
		"text value":       {options{params: map[string]any{"max_rows": "ten"}}, dbimp.ErrInvalidValue},
		"bool value":       {options{params: map[string]any{"no_scan": true}}, nil},
	} {
		if err := tt.opts.check(); !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", name, err, tt.want)
		}
	}
}

// TestResolveTakesOptionsOutOfTheArguments checks that an option argument is
// not a parameter of the statement, and that a later option wins.
func TestResolveTakesOptionsOutOfTheArguments(t *testing.T) {
	t.Parallel()
	ctx := WithOptions(context.Background(), WithTimeout(time.Second), WithDatabase("a"))
	args := []driver.NamedValue{
		{Ordinal: 1, Value: WithTimeout(3 * time.Second)},
		{Ordinal: 2, Value: int64(7)},
		{Ordinal: 3, Value: WithDatabase("b")},
		{Ordinal: 4, Value: "x"},
	}
	o, rest := dbimp.Resolve(ctx, options{}, args)
	if o.timeout != 3*time.Second || o.database != "b" {
		t.Errorf("got timeout %v and database %q, want 3s and b", o.timeout, o.database)
	}
	if len(rest) != 2 || rest[0].Ordinal != 1 || rest[0].Value != int64(7) || rest[1].Ordinal != 2 || rest[1].Value != "x" {
		t.Errorf("the other arguments are %+v", rest)
	}
}

func TestCheckNamedValueKeepsOptions(t *testing.T) {
	t.Parallel()
	opt := driver.NamedValue{Ordinal: 1, Value: WithTimeout(time.Second)}
	if err := checkNamedValue(&opt); err != nil {
		t.Errorf("an option: %v", err)
	}
	named := driver.NamedValue{Name: "id", Ordinal: 1, Value: int64(1)}
	if err := checkNamedValue(&named); !errors.Is(err, dbimp.ErrNotSupported) {
		t.Errorf("a named argument: got %v, want dbimp.ErrNotSupported", err)
	}
	plain := driver.NamedValue{Ordinal: 1, Value: int64(1)}
	if err := checkNamedValue(&plain); !errors.Is(err, driver.ErrSkip) {
		t.Errorf("a plain argument: got %v, want driver.ErrSkip", err)
	}
}
