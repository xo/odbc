package odbc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"unsafe"

	"github.com/xo/dbimp"
)

// Transaction isolation levels of ODBC.
const (
	isoReadUncommitted = 1
	isoReadCommitted   = 2
	isoRepeatableRead  = 4
	isoSerializable    = 8
)

// tx is a transaction on a connection.
type tx struct {
	c *conn
	// iso is the isolation level to restore, or zero to leave it.
	iso uint32
}

var _ driver.Tx = (*tx)(nil)

// BeginTx implements driver.ConnBeginTx.
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.begin(opts)
}

// Begin implements driver.Conn.
func (c *conn) Begin() (driver.Tx, error) { return c.begin(driver.TxOptions{}) }

func (c *conn) begin(opts driver.TxOptions) (driver.Tx, error) {
	if c.closed {
		return nil, driver.ErrBadConn
	}
	if opts.ReadOnly {
		return nil, fmt.Errorf("beginning a transaction: ODBC has no read only transaction that every database driver enforces: %w", dbimp.ErrNotSupported)
	}
	t := &tx{c: c}
	level, err := isolationLevel(sql.IsolationLevel(opts.Isolation))
	if err != nil {
		return nil, err
	}
	if level != 0 {
		var old uint32
		var n int32
		if c.api.getConnAttr(c.dbc, attrTxnIsolation, unsafe.Pointer(&old), 0, &n) == sqlSuccess {
			t.iso = old
		}
		ret := c.api.setConnAttr(c.dbc, attrTxnIsolation, uintptr(level), 0)
		if err := c.api.check("setting the isolation level", ret, handleDbc, c.dbc); err != nil {
			return nil, err
		}
	}
	ret := c.api.setConnAttr(c.dbc, attrAutocommit, autocommitOff, 0)
	if err := c.api.check("beginning a transaction", ret, handleDbc, c.dbc); err != nil {
		return nil, err
	}
	return t, nil
}

func isolationLevel(l sql.IsolationLevel) (uint32, error) {
	switch l {
	case sql.LevelDefault:
		return 0, nil
	case sql.LevelReadUncommitted:
		return isoReadUncommitted, nil
	case sql.LevelReadCommitted:
		return isoReadCommitted, nil
	case sql.LevelRepeatableRead:
		return isoRepeatableRead, nil
	case sql.LevelSerializable:
		return isoSerializable, nil
	default:
		return 0, fmt.Errorf("beginning a transaction: the isolation level %s: %w", l, dbimp.ErrNotSupported)
	}
}

// Commit implements driver.Tx.
func (t *tx) Commit() error { return t.end("committing", commit) }

// Rollback implements driver.Tx.
func (t *tx) Rollback() error { return t.end("rolling back", rollback) }

func (t *tx) end(op string, how int16) error {
	a, c := t.c.api, t.c
	err := a.check(op, a.endTran(handleDbc, c.dbc, how), handleDbc, c.dbc)
	// the connection goes back to autocommit whether or not the end worked
	if e := a.check("ending the transaction", a.setConnAttr(c.dbc, attrAutocommit, autocommitOn, 0), handleDbc, c.dbc); err == nil {
		err = e
	}
	if t.iso != 0 {
		_ = a.setConnAttr(c.dbc, attrTxnIsolation, uintptr(t.iso), 0)
	}
	return err
}
