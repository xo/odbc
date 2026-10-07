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

// ConnectionError reports whether the SQLSTATE is in class 08, a failure of
// the connection.
func (e *Error) ConnectionError() bool {
	return strings.HasPrefix(e.State, "08")
}

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
