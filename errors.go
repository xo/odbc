package odbc

import (
	"fmt"
	"strings"
	"unsafe"
)

// Error is a diagnostic record of the driver manager. It is the error that a
// failed call returns, so a caller finds the SQLSTATE with errors.As.
type Error struct {
	// Op names the call that failed.
	Op string
	// State is the five character SQLSTATE.
	State string
	// Native is the code of the database.
	Native int32
	// Message is the text of the database.
	Message string
	// Next holds the further records of the same call.
	Next []Error
}

// Error implements error.
func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (%s", e.Op, e.Message, e.State)
	if e.Native != 0 {
		fmt.Fprintf(&b, ", native %d", e.Native)
	}
	b.WriteByte(')')
	return b.String()
}

// Unwrap returns the next record, so errors.Is and As reach every one.
func (e *Error) Unwrap() []error {
	out := make([]error, len(e.Next))
	for i := range e.Next {
		out[i] = &e.Next[i]
	}
	return out
}

// ClassError is a class of SQLSTATE, which is the first two characters of the state,
// or three for a timeout. errors.Is(err, odbc.ErrIntegrity) is true for an
// Error whose state is in that class. A SQLSTATE class is the same in every
// database, so a caller can test for it without knowing the database (D23).
type ClassError string

// Error implements error.
func (c ClassError) Error() string { return "odbc: SQLSTATE class " + string(c) }

// The classes that a caller tests for most often.
const (
	// ErrConnection is a failure of the connection (08).
	ErrConnection ClassError = "08"
	// ErrData is a value the database cannot take, such as an overflow (22).
	ErrData ClassError = "22"
	// ErrIntegrity is a violation of a constraint, such as a duplicate key (23).
	ErrIntegrity ClassError = "23"
	// ErrRollback is a transaction that the database rolled back, such as a
	// deadlock or a serialization failure (40).
	ErrRollback ClassError = "40"
	// ErrSyntax is a syntax error or a refused access (42).
	ErrSyntax ClassError = "42"
	// ErrTimeout is a timeout, of a statement or of a login (HYT).
	ErrTimeout ClassError = "HYT"
)

// Is reports whether target is a ClassError that the state of e is in.
func (e *Error) Is(target error) bool {
	c, ok := target.(ClassError)
	return ok && strings.HasPrefix(e.State, string(c))
}

// Native codes of a login that failed for a reason that a new password does not
// cure: a locked account, an expired password and a password that must change.
// SQL Server sends 18486, 18487 and 18488 with the state 28000. MySQL and
// MariaDB send 3118 and 1862.
var noRetryNative = map[int32]bool{18486: true, 18487: true, 18488: true, 3118: true, 1862: true}

// AuthenticationRefused reports whether the database refused the credential in
// a way that a new password can cure. That means a wrong password, a user that
// the database does not know, or a password that the database needs and the
// client did not send. It looks at this record and at the records of the same
// call. A client such as usql asks every error of a tree for this method, and
// takes a true from any (D25).
//
// It is false for a refusal for lack of a permission, for a locked account or
// an expired password, and for a failure of the network, of TLS or of a timeout.
func (e *Error) AuthenticationRefused() bool {
	if e.refusedHere() {
		return true
	}
	for i := range e.Next {
		if e.Next[i].refusedHere() {
			return true
		}
	}
	return false
}

// refusedHere reports whether this record alone says that the credential was
// refused. SQLSTATE class 28 is "invalid authorization specification". MariaDB,
// MySQL and SQL Server answer a refused login with 28000.
//
// psqlODBC answers 08001, the class of a failed connection, with the text of the
// server. For it the text decides. libpq says "authentication failed" for a
// refused password. It says "no password supplied" when the server asks and the
// client has none. A connection that failed for another reason has neither text.
// The state 08004 is not counted, because a server sends it for other reasons
// too.
func (e *Error) refusedHere() bool {
	if strings.HasPrefix(e.State, "28") {
		return !noRetryNative[e.Native]
	}
	if e.State != "08001" {
		return false
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "authentication failed") || strings.Contains(msg, "no password supplied")
}

// ConnectionError reports whether the SQLSTATE is in class 08, a failure of
// the connection.
func (e *Error) ConnectionError() bool {
	return strings.HasPrefix(e.State, "08")
}

// ErrTruncated is the error of a value that does not fit the buffer of its column
// when the rows are fetched in blocks (D24). The driver reports it and never
// returns a value that was cut short. Read the result without WithFetchSize.
const ErrTruncated = closedError("odbc: a value was cut short")

// errClosed is returned when a handle is used after it was freed.
const errClosed = closedError("odbc: use of a closed handle")

type closedError string

func (e closedError) Error() string { return string(e) }

// check turns a return code into an error. A success, with or without
// information, returns nil.
func (a *api) check(op string, ret int16, handleType int16, handle uintptr) error {
	if ret == sqlSuccess || ret == sqlSuccessWithInfo {
		return nil
	}
	return a.diag(op, ret, handleType, handle)
}

// diag reads the diagnostic records of a handle.
func (a *api) diag(op string, ret int16, handleType int16, handle uintptr) error {
	var out []Error
	for rec := int16(1); rec < 16; rec++ {
		// the state buffer holds six units and the call takes no length for it
		state := make([]byte, 6*a.wchar)
		var native int32
		msg := make([]byte, 1024*a.wchar)
		var n int16
		r := a.getDiagRec(handleType, handle, rec, unsafe.Pointer(&state[0]), &native, unsafe.Pointer(&msg[0]), int16(a.units(len(msg))), &n)
		if r != sqlSuccess && r != sqlSuccessWithInfo {
			break
		}
		out = append(out, Error{
			Op:      op,
			State:   a.decode(state[:5*a.wchar]),
			Native:  native,
			Message: a.decode(msg[:min(int(n), a.units(len(msg))-1)*a.wchar]),
		})
	}
	if len(out) == 0 {
		return fmt.Errorf("%s: return code %d with no diagnostic", op, ret)
	}
	first := out[0]
	first.Next = out[1:]
	return &first
}
