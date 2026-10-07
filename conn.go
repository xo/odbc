package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/xo/dbimp"
)

// conn is one connection. database/sql uses it from one goroutine at a time,
// except that a context can call cancel on a statement from another.
type conn struct {
	api *api
	dbc uintptr
	// dbms is the name the database gives itself, such as DuckDB. It selects
	// the few quirks of a driver that the ODBC types cannot express.
	dbms      string
	onWarning func(*Error)
	loc       *time.Location
	traced    bool
	inTx      bool
	closed    bool
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
	return checkNamedValue(nv)
}

// checkNamedValue keeps an Option, which Resolve takes out of the arguments of
// the statement, and refuses a named argument.
func checkNamedValue(nv *driver.NamedValue) error {
	if dbimp.IsOption[options](nv.Value) {
		return nil
	}
	if nv.Name != "" {
		return fmt.Errorf("checking the arguments: ODBC parameters are positional, not named: %w", dbimp.ErrNotSupported)
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
	if c.traced {
		// The driver manager of Windows keeps the trace file open until the trace is
		// off, and a caller cannot remove the file before then.
		_ = c.api.setConnAttr(c.dbc, attrTrace, traceOff, 0)
	}
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

// warn passes the diagnostics of a call that succeeded with information to the
// function of the Config, when it has one (D23).
func (c *conn) warn(op string, ret int16, handleType int16, handle uintptr) {
	if ret != sqlSuccessWithInfo || c.onWarning == nil {
		return
	}
	var e *Error
	if err := c.api.diag(op, ret, handleType, handle); errors.As(err, &e) {
		c.onWarning(e)
	}
}

// InfoType names a kind of information that SQLGetInfo gives.
type InfoType uint16

// Some of the information types. The ODBC reference lists the others.
const (
	InfoDataSourceName      InfoType = 2
	InfoDriverName          InfoType = 6
	InfoDriverVersion       InfoType = 7
	InfoDBMSName            InfoType = 17
	InfoDBMSVersion         InfoType = 18
	InfoIdentifierQuoteChar InfoType = 29
	InfoDriverODBCVersion   InfoType = 77
)

// Conn is the connection of the driver, which a program reaches through
// sql.Conn.Raw to ask the database about itself (D23).
//
//	conn.Raw(func(dc any) error {
//		name, err := dc.(odbc.Conn).GetInfoString(odbc.InfoDBMSName)
//		...
//	})
//
// The type of the answer depends on the information type, and the ODBC
// reference says which. A text type reads with GetInfoString, a 16 bit number
// with GetInfoUint16 and a 32 bit number with GetInfoUint32.
//
// Tables, Columns and PrimaryKeys wrap the catalog functions of ODBC, which read
// the metadata of any database in one way.
type Conn interface {
	GetInfoString(info InfoType) (string, error)
	GetInfoUint16(info InfoType) (uint16, error)
	GetInfoUint32(info InfoType) (uint32, error)
	Tables(ctx context.Context, catalog, schema, table, tableTypes string) ([]TableInfo, error)
	Columns(ctx context.Context, catalog, schema, table, column string) ([]ColumnInfo, error)
	PrimaryKeys(ctx context.Context, catalog, schema, table string) ([]PrimaryKeyInfo, error)
}

var _ Conn = (*conn)(nil)

// GetInfoString reads a text value with SQLGetInfo.
func (c *conn) GetInfoString(info InfoType) (string, error) {
	if c.closed {
		return "", driver.ErrBadConn
	}
	buf := make([]byte, 512*c.api.wchar)
	var n int16
	// the length of a string that SQLGetInfoW takes is in bytes
	ret := c.api.getInfo(c.dbc, uint16(info), unsafe.Pointer(&buf[0]), int16(len(buf)), &n)
	if err := c.api.check("reading information", ret, handleDbc, c.dbc); err != nil {
		return "", err
	}
	return c.api.decode(buf[:min(int(n), len(buf))]), nil
}

// GetInfoUint16 reads a 16 bit number with SQLGetInfo.
func (c *conn) GetInfoUint16(info InfoType) (uint16, error) {
	var v uint16
	err := c.getInfoNumber(info, unsafe.Pointer(&v))
	return v, err
}

// GetInfoUint32 reads a 32 bit number with SQLGetInfo.
func (c *conn) GetInfoUint32(info InfoType) (uint32, error) {
	var v uint32
	err := c.getInfoNumber(info, unsafe.Pointer(&v))
	return v, err
}

func (c *conn) getInfoNumber(info InfoType, p unsafe.Pointer) error {
	if c.closed {
		return driver.ErrBadConn
	}
	var n int16
	ret := c.api.getInfo(c.dbc, uint16(info), p, 0, &n)
	return c.api.check("reading information", ret, handleDbc, c.dbc)
}
