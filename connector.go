package odbc

import (
	"context"
	"database/sql/driver"
	"io"
	"math"
	"sync"
	"time"
	"unsafe"
)

// Connector opens connections. It owns the ODBC environment, which the
// connections it opens belong to. Close it after every connection is closed,
// and database/sql does that when the DB is closed.
type Connector struct {
	api *api
	cfg Config

	mu  sync.Mutex
	env uintptr
}

var (
	_ driver.Connector = (*Connector)(nil)
	_ io.Closer        = (*Connector)(nil)
)

// NewConnector loads the driver manager and allocates an environment.
func NewConnector(cfg Config) (*Connector, error) {
	a, err := loadManager(cfg.Manager, cfg.WChar)
	if err != nil {
		return nil, err
	}
	c := &Connector{api: a, cfg: cfg}
	if err := a.check("allocating the environment", a.allocHandle(handleEnv, 0, &c.env), handleEnv, 0); err != nil {
		return nil, err
	}
	if err := a.check("setting the ODBC version", a.setEnvAttr(c.env, attrODBCVersion, odbcVersion3, 0), handleEnv, c.env); err != nil {
		_ = a.freeHandle(handleEnv, c.env)
		return nil, err
	}
	return c, nil
}

// Driver returns the driver.
func (*Connector) Driver() driver.Driver { return &Driver{} }

// Connect opens a connection. The driver manager cannot cancel a connection
// that is being made, so the context is checked before the call and its
// deadline sets the login timeout.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	env := c.env
	c.mu.Unlock()
	if env == 0 {
		return nil, errClosed
	}
	a := c.api
	var dbc uintptr
	if err := a.check("allocating the connection", a.allocHandle(handleDbc, env, &dbc), handleEnv, env); err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		secs := max(1, int(math.Ceil(time.Until(dl).Seconds())))
		// a driver that ignores the login timeout is not an error
		_ = a.setConnAttr(dbc, attrLoginTimeout, uintptr(secs), 0)
	}
	in := a.encode(c.cfg.ConnString)
	out := make([]byte, 1024*a.wchar)
	var n int16
	ret := a.driverConnect(dbc, 0, unsafe.Pointer(&in[0]), nts, unsafe.Pointer(&out[0]), int16(a.units(len(out))), &n, driverNoPrompt)
	if err := a.check("connecting", ret, handleDbc, dbc); err != nil {
		_ = a.freeHandle(handleDbc, dbc)
		return nil, err
	}
	c2 := &conn{api: a, dbc: dbc}
	name := make([]byte, 128*a.wchar)
	var nameLen int16
	// the length of a string that SQLGetInfoW takes is in bytes
	if a.getInfo(dbc, infoDBMSName, unsafe.Pointer(&name[0]), int16(len(name)), &nameLen) == sqlSuccess {
		c2.dbms = a.decode(name)
	}
	return c2, nil
}

// Close frees the environment.
func (c *Connector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.env == 0 {
		return nil
	}
	env := c.env
	c.env = 0
	return c.api.check("freeing the environment", c.api.freeHandle(handleEnv, env), handleEnv, env)
}
