package odbc

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// state returns the SQLSTATE bytes that a manager with this width writes.
func state(width int, s string) []byte {
	out := make([]byte, 24)
	for i, r := range s {
		switch width {
		case 2:
			binary.NativeEndian.PutUint16(out[2*i:], uint16(r))
		case 4:
			binary.NativeEndian.PutUint32(out[4*i:], uint32(r))
		}
	}
	return out
}

func TestWidthOfState(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"HY092", "IM002", "01000", "08001", "HYC00"} {
		for _, width := range []int{2, 4} {
			got, ok := widthOfState(state(width, s))
			if !ok || got != width {
				t.Errorf("%s as %d bytes: got %d, %v", s, width, got, ok)
			}
		}
	}
	for name, b := range map[string][]byte{
		"zeros": make([]byte, 24),
		"short": make([]byte, 8),
		"text":  bytes.Repeat([]byte{'a'}, 24),
	} {
		if got, ok := widthOfState(b); ok {
			t.Errorf("%s: got %d, expected no answer", name, got)
		}
	}
}

func TestEncodeDecode(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "hello", "héllo ✓ 世界", "emoji 😀", "a\u0000b"} {
		for _, width := range []int{2, 4} {
			a := &api{wchar: width}
			enc := a.encode(s)
			if len(enc)%width != 0 || !bytes.Equal(enc[len(enc)-width:], make([]byte, width)) {
				t.Errorf("%q as %d bytes: no terminator in %x", s, width, enc)
			}
			// decode stops at the first zero
			want, _, _ := strings.Cut(s, "\x00")
			if got := a.decode(enc); got != want {
				t.Errorf("%q as %d bytes: got %q", s, width, got)
			}
		}
	}
}

// TestProbeOnTheSystemManager loads the driver manager of this machine and
// checks the width the probe finds. unixODBC and Windows are 2 bytes. Anything
// else is the answer of a manager this test does not know, and is logged.
func TestProbeOnTheSystemManager(t *testing.T) {
	t.Parallel()
	a, err := loadManager("", 0)
	if err != nil {
		t.Skipf("no driver manager: %v", err)
	}
	t.Logf("SQLWCHAR is %d bytes", a.wchar)
	if a.wchar != 2 && a.wchar != 4 {
		t.Errorf("the width is %d", a.wchar)
	}
}
