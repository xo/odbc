package test

import (
	"database/sql"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/xo/dbimp"
	"github.com/xo/dbimp/dbimptest"
)

// typeCase is the round trip of one kind of value, with the type of the column
// that holds it in each product.
type typeCase struct {
	name string
	// column returns the type of the column for the product, or the reason the
	// product has none, which skips the case.
	column func(p product) (typ, skip string)
	values []dbimptest.Value
	// adjust changes the values for a product that holds less than the others.
	adjust func(p product, values []dbimptest.Value) []dbimptest.Value
	equal  func(got, want any) bool
}

func decimal(s string) *apd.Decimal {
	d, _, err := apd.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func sameDecimal(got, want any) bool {
	g, ok := got.(*apd.Decimal)
	w, ok2 := want.(*apd.Decimal)
	return ok && ok2 && g.Cmp(w) == 0
}

// by returns the column type that a map names for a product, and a reason when
// it names none.
func by(types map[string]string) func(p product) (string, string) {
	return func(p product) (string, string) {
		if t, ok := types[p.name]; ok {
			return t, ""
		}
		return "", "no such column type is tested for " + p.name
	}
}

var typeCases = []typeCase{{
	name: "integer",
	column: by(map[string]string{
		"postgres": "bigint", "mariadb": "bigint", "mysql": "bigint", "sqlserver": "bigint", "sqlite": "integer", "duckdb": "bigint",
	}),
	values: []dbimptest.Value{
		{Name: "max", In: int64(math.MaxInt64)},
		{Name: "min", In: int64(math.MinInt64)},
		{Name: "zero", In: int64(0)},
	},
}, {
	name: "float",
	column: by(map[string]string{
		"postgres": "double precision", "mariadb": "double", "mysql": "double", "sqlserver": "float", "sqlite": "double", "duckdb": "double",
	}),
	values: []dbimptest.Value{
		{Name: "half", In: 1.5},
		{Name: "negative", In: -0.25},
		{Name: "zero", In: 0.0},
	},
}, {
	name: "decimal",
	column: by(map[string]string{
		"postgres": "numeric(20,4)", "mariadb": "decimal(20,4)", "mysql": "decimal(20,4)", "sqlserver": "decimal(20,4)", "duckdb": "decimal(20,4)",
	}),
	values: []dbimptest.Value{
		{Name: "large", In: "12345678901234.5678", Want: decimal("12345678901234.5678")},
		{Name: "negative", In: "-0.0001", Want: decimal("-0.0001")},
		{Name: "zero", In: "0", Want: decimal("0")},
	},
	equal: sameDecimal,
}, {
	name: "unsigned",
	column: by(map[string]string{
		"mariadb": "bigint unsigned", "mysql": "bigint unsigned", "duckdb": "ubigint",
	}),
	// database/sql refuses a uint64 with the high bit set as an argument, so
	// the largest is written as text
	values: []dbimptest.Value{
		{Name: "max", In: "18446744073709551615", Want: uint64(math.MaxUint64)},
		{Name: "zero", In: "0", Want: uint64(0)},
	},
}, {
	name: "string",
	column: func(p product) (string, string) {
		return p.unicode, ""
	},
	values: []dbimptest.Value{
		{Name: "ascii", In: "hello"},
		{Name: "unicode", In: "héllo ✓ 世界"},
		{Name: "empty", In: ""},
		{Name: "long", In: strings.Repeat("abcdefghij", 15)},
	},
}, {
	name: "long text",
	column: by(map[string]string{
		"postgres": "text", "mariadb": "longtext", "mysql": "longtext", "sqlserver": "nvarchar(max)", "sqlite": "text", "duckdb": "varchar",
	}),
	values: []dbimptest.Value{
		{Name: "100000", In: strings.Repeat("a", 100000)},
		{Name: "unicode", In: strings.Repeat("世", 30000)},
	},
}, {
	name: "binary",
	column: func(p product) (string, string) {
		return p.binary, ""
	},
	values: []dbimptest.Value{
		{Name: "bytes", In: []byte{0, 1, 2, 255}},
		{Name: "text", In: []byte("hello")},
	},
}, {
	name: "boolean",
	column: by(map[string]string{
		"postgres": "boolean", "sqlserver": "bit", "duckdb": "boolean",
	}),
	values: []dbimptest.Value{
		{Name: "true", In: true},
		{Name: "false", In: false},
	},
}, {
	name: "date",
	column: by(map[string]string{
		"postgres": "date", "mariadb": "date", "mysql": "date", "sqlserver": "date", "sqlite": "date", "duckdb": "date",
	}),
	values: []dbimptest.Value{
		{Name: "today", In: dbimp.Date{Year: 2026, Month: 10, Day: 7}},
		{Name: "epoch", In: dbimp.Date{Year: 1970, Month: 1, Day: 1}},
	},
}, {
	name: "time",
	column: by(map[string]string{
		"postgres": "time(6)", "mariadb": "time(6)", "mysql": "time(6)", "sqlserver": "time(6)", "duckdb": "time",
	}),
	values: []dbimptest.Value{
		{Name: "fraction", In: dbimp.LocalTime{Hour: 12, Minute: 30, Second: 45, Nanosecond: 123456000}},
		{Name: "midnight", In: dbimp.LocalTime{}},
	},
}, {
	name: "timestamp",
	column: by(map[string]string{
		"postgres": "timestamp(6)", "mariadb": "datetime(6)", "mysql": "datetime(6)", "sqlserver": "datetime2(6)", "sqlite": "timestamp", "duckdb": "timestamp",
	}),
	values: []dbimptest.Value{{
		Name: "fraction",
		In:   time.Date(2026, 10, 7, 12, 30, 45, 123456000, time.UTC),
		Want: dbimp.LocalDateTime{
			Date: dbimp.Date{Year: 2026, Month: 10, Day: 7},
			Time: dbimp.LocalTime{Hour: 12, Minute: 30, Second: 45, Nanosecond: 123456000},
		},
	}, {
		Name: "epoch",
		In:   time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		Want: dbimp.LocalDateTime{Date: dbimp.Date{Year: 1970, Month: 1, Day: 1}},
	}},
	// The SQLite ODBC driver keeps milliseconds and drops the rest.
	adjust: func(p product, values []dbimptest.Value) []dbimptest.Value {
		if p.name != "sqlite" {
			return values
		}
		out := slices.Clone(values)
		out[0].In = time.Date(2026, 10, 7, 12, 30, 45, 123000000, time.UTC)
		out[0].Want = dbimp.LocalDateTime{
			Date: dbimp.Date{Year: 2026, Month: 10, Day: 7},
			Time: dbimp.LocalTime{Hour: 12, Minute: 30, Second: 45, Nanosecond: 123000000},
		}
		return out
	},
}, {
	name: "guid",
	column: by(map[string]string{
		"postgres": "uuid", "sqlserver": "uniqueidentifier", "duckdb": "uuid",
	}),
	values: []dbimptest.Value{
		{Name: "a", In: "6f9619ff-8b86-d011-b42d-00c04fc964ff"},
		{Name: "b", In: "00000000-0000-0000-0000-000000000000"},
	},
	equal: func(got, want any) bool {
		g, ok := got.(string)
		w, ok2 := want.(string)
		return ok && ok2 && strings.EqualFold(g, w)
	},
}}

// literal writes a value as an SQL literal for the product, or says that it
// cannot.
func literal(p product, v any) (string, error) {
	text := func(s string) string {
		q := "'" + strings.ReplaceAll(s, "'", "''") + "'"
		if p.name == "sqlserver" {
			return "N" + q
		}
		return q
	}
	switch v := v.(type) {
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		switch {
		case p.name == "postgres":
			return strconv.FormatBool(v), nil
		case v:
			return "1", nil
		}
		return "0", nil
	case string:
		if p.name == "sqlite" && !isASCII(v) {
			// The SQLite ODBC driver converts the text of a statement to a narrow
			// string, and on macOS and in a container with no locale it cuts each
			// character to its low byte. A literal can then hold ASCII only. An
			// argument is not affected, and the runner of CI converts it correctly.
			return "", fmt.Errorf("writing %q as a literal: %w", v, dbimp.ErrNotSupported)
		}
		return text(v), nil
	case dbimp.Date:
		return "'" + v.String() + "'", nil
	case dbimp.LocalTime:
		return "'" + v.String() + "'", nil
	case time.Time:
		return "'" + v.Format("2006-01-02 15:04:05.999999") + "'", nil
	}
	return "", fmt.Errorf("writing %T as a literal: %w", v, dbimp.ErrNotSupported)
}

// TestRoundTrip stores values of each kind through the driver and reads them
// back, once as arguments and once as literals. It compares the Go type as well
// as the value, so D18 is checked by it.
func TestRoundTrip(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		for _, c := range typeCases {
			t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
				typ, skip := c.column(p)
				if skip != "" {
					t.Skip(skip)
				}
				values := c.values
				if c.adjust != nil {
					values = c.adjust(p, values)
				}
				table := "odbc_rt_" + strings.ReplaceAll(c.name, " ", "_")
				dbimptest.RoundTrip(t, db, dbimptest.RoundTripCase{
					Type: c.name,
					Setup: []string{
						"DROP TABLE IF EXISTS " + table,
						"CREATE TABLE " + table + " (k varchar(100) PRIMARY KEY, v " + typ + ")",
					},
					Teardown: []string{"DROP TABLE IF EXISTS " + table},
					Insert:   "INSERT INTO " + table + " (k, v) VALUES (?, ?)",
					Literal: func(key string, v any) (string, error) {
						lit, err := literal(p, v)
						if err != nil {
							return "", err
						}
						return "INSERT INTO " + table + " (k, v) VALUES ('" + key + "', " + lit + ")", nil
					},
					Select: "SELECT v FROM " + table + " WHERE k = ?",
					Update: "UPDATE " + table + " SET v = ? WHERE k = ?",
					Delete: "DELETE FROM " + table + " WHERE k = ?",
					Values: values,
					Equal:  c.equal,
				})
			})
		}
	})
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
