//go:build windows

package ownership

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procWaitNamedPipeW = kernel32.NewProc("WaitNamedPipeW")
	procPeekNamedPipe  = kernel32.NewProc("PeekNamedPipe")
)

func waitNamedPipe(name *uint16, timeout uint32) error {
	ok, _, callErr := procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(name)), uintptr(timeout))
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.EINVAL
	}
	return nil
}

func pipeHasData(handle windows.Handle) (bool, error) {
	var available uint32
	ok, _, callErr := procPeekNamedPipe.Call(
		uintptr(handle),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&available)),
		0,
	)
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return false, callErr
		}
		return false, syscall.EINVAL
	}
	return available != 0, nil
}
