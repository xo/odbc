package odbc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/cockroachdb/apd/v3"
	"github.com/xo/dbimp"
)

// column describes one column of a result set.
type column struct {
	name     string
	sqlType  int16
	typeName string
	size     uintptr
	digits   int16
	nullable int16
	unsigned bool
	// boolText is true for a boolean that the driver reports as text, which
	// DuckDB does, and psqlODBC does unless BoolsAsChar is 0.
	boolText bool
}

// rows is a result set. It reads one row at a time.
type rows struct {
	ctx   context.Context //nolint:containedctx // the context of the query bounds every fetch
	s     *stmt
	cols  []column
	plans []plan
	row   []driver.Value
	// blk is set when the rows are fetched in blocks (D24).
	blk    *block
	own    bool
	closed bool
	// undo puts the connection back as the options of the statement changed it.
	undo func()
}

var (
	_ driver.Rows                           = (*rows)(nil)
	_ driver.RowsNextResultSet              = (*rows)(nil)
	_ driver.RowsColumnScanner              = (*rows)(nil)
	_ driver.RowsColumnTypeDatabaseTypeName = (*rows)(nil)
	_ driver.RowsColumnTypeNullable         = (*rows)(nil)
	_ driver.RowsColumnTypeLength           = (*rows)(nil)
	_ driver.RowsColumnTypePrecisionScale   = (*rows)(nil)
	_ driver.RowsColumnTypeScanType         = (*rows)(nil)
)

// SQL types of ODBC that the reader treats on their own.
const (
	tChar          = 1
	tNumeric       = 2
	tDecimal       = 3
	tInteger       = 4
	tSmallint      = 5
	tFloat         = 6
	tReal          = 7
	tDouble        = 8
	tDate          = 9
	tTime          = 10
	tVarchar       = 12
	tTypeDate      = 91
	tTypeTime      = 92
	tTypeTimestamp = 93
	tGUID          = -11
	tSSTime        = -154
	tSSOffset      = -155
	tLongVarchar   = -1
	tBinary        = -2
	tVarbinary     = -3
	tLongVarbinary = -4
	tBigint        = -5
	tTinyint       = -6
	tBit           = -7
	tWChar         = -8
	tWVarchar      = -9
	tWLongVarchar  = -10

	nullableYes = 1
)

// describe reads the metadata of the current result set.
func (r *rows) describe() error {
	a, h := r.s.c.api, r.s.h
	var n int16
	if err := a.check("counting the columns", a.numResultCols(h, &n), handleStmt, h); err != nil {
		return err
	}
	r.cols = make([]column, n)
	for i := range r.cols {
		var (
			name     = make([]byte, 256*a.wchar)
			nameLen  int16
			dataType int16
			size     uintptr
			digits   int16
			nullable int16
		)
		ret := a.describeCol(h, uint16(i+1), unsafe.Pointer(&name[0]), int16(a.units(len(name))), &nameLen, &dataType, &size, &digits, &nullable)
		if err := a.check("describing a column", ret, handleStmt, h); err != nil {
			return err
		}
		c := column{
			name:     a.decode(name[:min(int(nameLen), a.units(len(name))-1)*a.wchar]),
			sqlType:  dataType,
			size:     size,
			digits:   digits,
			nullable: nullable,
		}
		tn := make([]byte, 128*a.wchar)
		var tnLen int16
		var num int
		// the type name is a nicety, so a driver that refuses it is not an error
		if a.colAttribute(h, uint16(i+1), descTypeName, unsafe.Pointer(&tn[0]), int16(len(tn)), &tnLen, &num) == sqlSuccess {
			c.typeName = a.decode(tn)
		}
		switch strings.ToLower(c.typeName) {
		case "bool", "boolean":
			c.boolText = true
		}
		if dataType == tBigint {
			// SQL_DESC_UNSIGNED is 1 for an unsigned type
			var u int
			if a.colAttribute(h, uint16(i+1), descUnsigned, nil, 0, nil, &u) == sqlSuccess {
				c.unsigned = u == 1
			}
		}
		r.cols[i] = c
	}
	r.plans = make([]plan, len(r.cols))
	for i, c := range r.cols {
		r.plans[i] = r.planFor(c)
	}
	return nil
}

// Columns implements driver.Rows.
func (r *rows) Columns() []string {
	out := make([]string, len(r.cols))
	for i, c := range r.cols {
		out[i] = c.name
	}
	return out
}

