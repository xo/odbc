package test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"testing"

	"github.com/xo/dbmeta"
)

// TestFixture builds the dbmeta fixture of each database through the driver,
// asks dbmeta for the version of the server, and runs every query that dbmeta
// answers for it. A statement that the driver cannot run is a finding.
func TestFixture(t *testing.T) {
	each(t, func(t *testing.T, p product, db *sql.DB) {
		ctx := t.Context()
		versions, err := p.dialect.Version(ctx, db)
		if err != nil {
			t.Fatalf("reading the version: %v", err)
		}
		m, err := dbmeta.New(p.dialect, versions)
		if err != nil {
			t.Fatalf("building the meta: %v", err)
		}
		up, down, err := p.fixture(versions)
		if err != nil {
			t.Fatalf("resolving the fixture: %v", err)
		}
		run := func(ctx context.Context, steps []step, fatal bool) {
			for _, s := range steps {
				if _, err := db.ExecContext(ctx, s.query); err != nil && fatal {
					t.Fatalf("%s: %v\n%s", s.name, err, s.query)
				}
			}
		}
		run(ctx, down, false)
		run(ctx, up, true)
		t.Cleanup(func() { run(context.WithoutCancel(ctx), down, false) })
		t.Logf("server reports %s, and the fixture ran %d steps", m, len(up))

		var ran, tooOld, unsupported, reordered int
		for _, q := range dbmeta.Queries() {
			if q.Support(m) != dbmeta.Supported {
				unsupported++
				continue
			}
			query, vals, err := q.Build(m, nil)
			switch {
			case errors.Is(err, dbmeta.ErrVersionTooOld):
				tooOld++
				continue
			case err != nil:
				t.Errorf("%s: building: %v", q.Name(), err)
				continue
			}
			query, ok := positional(query)
			if !ok {
				reordered++
				continue
			}
			cols, err := drain(t, db, query, vals)
			if err != nil {
				t.Errorf("%s: %v\n%s", q.Name(), err, query)
				continue
			}
			fields, err := q.Fields(m)
			switch {
			case err != nil:
				t.Errorf("%s: reading the fields: %v", q.Name(), err)
				continue
			case len(cols) != len(fields):
				t.Errorf("%s: declares %d fields and returns %d columns", q.Name(), len(fields), len(cols))
				continue
			}
			for i := range cols {
				if cols[i] != fields[i].Name {
					t.Errorf("%s: column %d is %q and the field is %q", q.Name(), i, cols[i], fields[i].Name)
				}
			}
			ran++
		}
		t.Logf("%d queries ran, %d were too new, %d are not for this product, and %d use a placeholder twice or out of order",
			ran, tooOld, unsupported, reordered)
		if ran == 0 {
			t.Error("no query ran")
		}
	})
}

// numbered matches a placeholder that carries its number: $1 or @p1.
var numbered = regexp.MustCompile(`\$([0-9]+)|@p([0-9]+)`)

// positional rewrites numbered placeholders as the ? that ODBC takes (D16). It
// reports false when the numbers are not 1, 2, 3 and so on, each once, because
// ? cannot say which argument a placeholder takes then.
func positional(query string) (string, bool) {
	want, ok := 1, true
	out := numbered.ReplaceAllStringFunc(query, func(m string) string {
		n, _ := strconv.Atoi(numbered.FindStringSubmatch(m)[1] + numbered.FindStringSubmatch(m)[2])
		if n != want {
			ok = false
		}
		want++
		return "?"
	})
	return out, ok
}

// drain runs a query and reads every column of every row, so that each type
// the query returns passes through the driver. It returns the column names.
func drain(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	dest := make([]any, len(cols))
	for rows.Next() {
		row := make([]any, len(cols))
		for i := range dest {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
	}
	return cols, rows.Err()
}
