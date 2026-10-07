package odbc

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Return codes of the ODBC functions.
const (
	sqlSuccess         = 0
	sqlSuccessWithInfo = 1
	sqlNoData          = 100
)

// Handle types.
const (
	handleEnv  = 1
	handleDbc  = 2
	handleStmt = 3
)

// Attributes and options.
const (
	attrODBCVersion  = 200
	odbcVersion3     = 3
	attrAutocommit   = 102
	attrTxnIsolation = 108
	attrConnDead     = 1209
	attrLoginTimeout = 103
	autocommitOff    = 0
	autocommitOn     = 1
	commit           = 0
	rollback         = 1
	closeCursor      = 0
	resetParams      = 3
	nts              = -3
	nullData         = -1
	noTotal          = -4
	driverNoPrompt   = 0
	connDeadTrue     = 1
	descTypeName     = 14
	descUnsigned     = 8
	attrTrace        = 104
	attrTraceFile    = 105
	traceOn          = 1
	traceOff         = 0
	fetchNext        = 1
	fetchFirst       = 2
	// probeAttribute is an environment attribute that no manager knows.
	probeAttribute = 0x7ffffff0
	infoDBMSName   = 17
)

// api holds the ODBC functions, registered from the driver manager once.
type api struct {
	// wchar is the size of SQLWCHAR in bytes, 2 or 4 (D20).
	wchar       int
	allocHandle func(handleType int16, input uintptr, output *uintptr) int16
	freeHandle  func(handleType int16, handle uintptr) int16
	setEnvAttr  func(env uintptr, attr int32, value uintptr, length int32) int16
	setConnAttr func(dbc uintptr, attr int32, value uintptr, length int32) int16
	// setConnAttrPtr is the same function, for an attribute whose value is a
	// string.
	setConnAttrPtr func(dbc uintptr, attr int32, value unsafe.Pointer, length int32) int16
	setStmtAttr    func(stmt uintptr, attr int32, value uintptr, length int32) int16
	getConnAttr    func(dbc uintptr, attr int32, value unsafe.Pointer, bufLen int32, strLen *int32) int16
	driverConnect  func(dbc, window uintptr, in unsafe.Pointer, inLen int16, out unsafe.Pointer, outMax int16, outLen *int16, completion uint16) int16
	disconnect     func(dbc uintptr) int16
	execDirect     func(stmt uintptr, text unsafe.Pointer, length int32) int16
	prepare        func(stmt uintptr, text unsafe.Pointer, length int32) int16
	execute        func(stmt uintptr) int16
	numResultCols  func(stmt uintptr, cols *int16) int16
	describeCol    func(stmt uintptr, col uint16, name unsafe.Pointer, nameMax int16, nameLen *int16, dataType *int16, size *uintptr, digits *int16, nullable *int16) int16
	colAttribute   func(stmt uintptr, col, field uint16, charAttr unsafe.Pointer, bufLen int16, strLen *int16, numAttr *int) int16
	fetch          func(stmt uintptr) int16
	getData        func(stmt uintptr, col uint16, ctype int16, buf unsafe.Pointer, bufLen int, ind *int) int16
	rowCount       func(stmt uintptr, count *int) int16
	moreResults    func(stmt uintptr) int16
	freeStmt       func(stmt uintptr, option uint16) int16
	cancel         func(stmt uintptr) int16
	endTran        func(handleType int16, handle uintptr, completion int16) int16
	bindParameter  func(stmt uintptr, num uint16, ioType, ctype, sqlType int16, size uintptr, digits int16, value unsafe.Pointer, bufLen int, ind *int) int16
	drivers        func(env uintptr, direction uint16, desc unsafe.Pointer, descMax int16, descLen *int16, attr unsafe.Pointer, attrMax int16, attrLen *int16) int16
	dataSources    func(env uintptr, direction uint16, server unsafe.Pointer, serverMax int16, serverLen *int16, desc unsafe.Pointer, descMax int16, descLen *int16) int16
	bindCol        func(stmt uintptr, col uint16, ctype int16, buf unsafe.Pointer, bufLen int, ind *int) int16
	tables         func(stmt uintptr, cat unsafe.Pointer, catLen int16, schema unsafe.Pointer, schemaLen int16, table unsafe.Pointer, tableLen int16, tableType unsafe.Pointer, typeLen int16) int16
	columns        func(stmt uintptr, cat unsafe.Pointer, catLen int16, schema unsafe.Pointer, schemaLen int16, table unsafe.Pointer, tableLen int16, column unsafe.Pointer, columnLen int16) int16
	primaryKeys    func(stmt uintptr, cat unsafe.Pointer, catLen int16, schema unsafe.Pointer, schemaLen int16, table unsafe.Pointer, tableLen int16) int16
	getInfo        func(dbc uintptr, info uint16, value unsafe.Pointer, bufLen int16, strLen *int16) int16
	getDiagRec     func(handleType int16, handle uintptr, rec int16, state unsafe.Pointer, native *int32, msg unsafe.Pointer, bufLen int16, msgLen *int16) int16
}

