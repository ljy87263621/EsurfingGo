package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NetworkInterface represents an available network interface with a 1-based index.
type NetworkInterface struct {
	Index int    // 1-based index for user display
	Name  string // OS-reported interface name (e.g. "以太网", "WLAN", "eth0")
}

// ListNetworkInterfaces returns all network interfaces that are up and non-loopback.
// Uses Go's standard net.Interfaces() which works across Windows, Linux, and macOS.
func ListNetworkInterfaces() ([]NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var result []NetworkInterface
	idx := 1
	for _, iface := range ifaces {
		// Skip loopback interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Skip interfaces that are not up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		result = append(result, NetworkInterface{
			Index: idx,
			Name:  iface.Name,
		})
		idx++
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no available network interfaces found")
	}

	return result, nil
}

// PrintNetworkInterfaces prints all available interfaces in the format "1：name".
func PrintNetworkInterfaces(ifaces []NetworkInterface) {
	for _, iface := range ifaces {
		fmt.Printf("%d：%s\n", iface.Index, iface.Name)
	}
}

// GetNetworkInterfaceByIndex returns the interface matching the given 1-based index.
// Returns an error if the index is out of range.
func GetNetworkInterfaceByIndex(ifaces []NetworkInterface, index int) (*NetworkInterface, error) {
	for i := range ifaces {
		if ifaces[i].Index == index {
			return &ifaces[i], nil
		}
	}
	return nil, fmt.Errorf("invalid interface number: %d (valid range: 1-%d)", index, len(ifaces))
}

// NewBoundHTTPTransport creates an http.Transport whose TCP connections are bound
// to the IPv4 address of the specified network interface. This ensures all HTTP
// traffic goes through the chosen interface instead of the OS default route.
// Works cross-platform (Windows, Linux, macOS) via net.Dialer.LocalAddr binding.
func NewBoundHTTPTransport(iface *NetworkInterface) (*http.Transport, error) {
	// Look up the OS-level interface by name
	netIface, err := net.InterfaceByName(iface.Name)
	if err != nil {
		return nil, fmt.Errorf("interface %q not found: %w", iface.Name, err)
	}

	addrs, err := netIface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %q: %w", iface.Name, err)
	}

	// Find the first IPv4 address on this interface
	var localIP net.IP
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && ip.To4() != nil {
			localIP = ip
			break
		}
	}

	if localIP == nil {
		return nil, fmt.Errorf("no IPv4 address found on interface %q (is it connected?)", iface.Name)
	}

	// Create a dialer that binds to this interface's IP
	localAddr := &net.TCPAddr{IP: localIP}
	dialer := &net.Dialer{
		LocalAddr: localAddr,
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return transport, nil
}
