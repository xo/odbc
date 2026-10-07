package odbc

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"runtime"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/xo/dbimp"
)

// Option sets an option of one statement. An option comes from the context
// through WithOptions, then from an argument of the statement, and a later one
// wins. Every dbimp driver takes options in the same way (D22).
//
//	db.QueryContext(ctx, "SELECT ...", odbc.WithTimeout(5*time.Second), id)
type Option = dbimp.Option[options]

// options are the options of one statement.
type options struct {
	timeout  time.Duration
	readonly bool
	database string
	maxRows  int64
	noScan   bool
	// fetchSize is the number of rows for each fetch, and 0 or 1 reads one at a time.
	fetchSize int
	params    map[string]any
}

// Attributes that an option sets.
const (
	attrQueryTimeout   = 0
	attrCurrentCatalog = 109
	stateOptionChanged = "01S02"
)

// stmtAttrs are the statement attributes that WithParameter sets by name. The
// value of each is a number.
var stmtAttrs = map[string]int32{
	"query_timeout": 0,
	"max_rows":      1,
	"no_scan":       2,
	"max_length":    3,
	"cursor_type":   6,
	"concurrency":   7,
	"keyset_size":   8,
}

// WithTimeout sets how long the database gives the statement. The driver
// rounds a part of a second up, because ODBC counts whole seconds. A database
// driver that ignores the setting makes the statement fail with
// dbimp.ErrNotSupported.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithReadonly asks the database to refuse a statement that writes. ODBC has no
// setting for it that every database driver enforces. So WithReadonly(true)
// always fails with dbimp.ErrNotSupported, and WithReadonly(false) asks for
// nothing. A database that needs it can use a read only account.
func WithReadonly(v bool) Option {
	return func(o *options) { o.readonly = v }
}

// WithDatabase sets the catalog of one statement. A statement of a transaction
// runs in the catalog of the transaction, and another catalog fails with
// dbimp.ErrNotSupported.
func WithDatabase(name string) Option {
	return func(o *options) { o.database = name }
}

// WithMaxRows limits the number of rows that the statement returns. A negative
// value is not valid, and zero asks for no limit (D24).
func WithMaxRows(n int64) Option {
	return func(o *options) { o.maxRows = n }
}

// WithNoScan turns off the escape sequences of ODBC, such as {fn now()}, so
// that the database gets the text of the statement as it is (D24).
func WithNoScan(v bool) Option {
	return func(o *options) { o.noScan = v }
}

// WithFetchSize reads the rows of the result n at a time, in a block that the
// database driver fills with one call. It is faster for a large result. It is
// a hint about speed. A result with a column that cannot be bound, such as a
// long text, is read one row at a time all the same. A value that does not fit
// the buffer of its column fails the read and never comes back cut short (D24).
func WithFetchSize(n int) Option {
	return func(o *options) { o.fetchSize = n }
}

// WithParameter sets a statement attribute by its name. The names are
// query_timeout, max_rows, no_scan, max_length, cursor_type, concurrency and
// keyset_size, and the value is an integer or a bool. A parameter replaces what
// another option sets for the same attribute.
func WithParameter(name string, value any) Option {
	return func(o *options) {
		if o.params == nil {
			o.params = map[string]any{}
		}
		o.params[name] = value
	}
}

// WithOptions returns a context that carries opts. Each statement that starts
// with the context applies them.
func WithOptions(ctx context.Context, opts ...Option) context.Context {
	return dbimp.WithOptions(ctx, opts...)
}

// check returns an error for an option whose value is not valid.
func (o options) check() error {
	switch {
	case o.timeout < 0:
		return fmt.Errorf("applying the option WithTimeout: %v: %w", o.timeout, dbimp.ErrInvalidValue)
	case o.maxRows < 0:
		return fmt.Errorf("applying the option WithMaxRows: %d: %w", o.maxRows, dbimp.ErrInvalidValue)
	case o.fetchSize < 0 || o.fetchSize > maxFetchSize:
		return fmt.Errorf("applying the option WithFetchSize: %d: %w", o.fetchSize, dbimp.ErrInvalidValue)
	}
	for name, v := range o.params {
		if _, ok := stmtAttrs[name]; !ok {
			return fmt.Errorf("applying the option WithParameter: the name %q is unknown: %w", name, dbimp.ErrInvalidValue)
		}
		if _, err := number(v); err != nil {
			return fmt.Errorf("applying the option WithParameter: %s: %w", name, err)
		}
	}
	return nil
}

// number returns an integer or a bool as the value of an attribute.
func number(v any) (uintptr, error) {
	switch v := v.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case int:
		return uintptr(v), nil
	case int32:
		return uintptr(v), nil
	case int64:
		return uintptr(v), nil
	case uint:
		return uintptr(v), nil
	case uint32:
		return uintptr(v), nil
	case uint64:
		return uintptr(v), nil
	}
	return 0, fmt.Errorf("the value %v of type %T is not an integer or a bool: %w", v, v, dbimp.ErrInvalidValue)
}