// manager identifies a loaded driver manager: its path, and the width of
// SQLWCHAR that the caller asked for, which is 0 to detect it.
type manager struct {
	path  string
	width int
}

var (
	loadMu  sync.Mutex
	loaded  = map[manager]*api{}
	failure = map[manager]error{}
)

// loadManager loads the driver manager and returns its functions. A library
// is loaded once for the process for each path. It cannot be unloaded while a
// database driver that it loaded is in use. An empty path means the
// default names of the system.
func loadManager(path string, width int) (*api, error) {
	key := manager{path, width}
	loadMu.Lock()
	defer loadMu.Unlock()
	if a, ok := loaded[key]; ok {
		return a, nil
	}
	if err, ok := failure[key]; ok {
		return nil, err
	}
	a, err := openManager(path, width)
	if err != nil {
		failure[key] = err
		return nil, err
	}
	loaded[key] = a
	return a, nil
}

func openManager(path string, width int) (a *api, err error) {
	names := managerNames()
	if path != "" {
		names = []string{path}
	}
	var handle uintptr
	var errs []error
	for _, name := range names {
		var e error
		if handle, e = openLibrary(name); e == nil {
			break
		}
		errs = append(errs, e)
	}
	if handle == 0 {
		return nil, fmt.Errorf("loading the driver manager: %w", errors.Join(errs...))
	}
	defer func() {
		// purego panics when a function is missing
		if r := recover(); r != nil {
			a, err = nil, fmt.Errorf("loading the driver manager: %v", r)
		}
	}()
	a = new(api)
	for ptr, name := range map[any]string{
		&a.allocHandle:    "SQLAllocHandle",
		&a.freeHandle:     "SQLFreeHandle",
		&a.setEnvAttr:     "SQLSetEnvAttr",
		&a.setConnAttr:    "SQLSetConnectAttrW",
		&a.setConnAttrPtr: "SQLSetConnectAttrW",
		&a.setStmtAttr:    "SQLSetStmtAttrW",
		&a.getConnAttr:    "SQLGetConnectAttrW",
		&a.driverConnect:  "SQLDriverConnectW",
		&a.disconnect:     "SQLDisconnect",
		&a.execDirect:     "SQLExecDirectW",
		&a.prepare:        "SQLPrepareW",
		&a.execute:        "SQLExecute",
		&a.numResultCols:  "SQLNumResultCols",
		&a.describeCol:    "SQLDescribeColW",
		&a.colAttribute:   "SQLColAttributeW",
		&a.fetch:          "SQLFetch",
		&a.getData:        "SQLGetData",
		&a.rowCount:       "SQLRowCount",
		&a.moreResults:    "SQLMoreResults",
		&a.freeStmt:       "SQLFreeStmt",
		&a.cancel:         "SQLCancel",
		&a.endTran:        "SQLEndTran",
		&a.bindParameter:  "SQLBindParameter",
		&a.getInfo:        "SQLGetInfoW",
		&a.tables:         "SQLTablesW",
		&a.columns:        "SQLColumnsW",
		&a.primaryKeys:    "SQLPrimaryKeysW",
		&a.bindCol:        "SQLBindCol",
		&a.drivers:        "SQLDriversW",
		&a.dataSources:    "SQLDataSourcesW",
		&a.getDiagRec:     "SQLGetDiagRecW",
	} {
		purego.RegisterLibFunc(ptr, handle, name)
	}
	if err := a.setWidth(width); err != nil {
		return nil, err
	}
	return a, nil
}

// setWidth sets the size of SQLWCHAR. A width of 0 asks for it to be found, and
// the answer is 2 on Windows without a probe.
func (a *api) setWidth(width int) error {
	switch width {
	case 2, 4:
		a.wchar = width
		return nil
	case 0:
	default:
		return fmt.Errorf("the SQLWCHAR size %d is not 2 or 4", width)
	}
	if runtime.GOOS == "windows" {
		a.wchar = 2
		return nil
	}
	a.wchar = 2 // the probe reads only bytes, and a diagnostic needs no text
	w, err := a.probeWidth()
	if err != nil {
		return fmt.Errorf("finding the size of SQLWCHAR, which the wchar key sets: %w", err)
	}
	a.wchar = w
	return nil
}
