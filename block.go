package odbc

import (
	"database/sql/driver"
	"fmt"
	"io"
	"unsafe"
)

// Attributes of the statement that block fetching sets.
const (
	attrRowBindType    = 5
	attrRowsFetchedPtr = 26
	attrRowArraySize   = 27
	freeStmtUnbind     = 2
	bindByColumn       = 0
	maxFetchSize       = 100000
)

// block holds the state of rows that are fetched in blocks (D24). Each column
// has a buffer that holds a value for each row of the block, and an array of
// indicators. The driver fills them with one call of SQLFetch.
type block struct {
	n       int
	fetched uintptr
	count   int
	pos     int
	cols    []boundColumn
}

// boundColumn is the memory of one bound column.
type boundColumn struct {
	buf    []byte
	ind    []int
	stride int
}

// enableBlock fetches the rows of the result in blocks of n. It does nothing
// when a column cannot be bound or the database driver refuses, and the rows
// are then read one at a time as before. A fetch size is a hint about speed and
// not a limit, so it never fails the statement (D24).
func (r *rows) enableBlock(n int) {
	a, h := r.s.c.api, r.s.h
	b := &block{n: n, cols: make([]boundColumn, len(r.cols))}
	for i, p := range r.plans {
		stride := p.stride
		if p.size > 0 {
			stride = p.size
		}
		if stride == 0 {
			return
		}
		b.cols[i] = boundColumn{buf: make([]byte, n*stride), ind: make([]int, n), stride: stride}
	}
	r.blk = b
	ok := a.setStmtAttr(h, attrRowBindType, bindByColumn, 0) == sqlSuccess &&
		a.setStmtAttr(h, attrRowArraySize, uintptr(n), 0) == sqlSuccess &&
		a.setStmtAttr(h, attrRowsFetchedPtr, uintptr(unsafe.Pointer(&b.fetched)), 0) == sqlSuccess
	for i, p := range r.plans {
		if !ok {
			break
		}
		c := &b.cols[i]
		ok = a.bindCol(h, uint16(i+1), p.ctype, unsafe.Pointer(&c.buf[0]), c.stride, &c.ind[0]) == sqlSuccess
	}
	if !ok {
		r.disableBlock()
	}
}

// disableBlock puts the statement back to one row for each fetch.
func (r *rows) disableBlock() {
	if r.blk == nil {
		return
	}
	a, h := r.s.c.api, r.s.h
	_ = a.freeStmt(h, freeStmtUnbind)
	_ = a.setStmtAttr(h, attrRowsFetchedPtr, 0, 0)
	_ = a.setStmtAttr(h, attrRowArraySize, 1, 0)
	r.blk = nil
}

// nextBlockRow reads the next row from the block, and fetches another block
// when the last one is used up.
func (r *rows) nextBlockRow() error {
	b := r.blk
	if b.pos >= b.count {
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
		b.count, b.pos = int(b.fetched), 0
		if b.count == 0 {
			return io.EOF
		}
	}
	if len(r.row) != len(r.cols) {
		r.row = make([]driver.Value, len(r.cols))
	}
	i := b.pos
	b.pos++
	for j, p := range r.plans {
		c := b.cols[j]
		ind := c.ind[i]
		if ind == nullData {
			r.row[j] = nil
			continue
		}
		size := p.size
		if size == 0 {
			if ind == noTotal || ind > c.stride-p.term {
				return fmt.Errorf("reading column %q: a value is longer than the buffer of %d bytes: %w",
					r.cols[j].name, c.stride-p.term, ErrTruncated)
			}
			size = ind
		}
		raw := c.buf[i*c.stride : i*c.stride+size]
		v, err := p.conv(raw)
		if err != nil {
			return err
		}
		r.row[j] = v
	}
	return nil
}