// apply sets the options on the statement and the connection. It returns a
// function that puts the connection back as it was, and that the caller runs
// when the statement has executed.
func (s *stmt) apply(o options) (undo func(), err error) {
	var undos []func()
	undo = func() {
		for _, u := range slices.Backward(undos) {
			u()
		}
	}
	defer func() {
		if err != nil {
			undo()
			undo = func() {}
		}
	}()
	c := s.c
	if o.database != "" {
		restore, err := c.setCatalog(o.database)
		if err != nil {
			return undo, err
		}
		if restore != nil {
			undos = append(undos, restore)
		}
	}
	if o.readonly {
		// The access mode of ODBC is a hint. PostgreSQL, SQL Server and DuckDB
		// accept it and still write, which TestReadonlyIsEnforced shows, so the
		// driver cannot promise a read only statement (D22).
		return undo, fmt.Errorf("%w: ODBC has no read only setting that every database driver enforces", dbimp.Unsupported("WithReadonly"))
	}
	if o.timeout > 0 {
		secs := max(1, int(math.Ceil(o.timeout.Seconds())))
		if err := s.setStmtNumber("WithTimeout", attrQueryTimeout, uintptr(secs)); err != nil {
			return undo, err
		}
	}
	if o.maxRows > 0 {
		if err := s.setStmtNumber("WithMaxRows", stmtAttrs["max_rows"], uintptr(o.maxRows)); err != nil {
			return undo, err
		}
	}
	if o.noScan {
		if err := s.setStmtNumber("WithNoScan", stmtAttrs["no_scan"], 1); err != nil {
			return undo, err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(o.params)) {
		v, _ := number(o.params[name])
		if err := s.setStmtNumber("WithParameter "+name, stmtAttrs[name], v); err != nil {
			return undo, err
		}
	}
	return undo, nil
}

// setStmtNumber sets a numeric attribute of the statement.
func (s *stmt) setStmtNumber(option string, attr int32, v uintptr) error {
	a := s.c.api
	return s.c.honored(option, a.setStmtAttr(s.h, attr, v, 0), handleStmt, s.h)
}

// honored turns the return code of a call that sets an option into an error
// when the database driver did not honor it. A driver that changes the value
// answers with the state 01S02, and a caller must never believe that a limit
// holds when it does not (D22).
func (c *conn) honored(option string, ret int16, handleType int16, handle uintptr) error {
	a := c.api
	switch ret {
	case sqlSuccess:
		return nil
	case sqlSuccessWithInfo:
		var e *Error
		if err := a.diag("applying the option "+option, ret, handleType, handle); errors.As(err, &e) {
			for _, r := range append([]Error{*e}, e.Next...) {
				if r.State == stateOptionChanged {
					return fmt.Errorf("%w: %w", dbimp.Unsupported(option), err)
				}
			}
		}
		return nil
	}
	err := a.diag("applying the option "+option, ret, handleType, handle)
	return fmt.Errorf("%w: %w", dbimp.Unsupported(option), err)
}

// catalog returns the current catalog of the connection.
func (c *conn) catalog() (string, error) {
	a := c.api
	buf := make([]byte, 256*a.wchar)
	var n int32
	ret := a.getConnAttr(c.dbc, attrCurrentCatalog, unsafe.Pointer(&buf[0]), int32(len(buf)), &n)
	if err := a.check("reading the catalog", ret, handleDbc, c.dbc); err != nil {
		return "", err
	}
	return a.decode(buf), nil
}

// setCatalog sets the catalog of the connection. It returns a function that
// sets the old one again, or nil when the catalog is already the one named.
func (c *conn) setCatalog(name string) (restore func(), err error) {
	a := c.api
	old, err := c.catalog()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", dbimp.Unsupported("WithDatabase"), err)
	}
	if old == name {
		return nil, nil
	}
	if c.inTx {
		return nil, fmt.Errorf("%w: the transaction runs in %q", dbimp.Unsupported("WithDatabase"), old)
	}
	set := func(s string) int16 {
		enc := a.encode(s)
		ret := a.setConnAttrPtr(c.dbc, attrCurrentCatalog, unsafe.Pointer(&enc[0]), nts)
		runtime.KeepAlive(enc)
		return ret
	}
	if err := c.honored("WithDatabase", set(name), handleDbc, c.dbc); err != nil {
		return nil, err
	}
	// Some drivers accept the attribute and stay where they were, so read it
	// back.
	now, err := c.catalog()
	if err != nil || !strings.EqualFold(now, name) {
		_ = set(old)
		return nil, fmt.Errorf("%w: the catalog is %q after the change", dbimp.Unsupported("WithDatabase"), now)
	}
	return func() { _ = set(old) }, nil
}
