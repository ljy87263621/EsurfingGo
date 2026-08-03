//go:build !windows

package main

import "net"

func configureDialerForInterface(_ *net.Dialer, _ int) error {
	return nil
}
