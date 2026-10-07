package odbc

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/xo/dbimp"
)

// Limits of a value that a bound buffer holds. A longer value, or one of an
// unknown length, is read with SQLGetData and never in a block (D24).
const (
	maxBlockChars = 4000
	maxBlockBytes = 8000
	// The sizes of the text that a type with no length needs.
	timeChars = 40
	guidChars = 40
	boolChars = 16
)

// plan says how to read one column: with which C type, how many bytes a fixed
// value takes, and how to turn the bytes into a Go value. The same plan serves
// a read with SQLGetData and a read from a bound buffer.
type plan struct {
	ctype int16
	// size is the number of bytes of a value of a fixed size, or 0 when the
	// length varies.
	size int
	// term is the size of the terminator that the driver adds to character data.
	term int
	// conv turns the bytes of a value into the Go value. It copies what it keeps,
	// because the buffer is read again.
	conv func(raw []byte) (driver.Value, error)
	// stride is the size of a bound buffer for one value, or 0 when the column
	// cannot be bound.
	stride int
}

// planFor makes the plan of a column.
func (r *rows) planFor(c column) plan {
	a := r.s.c.api
	loc := r.s.c.loc
	text := func(chars int, conv func(string) (driver.Value, error)) plan {
		p := plan{ctype: cWChar, term: a.wchar, conv: func(raw []byte) (driver.Value, error) {
			return conv(a.decode(raw))
		}}
		if chars > 0 && chars <= maxBlockChars {
			p.stride = (chars + 1) * a.wchar
		}
		return p
	}
	asString := func(s string) (driver.Value, error) { return s, nil }
	if c.boolText && c.sqlType != tBit {
		return text(boolChars, func(s string) (driver.Value, error) {
			switch strings.ToLower(s) {
			case "t", "true", "1", "y", "yes":
				return true, nil
			case "f", "false", "0", "n", "no":
				return false, nil
			}
			return nil, fmt.Errorf("reading %q as a boolean", s)
		})
	}
	switch c.sqlType {
	case tBigint:
		if c.unsigned {
			return plan{ctype: cUBigint, size: 8, conv: func(raw []byte) (driver.Value, error) {
				return binary.NativeEndian.Uint64(raw), nil
			}}
		}
		fallthrough
	case tTinyint, tSmallint, tInteger:
		return plan{ctype: cSBigint, size: 8, conv: func(raw []byte) (driver.Value, error) {
			return int64(binary.NativeEndian.Uint64(raw)), nil
		}}
	case tBit:
		return plan{ctype: cSBigint, size: 8, conv: func(raw []byte) (driver.Value, error) {
			return binary.NativeEndian.Uint64(raw) != 0, nil
		}}
	case tReal, tFloat, tDouble:
		return plan{ctype: cDouble, size: 8, conv: func(raw []byte) (driver.Value, error) {
			return math.Float64frombits(binary.NativeEndian.Uint64(raw)), nil
		}}
	case tDate, tTypeDate:
		return plan{ctype: cTimestamp, size: timestampSize, conv: func(raw []byte) (driver.Value, error) {
			ts := readTimestamp(raw)
			return dbimp.Date{Year: int(ts.Year), Month: time.Month(ts.Month), Day: int(ts.Day)}, nil
		}}
	case tTypeTimestamp:
		return plan{ctype: cTimestamp, size: timestampSize, conv: func(raw []byte) (driver.Value, error) {
			ts := readTimestamp(raw)
			local := dbimp.LocalDateTime{
				Date: dbimp.Date{Year: int(ts.Year), Month: time.Month(ts.Month), Day: int(ts.Day)},
				Time: dbimp.LocalTime{Hour: int(ts.Hour), Minute: int(ts.Minute), Second: int(ts.Second), Nanosecond: int(ts.Fraction)},
			}
			if loc != nil {
				return local.In(loc), nil
			}
			return local, nil
		}}
	case tBinary, tVarbinary, tLongVarbinary:
		p := plan{ctype: cBinary, conv: func(raw []byte) (driver.Value, error) {
			return bytes.Clone(raw), nil
		}}
		if c.size > 0 && c.size <= maxBlockBytes {
			p.stride = int(c.size)
		}
		return p
	case tDecimal, tNumeric:
		return text(int(c.size)+3, func(s string) (driver.Value, error) {
			d, _, err := apd.NewFromString(s)
			if err != nil {
				return nil, fmt.Errorf("reading %q as a decimal: %w", s, err)
			}
			return d, nil
		})
	case tTime, tTypeTime, tSSTime:
		return text(timeChars, func(s string) (driver.Value, error) {
			t, err := dbimp.ParseLocalTime(s)
			if err != nil {
				return nil, err
			}
			return t, nil
		})
	case tSSOffset:
		return text(timeChars, func(s string) (driver.Value, error) {
			t, err := time.Parse("2006-01-02 15:04:05.999999999 -07:00", s)
			if err != nil {
				return nil, fmt.Errorf("reading %q as a timestamp with an offset: %w", s, err)
			}
			return t, nil
		})
	case tGUID:
		return text(guidChars, asString)
	case tChar, tVarchar, tWChar, tWVarchar:
		return text(int(c.size), asString)
	}
	// a long text type, or a type that has no kind of its own, is text of an
	// unknown length
	return text(0, asString)
}

// timestampSize is the size of SQL_TIMESTAMP_STRUCT.
const timestampSize = 16

// readTimestamp reads SQL_TIMESTAMP_STRUCT from the bytes of a column.
func readTimestamp(raw []byte) timestamp {
	return timestamp{
		Year:     int16(binary.NativeEndian.Uint16(raw[0:])),
		Month:    binary.NativeEndian.Uint16(raw[2:]),
		Day:      binary.NativeEndian.Uint16(raw[4:]),
		Hour:     binary.NativeEndian.Uint16(raw[6:]),
		Minute:   binary.NativeEndian.Uint16(raw[8:]),
		Second:   binary.NativeEndian.Uint16(raw[10:]),
		Fraction: binary.NativeEndian.Uint32(raw[12:]),
	}
}
