package odbc

import (
	"fmt"
	"strings"
	"unsafe"
)

// DriverInfo is an ODBC driver that the driver manager knows.
type DriverInfo struct {
	// Name is the name that a connection string gives as DRIVER.
	Name string
	// Attributes are the keys that the driver registered, such as its library.
	Attributes map[string]string
}

// DataSource is a data source name that the driver manager knows.
type DataSource struct {
	// Name is the name that a connection string gives as DSN.
	Name string
	// Description is the name of the driver of the data source.
	Description string
}

// Drivers lists the ODBC drivers that the driver manager knows. The zero
// Config uses the default driver manager of the system. Only its Manager and
// WChar apply (D23).
func Drivers(cfg Config) ([]DriverInfo, error) {
	a, env, err := cfg.environment()
	if err != nil {
		return nil, err
	}
	defer a.freeHandle(handleEnv, env)
	var out []DriverInfo
	for dir := uint16(fetchFirst); ; dir = fetchNext {
		desc := make([]byte, 256*a.wchar)
		attr := make([]byte, 4096*a.wchar)
		var descLen, attrLen int16
		ret := a.drivers(env, dir, unsafe.Pointer(&desc[0]), int16(a.units(len(desc))), &descLen,
			unsafe.Pointer(&attr[0]), int16(a.units(len(attr))), &attrLen)
		if ret == sqlNoData {
			return out, nil
		}
		if err := a.check("listing the drivers", ret, handleEnv, env); err != nil {
			return nil, err
		}
		info := DriverInfo{Name: a.decode(desc), Attributes: map[string]string{}}
		for _, kv := range a.decodeList(attr[:min(int(attrLen), a.units(len(attr)))*a.wchar]) {
			if k, v, ok := strings.Cut(kv, "="); ok {
				info.Attributes[k] = v
			}
		}
		out = append(out, info)
	}
}

// DataSources lists the data source names that the driver manager knows. The
// zero Config uses the default driver manager of the system.
func DataSources(cfg Config) ([]DataSource, error) {
	a, env, err := cfg.environment()
	if err != nil {
		return nil, err
	}
	defer a.freeHandle(handleEnv, env)
	var out []DataSource
	for dir := uint16(fetchFirst); ; dir = fetchNext {
		name := make([]byte, 256*a.wchar)
		desc := make([]byte, 512*a.wchar)
		var nameLen, descLen int16
		ret := a.dataSources(env, dir, unsafe.Pointer(&name[0]), int16(a.units(len(name))), &nameLen,
			unsafe.Pointer(&desc[0]), int16(a.units(len(desc))), &descLen)
		if ret == sqlNoData {
			return out, nil
		}
		if err := a.check("listing the data sources", ret, handleEnv, env); err != nil {
			return nil, err
		}
		out = append(out, DataSource{Name: a.decode(name), Description: a.decode(desc)})
	}
}

// environment loads the driver manager of cfg and allocates an environment.
// The caller frees the environment.
func (cfg Config) environment() (*api, uintptr, error) {
	a, err := loadManager(cfg.Manager, cfg.WChar)
	if err != nil {
		return nil, 0, err
	}
	var env uintptr
	if err := a.check("allocating the environment", a.allocHandle(handleEnv, 0, &env), handleEnv, 0); err != nil {
		return nil, 0, err
	}
	if err := a.check("setting the ODBC version", a.setEnvAttr(env, attrODBCVersion, odbcVersion3, 0), handleEnv, env); err != nil {
		_ = a.freeHandle(handleEnv, env)
		return nil, 0, fmt.Errorf("%w", err)
	}
	return a, env, nil
}

// decodeList splits the strings of a list that ends with a double zero, as the
// attributes of a driver do.
func (a *api) decodeList(b []byte) []string {
	var out []string
	for len(b) >= a.wchar {
		s := a.decode(b)
		if s == "" {
			break
		}
		out = append(out, s)
		// skip the string and its zero, which each take a unit
		n := len(a.encode(s))
		if n > len(b) {
			break
		}
		b = b[n:]
	}
	return out
}
