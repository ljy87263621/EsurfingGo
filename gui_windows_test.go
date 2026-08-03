//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestGUIMessageMatchesWin32MSGSize(t *testing.T) {
	want := uintptr(48)
	if runtime.GOARCH == "386" {
		want = 32
	}
	if got := unsafe.Sizeof(guiMsg{}); got != want {
		t.Fatalf("guiMsg size = %d, want Win32 MSG size %d", got, want)
	}
}

func TestGUINotifyIconDataMatchesWin32Layout(t *testing.T) {
	want := uintptr(976)
	if runtime.GOARCH == "386" {
		want = 956
	}
	if got := unsafe.Sizeof(guiNotifyIconData{}); got != want {
		t.Fatalf("guiNotifyIconData size = %d, want Win32 NOTIFYICONDATAW size %d", got, want)
	}
}

func TestGUITrayEventsRestoreOnlyForClickEvents(t *testing.T) {
	if !guiTrayEventRestoresWindow(guiWMLButtonUp) {
		t.Fatal("left click should restore the window")
	}
	if !guiTrayEventRestoresWindow(guiWMLButtonDoubleClick) {
		t.Fatal("left double click should restore the window")
	}
	if guiTrayEventRestoresWindow(guiWMRButtonUp) {
		t.Fatal("right click should open the tray menu, not restore directly")
	}
}
