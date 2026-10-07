package test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/xo/dbimp/dbimptest"
)

// TestNoGoroutineLeaks runs queries to the end, abandons some and cancels one.
// It closes the database and checks that no goroutine of the driver is left
// (step 12 of dbimp's DRIVER.md). A watcher that is not stopped shows here.
func TestNoGoroutineLeaks(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		dbimptest.CheckGoroutines(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		for range 5 {
			var n int
			if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
				t.Fatal(err)
			}
			abandon(ctx, t, db)
		}
		short, stop := context.WithTimeout(ctx, time.Millisecond)
		defer stop()
		<-short.Done()
		_ = db.QueryRowContext(short, "SELECT 1").Scan(new(int))
	})
}

// abandon opens a result and closes it without reading a row.
func abandon(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
