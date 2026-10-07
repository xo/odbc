package odbc

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config is what a data source name holds once it is parsed.
type Config struct {
	// ConnString is the ODBC connection string handed to the driver manager.
	ConnString string
	// Manager is the path of the driver manager library. It is empty to use
	// the default name of the system.
	Manager string
	// WChar is the size of SQLWCHAR in bytes, 2 or 4. It is 0 to detect it.
	WChar int
	// TraceFile is the file that the driver manager writes its trace to. An
	// empty name leaves the trace off (D23).
	TraceFile string
	// OnWarning is called with each informational diagnostic that a connection
	// or a statement returns with a success code, such as a message that the
	// database printed. It runs on the goroutine of the call, so it must
	// return quickly. Nil ignores them (D23).
	OnWarning func(*Error)
	// Location, when it is set, makes a timestamp a time.Time in that location,
	// and sends a time.Time as its time in that location. When it is nil, a
	// timestamp is a dbimp.LocalDateTime, as D18 says (D24).
	Location *time.Location
}

// ParseDSN parses a data source name. Two forms are accepted.
//
// A URL whose scheme is odbc+<driver>, where the driver is the name the
// driver manager knows, with a plus sign for each space:
//
//	odbc+PostgreSQL+Unicode://user:pass@host:5432/dbname?sslmode=disable
//	odbc+ODBC+Driver+18+for+SQL+Server://sa:pass@host:1433/instance/dbname
//
// The path is the database name, or the instance and then the database name.
// An instance becomes part of the server, as host\instance. The query keys
// become keys of the connection string. The key driver replaces the driver
// name, and it can be the path of a library. The key manager is the path of
// the driver manager, and the key wchar is the size of SQLWCHAR in bytes, 2 or
// 4, for a manager that the driver cannot probe. Neither is passed on.
//
// Anything else is taken to be an ODBC connection string, such as the one
// dburl builds, and is passed on as it is.
func ParseDSN(dsn string) (Config, error) {
	if !strings.Contains(dsn, "://") {
		if strings.TrimSpace(dsn) == "" {
			return Config{}, errors.New("parsing the data source name: it is empty")
		}
		return Config{ConnString: dsn}, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return Config{}, fmt.Errorf("parsing the data source name: %w", err)
	}
	// url.Parse lowercases the scheme, and a driver name keeps its case
	scheme, _, _ := strings.Cut(dsn, "://")
	driver, ok := strings.CutPrefix(scheme, "odbc+")
	if !ok || driver == "" {
		return Config{}, fmt.Errorf("parsing the data source name: the scheme %q is not odbc+<driver>", u.Scheme)
	}
	var cfg Config
	q := u.Query()
	driver = strings.ReplaceAll(driver, "+", " ")
	if v := q.Get("driver"); v != "" {
		driver = v
	}
	cfg.Manager = q.Get("manager")
	if v := q.Get("wchar"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || (n != 2 && n != 4) {
			return Config{}, fmt.Errorf("parsing the data source name: wchar is %q, and must be 2 or 4", v)
		}
		cfg.WChar = n
	}
	q.Del("driver")
	q.Del("manager")
	q.Del("wchar")
	var pairs [][2]string
	add := func(k, v string) {
		if v != "" {
			pairs = append(pairs, [2]string{k, v})
		}
	}
	add("Driver", driver)
	server := u.Hostname()
	var instance, dbname string
	path := strings.Trim(u.Path, "/")
	instance, dbname, found := strings.CutLast(path, "/")
	if !found {
		instance, dbname = "", path
	}
	if instance != "" {
		server += `\` + instance
	}
	port := u.Port()
	if port != "" && strings.Contains(strings.ToLower(driver), "sql server") {
		// the Microsoft drivers take the port as part of the server and
		// ignore a Port key
		server += "," + port
		port = ""
	}
	add("Server", server)
	add("Port", port)
	add("Database", dbname)
	if u.User != nil {
		add("UID", u.User.Username())
		pass, _ := u.User.Password()
		add("PWD", pass)
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		for _, v := range q[k] {
			add(k, v)
		}
	}
	var b strings.Builder
	for _, p := range pairs {
		b.WriteString(p[0] + "=" + quoteValue(p[1]) + ";")
	}
	cfg.ConnString = b.String()
	return cfg, nil
}

// quoteValue braces a connection string value that holds a character which
// ends it early. A closing brace inside is doubled.
func quoteValue(s string) string {
	if !strings.ContainsAny(s, ";{}= ") && s == strings.TrimSpace(s) {
		return s
	}
	return "{" + strings.ReplaceAll(s, "}", "}}") + "}"
}
