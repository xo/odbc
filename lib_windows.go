//go:build windows

package odbc

import (
	"fmt"
	"syscall"
)

// managerNames are the names of the driver manager, in the order tried.
func managerNames() []string {
	return []string{"odbc32.dll"}
}

// openLibrary opens the DLL at path or with the given name.
func openLibrary(path string) (uintptr, error) {
	h, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", path, err)
	}
	return uintptr(h), nil
}
