package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// NetworkInterface represents an available network interface with a 1-based index.
type NetworkInterface struct {
	Index int    // 1-based index for user display
	Name  string // OS-reported interface name (e.g. "以太网", "WLAN", "eth0")
}

// ListNetworkInterfaces returns usable IPv4 interfaces that are up and non-loopback.
// Known tunnel and virtual adapters are excluded because they cannot carry the
// campus authentication traffic when a system-wide TUN route is active.
func ListNetworkInterfaces() ([]NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var result []NetworkInterface
	idx := 1
	for _, iface := range ifaces {
		if !shouldListNetworkInterface(iface, interfaceHasIPv4(iface)) {
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

func shouldListNetworkInterface(iface net.Interface, hasIPv4 bool) bool {
	if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
		return false
	}
	if iface.Flags&net.FlagPointToPoint != 0 || !hasIPv4 {
		return false
	}
	return !isLikelyVirtualInterfaceName(iface.Name)
}

func interfaceHasIPv4(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip != nil && ip.To4() != nil {
			return true
		}
	}
	return false
}

func isLikelyVirtualInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{
		"clash",
		"mihomo",
		"wintun",
		"tun",
		"tap",
		"wireguard",
		"tailscale",
		"zerotier",
		"npcap",
		"loopback",
		"virtual",
		"miniport",
		"vethernet",
		"hyper-v",
		"hyperv",
		"wsl",
		"本地连接*",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func hasLikelyTunnelInterface() (bool, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false, fmt.Errorf("failed to inspect network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if isActiveLikelyTunnelInterface(iface.Name, iface.Flags, interfaceHasIPv4(iface)) {
			return true, nil
		}
	}
	return false, nil
}

func isActiveLikelyTunnelInterface(name string, flags net.Flags, hasIPv4 bool) bool {
	if flags&net.FlagLoopback != 0 || flags&net.FlagUp == 0 || !hasIPv4 {
		return false
	}
	return isLikelyTunnelInterfaceName(name)
}

func isLikelyTunnelInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{
		"clash",
		"mihomo",
		"wintun",
		"tun",
		"tap",
		"wireguard",
		"tailscale",
		"zerotier",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

// NewTUNSafeHTTPTransport returns a direct transport bound to the first usable
// physical interface when a likely system TUN adapter is present. A nil
// transport means normal OS routing is safe to keep using.
func NewTUNSafeHTTPTransport() (*http.Transport, string, error) {
	active, err := hasLikelyTunnelInterface()
	if err != nil || !active {
		return nil, "", err
	}

	ifaces, err := ListNetworkInterfaces()
	if err != nil {
		return nil, "", err
	}
	for i := range ifaces {
		transport, bindErr := NewBoundHTTPTransport(&ifaces[i])
		if bindErr == nil {
			return transport, ifaces[i].Name, nil
		}
	}
	return nil, "", fmt.Errorf("TUN is active but no usable physical IPv4 interface was found")
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
	if err := configureDialerForInterface(dialer, netIface.Index); err != nil {
		return nil, fmt.Errorf("failed to bind socket to interface %q: %w", iface.Name, err)
	}

	// Windows' native resolver can send DNS through a TUN adapter even when
	// the eventual TCP connection is bound to the physical interface. Use the
	// Go resolver and bind its UDP/TCP DNS sockets to the same interface.
	dnsUDPDialer := &net.Dialer{
		LocalAddr: &net.UDPAddr{IP: localIP},
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if err := configureDialerForInterface(dnsUDPDialer, netIface.Index); err != nil {
		return nil, fmt.Errorf("failed to bind DNS socket to interface %q: %w", iface.Name, err)
	}
	dnsTCPDialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: localIP},
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if err := configureDialerForInterface(dnsTCPDialer, netIface.Index); err != nil {
		return nil, fmt.Errorf("failed to bind DNS TCP socket to interface %q: %w", iface.Name, err)
	}
	dialer.Resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network == "tcp" || network == "tcp4" {
				return dnsTCPDialer.DialContext(ctx, "tcp4", address)
			}
			return dnsUDPDialer.DialContext(ctx, "udp4", address)
		},
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

func boundResolverNetwork(network string) string {
	if network == "udp" {
		return "udp4"
	}
	return network
}
