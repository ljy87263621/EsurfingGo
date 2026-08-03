package main

import "strings"

const (
	autostartRegistryKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartRegistryValue = "EsurfingGo"
)

func autostartCommand(executablePath string) string {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(executablePath, `"`, `\"`) + `" -autostart`
}

func autostartMatchesExecutable(command, executablePath string) bool {
	command = strings.TrimSpace(command)
	return command != "" && strings.EqualFold(command, autostartCommand(executablePath))
}
