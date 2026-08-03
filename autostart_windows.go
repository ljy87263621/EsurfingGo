//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser      uintptr = 0x80000001
	keyQueryValue        uintptr = 0x0001
	keySetValue          uintptr = 0x0002
	regSZ                uint32  = 1
	regOptionNonVolatile uint32  = 0
	regErrorFileNotFound uintptr = 2
)

var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

func setAutostart(executablePath string, enabled bool) error {
	if strings.TrimSpace(executablePath) == "" {
		return fmt.Errorf("executable path is empty")
	}
	keyPath, _ := syscall.UTF16PtrFromString(autostartRegistryKey)
	valueName, _ := syscall.UTF16PtrFromString(autostartRegistryValue)
	var key uintptr
	if enabled {
		result, _, _ := procRegCreateKeyExW.Call(
			hkeyCurrentUser,
			uintptr(unsafe.Pointer(keyPath)),
			0,
			0,
			uintptr(regOptionNonVolatile),
			keySetValue,
			0,
			uintptr(unsafe.Pointer(&key)),
			0,
		)
		if result != 0 {
			return fmt.Errorf("create startup registry key: %w", syscall.Errno(result))
		}
		defer procRegCloseKey.Call(key)
		command := syscall.StringToUTF16(autostartCommand(executablePath))
		result, _, _ = procRegSetValueExW.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(regSZ), uintptr(unsafe.Pointer(&command[0])), uintptr(len(command)*2))
		if result != 0 {
			return fmt.Errorf("set startup registry value: %w", syscall.Errno(result))
		}
		return nil
	}

	result, _, _ := procRegOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, keyQueryValue|keySetValue, uintptr(unsafe.Pointer(&key)))
	if result == regErrorFileNotFound {
		return nil
	}
	if result != 0 {
		return fmt.Errorf("open startup registry key: %w", syscall.Errno(result))
	}
	defer procRegCloseKey.Call(key)
	currentCommand, err := queryAutostartCommand(key, valueName)
	if err != nil {
		return err
	}
	if !autostartMatchesExecutable(currentCommand, executablePath) {
		return nil
	}
	result, _, _ = procRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(valueName)))
	if result != 0 && result != regErrorFileNotFound {
		return fmt.Errorf("delete startup registry value: %w", syscall.Errno(result))
	}
	return nil
}

func isAutostartEnabled(executablePath string) (bool, error) {
	if strings.TrimSpace(executablePath) == "" {
		return false, fmt.Errorf("executable path is empty")
	}
	keyPath, _ := syscall.UTF16PtrFromString(autostartRegistryKey)
	valueName, _ := syscall.UTF16PtrFromString(autostartRegistryValue)
	var key uintptr
	result, _, _ := procRegOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(keyPath)), 0, keyQueryValue, uintptr(unsafe.Pointer(&key)))
	if result == regErrorFileNotFound {
		return false, nil
	}
	if result != 0 {
		return false, fmt.Errorf("open startup registry key: %w", syscall.Errno(result))
	}
	defer procRegCloseKey.Call(key)

	command, err := queryAutostartCommand(key, valueName)
	if err != nil {
		return false, err
	}
	return autostartMatchesExecutable(command, executablePath), nil
}

func queryAutostartCommand(key uintptr, valueName *uint16) (string, error) {
	var valueType uint32
	var byteLength uint32
	result, _, _ := procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&valueType)), 0, uintptr(unsafe.Pointer(&byteLength)))
	if result == regErrorFileNotFound {
		return "", nil
	}
	if result != 0 {
		return "", fmt.Errorf("query startup registry value size: %w", syscall.Errno(result))
	}
	if valueType != regSZ || byteLength == 0 {
		return "", nil
	}
	buffer := make([]uint16, (byteLength+1)/2)
	result, _, _ = procRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&valueType)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&byteLength)))
	if result != 0 {
		return "", fmt.Errorf("query startup registry value: %w", syscall.Errno(result))
	}
	return strings.TrimSpace(syscall.UTF16ToString(buffer)), nil
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return path, nil
}
