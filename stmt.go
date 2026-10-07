package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

// stmt is a statement handle. A prepared one lives until Close. One made for a
// single call lives until the call, or its rows, end.
type stmt struct {
	c        *conn
	h        uintptr
	text     string
	prepared bool
	freed    bool
}

var (
	_ driver.Stmt              = (*stmt)(nil)
	_ driver.StmtExecContext   = (*stmt)(nil)
	_ driver.StmtQueryContext  = (*stmt)(nil)
	_ driver.NamedValueChecker = (*stmt)(nil)
)

// NumInput implements driver.Stmt. It is -1, so that database/sql does not
// count the placeholders.
func (*stmt) NumInput() int { return -1 }

// CheckNamedValue implements driver.NamedValueChecker.
func (*stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if nv.Name != "" {
		return errors.New("checking the arguments: ODBC parameters are positional, not named")
	}
	return driver.ErrSkip
}

// Close implements driver.Stmt.
func (s *stmt) Close() error { return s.free() }

func (s *stmt) free() error {
	if s.freed {
		return nil
	}
	s.freed = true
	return s.c.api.check("freeing the statement", s.c.api.freeHandle(handleStmt, s.h), handleStmt, s.h)
}

// Exec implements driver.Stmt. database/sql calls ExecContext instead.
func (*stmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("executing: use ExecContext")
}

// Query implements driver.Stmt. database/sql calls QueryContext instead.
func (*stmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, errors.New("querying: use QueryContext")
}

// ExecContext implements driver.StmtExecContext.
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.exec(ctx, args)
}

// QueryContext implements driver.StmtQueryContext.
func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.query(ctx, args)
}

// result is the outcome of an Exec.
type result struct{ n int64 }

func (result) LastInsertId() (int64, error) {
	return 0, errors.New("reading the last insert id: ODBC does not report one")
}

func (r result) RowsAffected() (int64, error) { return r.n, nil }

// run binds the arguments and executes the statement.
func (s *stmt) run(ctx context.Context, args []driver.NamedValue) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a := s.c.api
	ps, err := s.bind(args)
	if err != nil {
		return err
	}
	stop := s.c.watch(ctx, s.h)
	var ret int16
	if s.prepared {
		ret = a.execute(s.h)
	} else {
		text := a.encode(s.text)
		ret = a.execDirect(s.h, unsafe.Pointer(&text[0]), nts)
	}
	stop()
	// The driver manager can hold the addresses of the parameters until the
	// statement is executed or reset.
	runtime.KeepAlive(ps)
	op := "executing"
	if ret == sqlNoData {
		// an update or delete that matched no row
		ret = sqlSuccess
	}
	if err := a.check(op, ret, handleStmt, s.h); err != nil {
		_ = a.freeStmt(s.h, resetParams)
		return wrapCtx(ctx, err)
	}
	return nil
}

func (s *stmt) exec(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := s.run(ctx, args); err != nil {
		return nil, err
	}
	a := s.c.api
	defer func() {
		_ = a.freeStmt(s.h, closeCursor)
		_ = a.freeStmt(s.h, resetParams)
	}()
	var n int
	if err := a.check("counting the rows affected", a.rowCount(s.h, &n), handleStmt, s.h); err != nil {
		return nil, err
	}
	return result{n: int64(max(n, 0))}, nil
}

func (s *stmt) query(ctx context.Context, args []driver.NamedValue) (*rows, error) {
	if err := s.run(ctx, args); err != nil {
		return nil, err
	}
	r := &rows{ctx: ctx, s: s}
	if err := r.describe(); err != nil {
		_ = s.c.api.freeStmt(s.h, closeCursor)
		return nil, err
	}
	// the parameters are not needed once the statement has run
	_ = s.c.api.freeStmt(s.h, resetParams)
	return r, nil
}

// param holds the memory of one bound parameter.
type param struct {
	buf []byte
	ts  timestamp
	ind int
}

// timestamp is SQL_TIMESTAMP_STRUCT.
type timestamp struct {
	Year     int16
	Month    uint16
	Day      uint16
	Hour     uint16
	Minute   uint16
	Second   uint16
	Fraction uint32
}

