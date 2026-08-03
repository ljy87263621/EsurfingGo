//go:build !windows

package main

import "errors"

func setAutostart(string, bool) error {
	return errors.New("Windows startup is available on Windows only")
}

func isAutostartEnabled(string) (bool, error) {
	return false, errors.New("Windows startup is available on Windows only")
}
