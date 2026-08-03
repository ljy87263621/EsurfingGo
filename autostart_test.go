package main

import "testing"

func TestAutostartCommandQuotesExecutablePath(t *testing.T) {
	got := autostartCommand(`C:\Program Files\Esurfing Go\esurfing.exe`)
	want := `"C:\Program Files\Esurfing Go\esurfing.exe" -autostart`
	if got != want {
		t.Fatalf("autostartCommand() = %q, want %q", got, want)
	}
}

func TestAutostartCommandRejectsEmptyExecutablePath(t *testing.T) {
	if got := autostartCommand(""); got != "" {
		t.Fatalf("autostartCommand(\"\") = %q, want empty command", got)
	}
}

func TestAutostartMatchesOnlyTheSameExecutable(t *testing.T) {
	executable := `C:\Program Files\Esurfing Go\esurfing.exe`
	command := autostartCommand(executable)

	if !autostartMatchesExecutable(command, executable) {
		t.Fatal("matching startup command should be recognized")
	}
	if !autostartMatchesExecutable(`"c:\program files\esurfing go\esurfing.exe" -autostart`, executable) {
		t.Fatal("startup command comparison should be case-insensitive")
	}
	if autostartMatchesExecutable(autostartCommand(`C:\Other\esurfing.exe`), executable) {
		t.Fatal("startup command for another executable should not match")
	}
	if autostartMatchesExecutable("", executable) {
		t.Fatal("empty startup command should not match")
	}
}
