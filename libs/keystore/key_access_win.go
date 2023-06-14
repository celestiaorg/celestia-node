//go:build windows

package keystore

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// keyAccess checks whether file is not accessible by all users.
func keyAccess(path string) error {

	// Check the file's DACL
	securityDescriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("Error getting file's DACL: %v\n", err)
	}
	defer func() {
		if err != nil {
			_, _ = windows.LocalFree((windows.Handle)(unsafe.Pointer(securityDescriptor)))
		}
	}()
	_, _, err = securityDescriptor.DACL()
	if err != nil {
		return fmt.Errorf("File is too Open with no DACL: %v\n", err)
	}

	return nil
}
