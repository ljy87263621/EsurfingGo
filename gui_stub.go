//go:build !windows

package main

import "errors"

func runGUI(bool) error {
	return errors.New("GUI is available on Windows only")
}
