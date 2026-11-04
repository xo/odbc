package test

import (
	"database/sql"
	"testing"
)

// composer is a destination that takes a decimal in parts, as the type of a
// decimal package does that implements the interface of database/sql. Scan
// calls Compose when the driver gives a decimal that has Decompose.
type composer struct {
	form        byte
	negative    bool
	coefficient []byte
	exponent    int32
	calls       int
}

func (c *composer) Compose(form byte, negative bool, coefficient []byte, exponent int32) error {
	c.calls++
	c.form, c.negative, c.exponent = form, negative, exponent
	c.coefficient = append([]byte(nil), coefficient...)
	return nil
}

// TestDecimalComposes scans a decimal into a destination that has Compose, and
// checks the parts that it gets (D26). A driver that gave the decimal as text
// leaves Compose uncalled and fails the scan.
func TestDecimalComposes(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		if p.name == "sqlite" {
			t.Skip("SQLite has no decimal type")
		}
		var c composer
		err := db.QueryRowContext(t.Context(), "SELECT CAST(-12345.6789 AS decimal(20,4))").Scan(&c)
		if err != nil {
			t.Fatal(err)
		}
		if c.calls != 1 || !c.negative || c.exponent != -4 {
			t.Errorf("%s: Compose got %d calls, negative %v and exponent %d", p.name, c.calls, c.negative, c.exponent)
		}
		// 123456789 is the coefficient of -12345.6789, as big endian bytes
		want := []byte{0x07, 0x5b, 0xcd, 0x15}
		if string(c.coefficient) != string(want) {
			t.Errorf("%s: coefficient %x, want %x", p.name, c.coefficient, want)
		}
	})
}
