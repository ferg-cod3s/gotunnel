//go:build windows

package tunnel

import (
	"syscall"
)

func setSocketOptions(network, address string, c syscall.RawConn) error {
	var opErr error
	if err := c.Control(func(fd uintptr) {
		handle := syscall.Handle(fd)
		opErr = syscall.SetsockoptInt(handle, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}); err != nil {
		return err
	}
	return opErr
}
