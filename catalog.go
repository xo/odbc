package odbc

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"runtime"
	"unsafe"
)

// TableInfo is a table or a view that SQLTables lists.
type TableInfo struct {
	Catalog string
	Schema  string
	Name    string
	// Type is TABLE, VIEW, SYSTEM TABLE and so on, as the database driver names it.
	Type    string
	Remarks string
}

// ColumnInfo is a column that SQLColumns lists.
type ColumnInfo struct {
	Catalog string
	Schema  string
	Table   string
	Name    string
	// DataType is the SQL type code of ODBC, such as 12 for SQL_VARCHAR.
	DataType int
	// TypeName is the name that the database gives the type.
	TypeName string
	// Size is the length of a text or the precision of a number.
	Size int
	// Digits is the number of digits after the decimal point.
	Digits int
	// Nullable is 0 for a column that holds no NULL, 1 for one that does, and 2
	// when the database driver does not know.
	Nullable int
	Remarks  string
	Default  string
	// Ordinal is the position of the column in the table, from 1.
	Ordinal int
}

// PrimaryKeyInfo is a column of the primary key that SQLPrimaryKeys lists.
type PrimaryKeyInfo struct {
	Catalog string
	Schema  string
	Table   string
	Column  string
	// Sequence is the position of the column in the key, from 1.
	Sequence int
	// Name is the name of the key, when the database has one.
	Name string
}

// Tables lists the tables that match. An empty argument matches everything. The
// name arguments are patterns, in which % matches any text and _ matches one
// character. tableTypes is a comma separated list such as "TABLE,VIEW" (D24).
func (c *conn) Tables(ctx context.Context, catalog, schema, table, tableTypes string) ([]TableInfo, error) {
	cat, sch, tab, typ := c.api.encodeOrNil(catalog), c.api.encodeOrNil(schema), c.api.encodeOrNil(table), c.api.encodeOrNil(tableTypes)
	rows, err := c.catalogRows(ctx, "listing the tables", func(h uintptr) int16 {
		return c.api.tables(h, ptr(cat), len0(cat), ptr(sch), len0(sch), ptr(tab), len0(tab), ptr(typ), len0(typ))
	})
	runtime.KeepAlive([][]byte{cat, sch, tab, typ})
	if err != nil {
		return nil, err
	}
	out := make([]TableInfo, len(rows))
	for i, r := range rows {
		out[i] = TableInfo{Catalog: str(r, 0), Schema: str(r, 1), Name: str(r, 2), Type: str(r, 3), Remarks: str(r, 4)}
	}
	return out, nil
}

// Columns lists the columns that match, in the order of the table. An empty
// argument matches everything, and the name arguments are patterns (D24).
func (c *conn) Columns(ctx context.Context, catalog, schema, table, column string) ([]ColumnInfo, error) {
	cat, sch, tab, col := c.api.encodeOrNil(catalog), c.api.encodeOrNil(schema), c.api.encodeOrNil(table), c.api.encodeOrNil(column)
	rows, err := c.catalogRows(ctx, "listing the columns", func(h uintptr) int16 {
		return c.api.columns(h, ptr(cat), len0(cat), ptr(sch), len0(sch), ptr(tab), len0(tab), ptr(col), len0(col))
	})
	runtime.KeepAlive([][]byte{cat, sch, tab, col})
	if err != nil {
		return nil, err
	}
	out := make([]ColumnInfo, len(rows))
	for i, r := range rows {
		out[i] = ColumnInfo{
			Catalog: str(r, 0), Schema: str(r, 1), Table: str(r, 2), Name: str(r, 3),
			DataType: num(r, 4), TypeName: str(r, 5), Size: num(r, 6), Digits: num(r, 8),
			Nullable: num(r, 10), Remarks: str(r, 11), Default: str(r, 12), Ordinal: num(r, 16),
		}
	}
	return out, nil
}

// PrimaryKeys lists the columns of the primary key of one table. The table name
// is not a pattern, and an empty catalog or schema matches the current one
// (D24).
func (c *conn) PrimaryKeys(ctx context.Context, catalog, schema, table string) ([]PrimaryKeyInfo, error) {
	cat, sch, tab := c.api.encodeOrNil(catalog), c.api.encodeOrNil(schema), c.api.encodeOrNil(table)
	rows, err := c.catalogRows(ctx, "listing the primary key", func(h uintptr) int16 {
		return c.api.primaryKeys(h, ptr(cat), len0(cat), ptr(sch), len0(sch), ptr(tab), len0(tab))
	})
	runtime.KeepAlive([][]byte{cat, sch, tab})
	if err != nil {
		return nil, err
	}
	out := make([]PrimaryKeyInfo, len(rows))
	for i, r := range rows {
		out[i] = PrimaryKeyInfo{Catalog: str(r, 0), Schema: str(r, 1), Table: str(r, 2), Column: str(r, 3), Sequence: num(r, 4), Name: str(r, 5)}
	}
	return out, nil
}

// catalogRows runs a catalog function on a statement of its own and reads every
// row of the result.
func (c *conn) catalogRows(ctx context.Context, op string, call func(h uintptr) int16) ([][]driver.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s, err := c.newStmt("")
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.free() }()
	stop := c.watch(ctx, s.h)
	ret := call(s.h)
	stop()
	if err := c.api.check(op, ret, handleStmt, s.h); err != nil {
		return nil, wrapCtx(ctx, err)
	}
	r := &rows{ctx: ctx, s: s, own: true, undo: func() {}}
	if err := r.describe(); err != nil {
		return nil, err
	}
	var out [][]driver.Value
	for {
		err := r.NextRow()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		out = append(out, append([]driver.Value(nil), r.row...))
	}
}

// encodeOrNil encodes a string, and returns nil for an empty one, which ODBC
// reads as "not given".
func (a *api) encodeOrNil(s string) []byte {
	if s == "" {
		return nil
	}
	return a.encode(s)
}

// ptr returns the address of an encoded string, or nil.
func ptr(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}

// len0 returns SQL_NTS for a string that is given, and 0 for one that is not.
func len0(b []byte) int16 {
	if len(b) == 0 {
		return 0
	}
	return nts
}

// str returns column i of a row as text.
func str(r []driver.Value, i int) string {
	if i >= len(r) {
		return ""
	}
	if s, ok := r[i].(string); ok {
		return s
	}
	return ""
}

// num returns column i of a row as an integer.
func num(r []driver.Value, i int) int {
	if i >= len(r) {
		return 0
	}
	switch v := r[i].(type) {
	case int64:
		return int(v)
	case uint64:
		return int(v)
	}
	return 0
}