// Close implements driver.Rows.
func (r *rows) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	a := r.s.c.api
	defer r.undo()
	r.disableBlock()
	if r.own {
		return r.s.free()
	}
	return a.check("closing the cursor", a.freeStmt(r.s.h, closeCursor), handleStmt, r.s.h)
}

// Next implements driver.Rows, for versions of Go before database/sql used
// NextRow.
func (r *rows) Next(dest []driver.Value) error {
	if err := r.NextRow(); err != nil {
		return err
	}
	copy(dest, r.row)
	return nil
}

// NextRow implements driver.RowsColumnScanner. It fetches the next row and
// reads every column of it.
func (r *rows) NextRow() error {
	if r.closed {
		return errClosed
	}
	if len(r.cols) == 0 {
		// a statement that returns no result set, such as CREATE TABLE
		return io.EOF
	}
	if r.blk != nil {
		return r.nextBlockRow()
	}
	a, h := r.s.c.api, r.s.h
	stop := r.s.c.watch(r.ctx, h)
	ret := a.fetch(h)
	stop()
	if ret == sqlNoData {
		return io.EOF
	}
	if err := a.check("fetching", ret, handleStmt, h); err != nil {
		return wrapCtx(r.ctx, err)
	}
	r.s.c.warn("fetching", ret, handleStmt, h)
	if len(r.row) != len(r.cols) {
		r.row = make([]driver.Value, len(r.cols))
	}
	for i := range r.cols {
		v, err := r.read(i)
		if err != nil {
			return wrapCtx(r.ctx, err)
		}
		r.row[i] = v
	}
	return nil
}

// ScanColumn implements driver.RowsColumnScanner, so that a decimal and the
// civil types of dbimp reach the destination the caller names.
func (r *rows) ScanColumn(scanCtx driver.ScanContext, i int, dest any) error {
	return dbimp.Assign(scanCtx, dest, r.row[i])
}

// HasNextResultSet implements driver.RowsNextResultSet. ODBC cannot say
// without moving on, so it is always true, and NextResultSet answers io.EOF
// when there is none.
func (*rows) HasNextResultSet() bool { return true }

// NextResultSet implements driver.RowsNextResultSet.
func (r *rows) NextResultSet() error {
	a, h := r.s.c.api, r.s.h
	r.disableBlock()
	ret := a.moreResults(h)
	if ret == sqlNoData {
		return io.EOF
	}
	if err := a.check("moving to the next result", ret, handleStmt, h); err != nil {
		return err
	}
	return r.describe()
}

// read reads column i of the current row.
func (r *rows) read(i int) (driver.Value, error) {
	col := uint16(i + 1)
	p := r.plans[i]
	if p.size > 0 {
		raw := make([]byte, p.size)
		null, err := r.fixed(col, p.ctype, unsafe.Pointer(&raw[0]), p.size)
		if err != nil || null {
			return nil, err
		}
		return p.conv(raw)
	}
	raw, null, err := r.chunks(col, p.ctype, p.term)
	if err != nil || null {
		return nil, err
	}
	return p.conv(raw)
}

// fixed reads a value of a fixed size.
func (r *rows) fixed(col uint16, ctype int16, p unsafe.Pointer, size int) (null bool, err error) {
	a, h := r.s.c.api, r.s.h
	var ind int
	if err := a.check("reading a column", a.getData(h, col, ctype, p, size, &ind), handleStmt, h); err != nil {
		return false, err
	}
	return ind == nullData, nil
}

// chunks reads a value of any length. term is the size of the terminator that
// the driver adds to character data.
func (r *rows) chunks(col uint16, ctype int16, term int) (out []byte, null bool, err error) {
	a, h := r.s.c.api, r.s.h
	buf := make([]byte, 8192)
	for {
		var ind int
		ret := a.getData(h, col, ctype, unsafe.Pointer(&buf[0]), len(buf), &ind)
		if ret == sqlNoData {
			break
		}
		if err := a.check("reading a column", ret, handleStmt, h); err != nil {
			return nil, false, err
		}
		if ind == nullData {
			return nil, true, nil
		}
		room := len(buf) - term
		if ind == noTotal || ind > room {
			out = append(out, buf[:room]...)
			continue
		}
		out = append(out, buf[:ind]...)
		break
	}
	if out == nil {
		out = []byte{}
	}
	return out, false, nil
}

