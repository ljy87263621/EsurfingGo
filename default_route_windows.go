//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// defaultRouteInterfaceIndices returns interfaces that own an actual IPv4
// default route. Checking the route table avoids treating a hotspot or a
// virtual adapter's gateway metadata as the system's default path.
func defaultRouteInterfaceIndices() (map[int]bool, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_INET, &table); err != nil {
		return nil, fmt.Errorf("GetIpForwardTable2 failed: %w", err)
	}
	if table == nil {
		return map[int]bool{}, nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	result := make(map[int]bool)
	for _, row := range table.Rows() {
		if isIPv4DefaultRoute(&row) {
			result[int(row.InterfaceIndex)] = true
		}
	}
	return result, nil
}

func isIPv4DefaultRoute(row *windows.MibIpForwardRow2) bool {
	return row != nil &&
		row.InterfaceIndex != 0 &&
		row.DestinationPrefix.Prefix.Family == windows.AF_INET &&
		row.DestinationPrefix.PrefixLength == 0
}
