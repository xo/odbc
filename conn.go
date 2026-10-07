package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"unsafe"
)

// conn is one connection. database/sql uses it from one goroutine at a time,
// except that a context can call cancel on a statement from another.
type conn struct {
	api *api
	dbc uintptr
	// dbms is the name the database gives itself, such as DuckDB. It selects
	// the few quirks of a driver that the ODBC types cannot express.
	dbms   string
	closed bool
}

var (
	_ driver.Conn               = (*conn)(nil)
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.Pinger             = (*conn)(nil)
	_ driver.Validator          = (*conn)(nil)
	_ driver.SessionResetter    = (*conn)(nil)
	_ driver.NamedValueChecker  = (*conn)(nil)
)

// dead reports whether the driver says the connection is gone. A driver that
// does not know says it is alive.
func (c *conn) dead() bool {
	var v uint32
	var n int32
	if c.api.getConnAttr(c.dbc, attrConnDead, unsafe.Pointer(&v), 0, &n) != sqlSuccess {
		return false
	}
	return v == connDeadTrue
}

// IsValid implements driver.Validator.
func (c *conn) IsValid() bool { return !c.closed && !c.dead() }

// ResetSession implements driver.SessionResetter. The statement has not been
// sent, so a dead connection returns driver.ErrBadConn.
func (c *conn) ResetSession(context.Context) error {
	if !c.IsValid() {
		return driver.ErrBadConn
	}
	return nil
}

// Ping implements driver.Pinger.
func (c *conn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.IsValid() {
		return driver.ErrBadConn
	}
	return nil
}

// CheckNamedValue implements driver.NamedValueChecker. ODBC parameters are
// positional.
func (*conn) CheckNamedValue(nv *driver.NamedValue) error {
	if nv.Name != "" {
		return errors.New("checking the arguments: ODBC parameters are positional, not named")
	}
	return driver.ErrSkip
}

// Close frees the connection.
func (c *conn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	err := c.api.check("disconnecting", c.api.disconnect(c.dbc), handleDbc, c.dbc)
	if e := c.api.check("freeing the connection", c.api.freeHandle(handleDbc, c.dbc), handleDbc, c.dbc); err == nil {
		err = e
	}
	return err
}

// newStmt allocates a statement handle.
func (c *conn) newStmt(text string) (*stmt, error) {
	if c.closed {
		return nil, driver.ErrBadConn
	}
	var h uintptr
	if err := c.api.check("allocating the statement", c.api.allocHandle(handleStmt, c.dbc, &h), handleDbc, c.dbc); err != nil {
		return nil, err
	}
	return &stmt{c: c, h: h, text: text}, nil
}

// Prepare implements driver.Conn.
func (c *conn) Prepare(query string) (driver.Stmt, error) { return c.prepare(query) }

// PrepareContext implements driver.ConnPrepareContext.
func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.prepare(query)
}

func (c *conn) prepare(query string) (*stmt, error) {
	s, err := c.newStmt(query)
	if err != nil {
		return nil, err
	}
	text := c.api.encode(query)
	if err := c.api.check("preparing", c.api.prepare(s.h, unsafe.Pointer(&text[0]), nts), handleStmt, s.h); err != nil {
		_ = s.free()
		return nil, err
	}
	s.prepared = true
	return s, nil
}

// ExecContext implements driver.ExecerContext.
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	s, err := c.newStmt(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.free() }()
	return s.exec(ctx, args)
}

// QueryContext implements driver.QueryerContext.
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s, err := c.newStmt(query)
	if err != nil {
		return nil, err
	}
	r, err := s.query(ctx, args)
	if err != nil {
		_ = s.free()
		return nil, err
	}
	r.own = true
	return r, nil
}

// watch cancels the statement when the context ends. The function it returns
// stops the watch and waits for a cancel call that has started, so the
// statement is never freed under it.
func (c *conn) watch(ctx context.Context, h uintptr) (stop func()) {
	if ctx.Done() == nil {
		return func() {}
	}
	done := make(chan struct{})
	cancel := context.AfterFunc(ctx, func() {
		defer close(done)
		_ = c.api.cancel(h)
	})
	return func() {
		if !cancel() {
			<-done
		}
	}
}

// wrapCtx returns the error of the context when it ended the call.
func wrapCtx(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil {
		return fmt.Errorf("%w: %w", ctx.Err(), err)
	}
	return err
}
