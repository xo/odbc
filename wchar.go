package odbc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
	"unsafe"
)

// SQLWCHAR is the character type of the wide functions. It is 2 bytes, as UTF-16,
// on Windows and in unixODBC unless that was built with SQL_WCHART_CONVERT. It
// is 4 bytes, as UTF-32, in iODBC and in that build. The driver finds out which
// when it loads the manager (D20), and every wide string passes through
// encode and decode, which know the width.

// encode converts a string to SQLWCHAR units with a terminating zero.
func (a *api) encode(s string) []byte {
	if a.wchar == 4 {
		out := make([]byte, 0, 4*(len(s)+1))
		for _, r := range s {
			out = binary.NativeEndian.AppendUint32(out, uint32(r))
		}
		return binary.NativeEndian.AppendUint32(out, 0)
	}
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2*(len(units)+1))
	for _, u := range units {
		out = binary.NativeEndian.AppendUint16(out, u)
	}
	return binary.NativeEndian.AppendUint16(out, 0)
}

// decode converts SQLWCHAR units to a string. It stops at the first zero unit.
func (a *api) decode(b []byte) string {
	if a.wchar == 4 {
		var rs []rune
		for ; len(b) >= 4; b = b[4:] {
			r := binary.NativeEndian.Uint32(b)
			if r == 0 {
				break
			}
			rs = append(rs, rune(r))
		}
		return string(rs)
	}
	var us []uint16
	for ; len(b) >= 2; b = b[2:] {
		u := binary.NativeEndian.Uint16(b)
		if u == 0 {
			break
		}
		us = append(us, u)
	}
	return string(utf16.Decode(us))
}

// units returns the number of SQLWCHAR units in n bytes.
func (a *api) units(n int) int { return n / a.wchar }

// widthOfState reads the SQLSTATE that SQLGetDiagRecW wrote and says how wide
// its characters are. A SQLSTATE is five characters of ASCII, so a 2 byte
// buffer holds the character in each pair of bytes, and a 4 byte buffer in each
// four. It reports false when the bytes fit neither or both.
func widthOfState(b []byte) (int, bool) {
	if len(b) < 20 {
		return 0, false
	}
	printable := func(c uint32) bool { return c >= 0x30 && c <= 0x7e }
	two := true
	for i := range 5 {
		two = two && printable(uint32(binary.NativeEndian.Uint16(b[2*i:])))
	}
	four := true
	for i := range 5 {
		four = four && printable(binary.NativeEndian.Uint32(b[4*i:]))
	}
	switch {
	case two && !four:
		return 2, true
	case four && !two:
		return 4, true
	}
	return 0, false
}

// probeWidth asks the manager for a diagnostic that it makes itself, an
// invalid attribute of the environment, and reads the width from its SQLSTATE.
// The state buffer has room for six characters of 4 bytes, because
// SQLGetDiagRecW takes no length for it and a 4 byte manager writes that much.
func (a *api) probeWidth() (int, error) {
	var env uintptr
	if ret := a.allocHandle(handleEnv, 0, &env); ret != sqlSuccess && ret != sqlSuccessWithInfo {
		return 0, fmt.Errorf("allocating the environment: return code %d", ret)
	}
	defer a.freeHandle(handleEnv, env)
	if a.setEnvAttr(env, probeAttribute, 0, 0) == sqlSuccess {
		return 0, errors.New("the manager accepted an invalid attribute")
	}
	var (
		state  [24]byte
		msg    [512]byte
		native int32
		n      int16
	)
	ret := a.getDiagRec(handleEnv, env, 1, unsafe.Pointer(&state[0]), &native, unsafe.Pointer(&msg[0]), 128, &n)
	if ret != sqlSuccess && ret != sqlSuccessWithInfo {
		return 0, errors.New("the manager recorded no diagnostic")
	}
	w, ok := widthOfState(state[:])
	if !ok {
		return 0, fmt.Errorf("the SQLSTATE %x is neither UTF-16 nor UTF-32", state[:])
	}
	return w, nil
}
