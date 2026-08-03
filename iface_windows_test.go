//go:build windows

package main

import "testing"

func TestInterfaceIndexSocketValueUsesWindowsNetworkByteOrder(t *testing.T) {
	for _, test := range []struct {
		index int
		want  int
	}{
		{index: 1, want: 0x01000000},
		{index: 14, want: 0x0e000000},
	} {
		if got := interfaceIndexSocketValue(test.index); got != test.want {
			t.Fatalf("interfaceIndexSocketValue(%d) = %#x, want %#x", test.index, got, test.want)
		}
	}
}
