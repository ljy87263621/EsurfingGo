//go:build !windows

package main

func defaultRouteInterfaceIndices() (map[int]bool, error) {
	// Non-Windows builds keep the existing stable interface enumeration. The
	// Windows implementation uses the native IP Helper API for route metadata.
	return nil, nil
}
