//go:build !windows

package odbc

import (
	"fmt"
	"runtime"

	"github.com/ebitengine/purego"
)

// managerNames are the names of the driver manager, in the order tried.
func managerNames() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"libodbc.2.dylib", "libodbc.dylib", "/opt/homebrew/lib/libodbc.2.dylib", "/usr/local/lib/libodbc.2.dylib", "/opt/local/lib/libodbc.2.dylib"}
	default:
		return []string{"libodbc.so.2", "libodbc.so"}
	}
}

// openLibrary opens the shared library at path or with the given name.
func openLibrary(path string) (uintptr, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", path, err)
	}
	return h, nil
}
