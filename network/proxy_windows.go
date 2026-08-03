//go:build windows

package network

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"syscall"
	"unsafe"
)

const (
	proxyHKEYCurrentUser = 0x80000001
	proxyKeyQueryValue   = 0x0001
	proxyRegDWord        = 4
	proxyRegSZ           = 1
)

var (
	proxyAdvapi32         = syscall.NewLazyDLL("advapi32.dll")
	proxyRegOpenKeyExW    = proxyAdvapi32.NewProc("RegOpenKeyExW")
	proxyRegQueryValueExW = proxyAdvapi32.NewProc("RegQueryValueExW")
	proxyRegCloseKey      = proxyAdvapi32.NewProc("RegCloseKey")
)

func systemProxyURL() (*url.URL, error) {
	keyPath, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var key syscall.Handle
	result, _, _ := proxyRegOpenKeyExW.Call(
		proxyHKEYCurrentUser,
		uintptr(unsafe.Pointer(keyPath)),
		0,
		proxyKeyQueryValue,
		uintptr(unsafe.Pointer(&key)),
	)
	if result != 0 {
		return nil, syscall.Errno(result)
	}
	defer proxyRegCloseKey.Call(uintptr(key))

	enabled, err := readProxyDWord(key, "ProxyEnable")
	if err != nil {
		return nil, err
	}
	if enabled == 0 {
		return nil, nil
	}
	raw, err := readProxyString(key, "ProxyServer")
	if err != nil {
		return nil, err
	}
	return parseConfiguredProxy(raw)
}

func readProxyDWord(key syscall.Handle, name string) (uint32, error) {
	typeValue, data, err := readProxyValue(key, name)
	if err != nil {
		return 0, err
	}
	if typeValue != proxyRegDWord || len(data) < 4 {
		return 0, fmt.Errorf("registry value %s is not a DWORD", name)
	}
	return binary.LittleEndian.Uint32(data[:4]), nil
}

func readProxyString(key syscall.Handle, name string) (string, error) {
	typeValue, data, err := readProxyValue(key, name)
	if err != nil {
		return "", err
	}
	if typeValue != proxyRegSZ {
		return "", fmt.Errorf("registry value %s is not a string", name)
	}
	if len(data) == 0 {
		return "", nil
	}
	values := unsafe.Slice((*uint16)(unsafe.Pointer(&data[0])), len(data)/2)
	return syscall.UTF16ToString(values), nil
}

func readProxyValue(key syscall.Handle, name string) (uint32, []byte, error) {
	valueName, _ := syscall.UTF16PtrFromString(name)
	var typeValue uint32
	var size uint32
	result, _, _ := proxyRegQueryValueExW.Call(
		uintptr(key),
		uintptr(unsafe.Pointer(valueName)),
		0,
		uintptr(unsafe.Pointer(&typeValue)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if result != 0 {
		return 0, nil, syscall.Errno(result)
	}
	data := make([]byte, size)
	var dataPtr *byte
	if len(data) > 0 {
		dataPtr = &data[0]
	}
	result, _, _ = proxyRegQueryValueExW.Call(
		uintptr(key),
		uintptr(unsafe.Pointer(valueName)),
		0,
		uintptr(unsafe.Pointer(&typeValue)),
		uintptr(unsafe.Pointer(dataPtr)),
		uintptr(unsafe.Pointer(&size)),
	)
	if result != 0 {
		return 0, nil, syscall.Errno(result)
	}
	return typeValue, data[:size], nil
}