// Types of ODBC that a parameter or a column uses.
const (
	cChar      = 1
	cDouble    = 8
	cBit       = -7
	cBinary    = -2
	cWChar     = -8
	cSBigint   = -25
	cUBigint   = -27
	cTimestamp = 93

	sqlVarchar       = 12
	sqlDouble        = 8
	sqlBit           = -7
	sqlVarbinary     = -3
	sqlLongVarbinary = -4
	sqlBigint        = -5
	sqlTinyint       = -6
	dbmsDuckDB       = "DuckDB"
	sqlWVarchar      = -9
	sqlWLongVarchar  = -10
	sqlTimestamp     = 93

	inputParam = 1
	// maxBounded is the longest value passed as a bounded type.
	maxBounded = 4000
)

// bind binds each argument to a parameter marker. The memory stays reachable
// through the returned slice.
func (s *stmt) bind(args []driver.NamedValue) ([]*param, error) {
	a := s.c.api
	ps := make([]*param, len(args))
	for i, arg := range args {
		p := new(param)
		ps[i] = p
		var (
			ctype, stype int16
			size         uintptr
			digits       int16
			ptr          unsafe.Pointer
			length       int
		)
		switch v := arg.Value.(type) {
		case nil:
			p.ind = nullData
			ctype, stype, size = cChar, sqlVarchar, 1
			p.buf = make([]byte, 1)
			ptr, length = unsafe.Pointer(&p.buf[0]), 1
		case int64:
			p.buf = make([]byte, 8)
			*(*int64)(unsafe.Pointer(&p.buf[0])) = v
			ctype, stype, size = cSBigint, sqlBigint, 19
			ptr, length, p.ind = unsafe.Pointer(&p.buf[0]), 8, 8
		case float64:
			p.buf = make([]byte, 8)
			*(*float64)(unsafe.Pointer(&p.buf[0])) = v
			ctype, stype, size = cDouble, sqlDouble, 15
			ptr, length, p.ind = unsafe.Pointer(&p.buf[0]), 8, 8
		case bool:
			p.buf = make([]byte, 1)
			if v {
				p.buf[0] = 1
			}
			ctype, stype, size = cBit, sqlBit, 1
			if s.c.dbms == dbmsDuckDB {
				// the DuckDB driver refuses SQL_BIT as the type of a parameter
				stype = sqlTinyint
			}
			ptr, length, p.ind = unsafe.Pointer(&p.buf[0]), 1, 1
		case string:
			enc := a.encode(v)
			p.buf = enc
			units := a.units(len(enc)) - 1
			ctype, stype = cWChar, sqlWVarchar
			size = uintptr(max(units, 1))
			if units > maxBounded {
				stype = sqlWLongVarchar
			}
			ptr, length, p.ind = unsafe.Pointer(&p.buf[0]), len(p.buf), len(enc)-a.wchar
		case []byte:
			p.buf = make([]byte, max(len(v), 1))
			copy(p.buf, v)
			ctype, stype = cBinary, sqlVarbinary
			size = uintptr(max(len(v), 1))
			if len(v) > maxBounded {
				stype = sqlLongVarbinary
			}
			ptr, length, p.ind = unsafe.Pointer(&p.buf[0]), len(p.buf), len(v)
		case time.Time:
			p.ts = timestamp{
				Year: int16(v.Year()), Month: uint16(v.Month()), Day: uint16(v.Day()),
				Hour: uint16(v.Hour()), Minute: uint16(v.Minute()), Second: uint16(v.Second()),
				// microseconds, because a database rejects digits it cannot hold
				Fraction: uint32(v.Nanosecond()/1000) * 1000,
			}
			ctype, stype, size, digits = cTimestamp, sqlTimestamp, 26, 6
			ptr, length, p.ind = unsafe.Pointer(&p.ts), int(unsafe.Sizeof(p.ts)), int(unsafe.Sizeof(p.ts))
		default:
			return nil, fmt.Errorf("binding argument %d: the type %T is not supported", arg.Ordinal, arg.Value)
		}
		ret := a.bindParameter(s.h, uint16(arg.Ordinal), inputParam, ctype, stype, size, digits, ptr, length, &p.ind)
		if err := a.check(fmt.Sprintf("binding argument %d", arg.Ordinal), ret, handleStmt, s.h); err != nil {
			_ = a.freeStmt(s.h, resetParams)
			return nil, err
		}
	}
	return ps, nil
}