// ColumnTypeDatabaseTypeName implements driver.RowsColumnTypeDatabaseTypeName.
func (r *rows) ColumnTypeDatabaseTypeName(i int) string {
	if c := r.cols[i]; c.typeName != "" {
		return c.typeName
	}
	return sqlTypeName(r.cols[i].sqlType)
}

// ColumnTypeNullable implements driver.RowsColumnTypeNullable.
func (r *rows) ColumnTypeNullable(i int) (nullable, ok bool) {
	switch r.cols[i].nullable {
	case 0:
		return false, true
	case nullableYes:
		return true, true
	}
	return false, false
}

// ColumnTypeLength implements driver.RowsColumnTypeLength.
func (r *rows) ColumnTypeLength(i int) (length int64, ok bool) {
	c := r.cols[i]
	switch c.sqlType {
	case tChar, tVarchar, tLongVarchar, tWChar, tWVarchar, tWLongVarchar, tBinary, tVarbinary, tLongVarbinary:
		return int64(c.size), true
	}
	return 0, false
}

// ColumnTypePrecisionScale implements driver.RowsColumnTypePrecisionScale.
func (r *rows) ColumnTypePrecisionScale(i int) (precision, scale int64, ok bool) {
	c := r.cols[i]
	switch c.sqlType {
	case tNumeric, tDecimal:
		return int64(c.size), int64(c.digits), true
	}
	return 0, 0, false
}

// ColumnTypeScanType implements driver.RowsColumnTypeScanType.
func (r *rows) ColumnTypeScanType(i int) reflect.Type {
	c := r.cols[i]
	null := c.nullable == nullableYes
	switch c.sqlType {
	case tBigint:
		if c.unsigned {
			return reflect.TypeFor[uint64]()
		}
		fallthrough
	case tTinyint, tSmallint, tInteger:
		if null {
			return reflect.TypeFor[sql.NullInt64]()
		}
		return reflect.TypeFor[int64]()
	case tBit:
		if null {
			return reflect.TypeFor[sql.NullBool]()
		}
		return reflect.TypeFor[bool]()
	}
	if c.boolText {
		if null {
			return reflect.TypeFor[sql.NullBool]()
		}
		return reflect.TypeFor[bool]()
	}
	switch c.sqlType {
	case tReal, tFloat, tDouble:
		if null {
			return reflect.TypeFor[sql.NullFloat64]()
		}
		return reflect.TypeFor[float64]()
	case tDate, tTypeDate:
		return reflect.TypeFor[dbimp.Date]()
	case tTypeTimestamp:
		return reflect.TypeFor[dbimp.LocalDateTime]()
	case tTime, tTypeTime, tSSTime:
		return reflect.TypeFor[dbimp.LocalTime]()
	case tSSOffset:
		if null {
			return reflect.TypeFor[sql.NullTime]()
		}
		return reflect.TypeFor[time.Time]()
	case tDecimal, tNumeric:
		return reflect.TypeFor[*apd.Decimal]()
	case tBinary, tVarbinary, tLongVarbinary:
		return reflect.TypeFor[[]byte]()
	}
	if null {
		return reflect.TypeFor[sql.NullString]()
	}
	return reflect.TypeFor[string]()
}

// sqlTypeName names an ODBC type code, for a driver that gives no name.
func sqlTypeName(t int16) string {
	names := map[int16]string{
		tChar: "CHAR", tNumeric: "NUMERIC", tDecimal: "DECIMAL", tInteger: "INTEGER",
		tSmallint: "SMALLINT", tFloat: "FLOAT", tReal: "REAL", tDouble: "DOUBLE",
		tDate: "DATE", tTime: "TIME", tVarchar: "VARCHAR", tTypeDate: "DATE",
		tTypeTime: "TIME", tTypeTimestamp: "TIMESTAMP", tLongVarchar: "LONGVARCHAR",
		tBinary: "BINARY", tVarbinary: "VARBINARY", tLongVarbinary: "LONGVARBINARY",
		tBigint: "BIGINT", tTinyint: "TINYINT", tBit: "BIT", tWChar: "WCHAR",
		tWVarchar: "WVARCHAR", tWLongVarchar: "WLONGVARCHAR",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return fmt.Sprintf("SQL_TYPE_%d", t)
}
