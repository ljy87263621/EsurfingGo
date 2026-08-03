//go:build windows

package main

import (
	"fmt"
	"net"
	"syscall"
)

const windowsIPUnicastIf = 31

func configureDialerForInterface(dialer *net.Dialer, interfaceIndex int) error {
	if interfaceIndex <= 0 {
		return fmt.Errorf("invalid interface index: %d", interfaceIndex)
	}

	dialer.Control = func(_, _ string, rawConn syscall.RawConn) error {
		var socketErr error
		if err := rawConn.Control(func(fd uintptr) {
			socketErr = syscall.SetsockoptInt(
				syscall.Handle(fd),
				syscall.IPPROTO_IP,
				windowsIPUnicastIf,
				interfaceIndexSocketValue(interfaceIndex),
			)
		}); err != nil {
			return err
		}
		return socketErr
	}
	return nil
}

// Windows expects the IPv4 interface index in network byte order for
// IP_UNICAST_IF, even though setsockopt receives a host integer.
func interfaceIndexSocketValue(interfaceIndex int) int {
	value := uint32(interfaceIndex)
	return int((value&0x000000ff)<<24 | (value&0x0000ff00)<<8 | (value&0x00ff0000)>>8 | (value&0xff000000)>>24)
}
