package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// NetworkInterface represents an available network interface with a 1-based index.
type NetworkInterface struct {
	Index       int    // 1-based index for user display
	Name        string // OS-reported interface name (e.g. "以太网", "WLAN", "eth0")
	SystemIndex int    // OS interface index used for route and socket binding
}

type interfaceBindingMode uint8

const (
	interfaceBindingSourceAddress interfaceBindingMode = iota
	interfaceBindingForcedRoute
)

// ListNetworkInterfaces returns usable IPv4 interfaces that are up and non-loopback.
// Known tunnel and virtual adapters are excluded because they cannot carry the
// campus authentication traffic when a system-wide TUN route is active.
func ListNetworkInterfaces() ([]NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}
	defaultRoutes, _ := defaultRouteInterfaceIndices()

	var result []NetworkInterface
	idx := 1
	for _, iface := range ifaces {
		addresses := interfaceIPv4Addresses(iface)
		if !shouldListNetworkInterfaceWithAddresses(iface, hasUsableIPv4(addresses), addresses) {
			continue
		}
		result = append(result, NetworkInterface{
			Index:       idx,
			Name:        iface.Name,
			SystemIndex: iface.Index,
		})
		idx++
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no available network interfaces found")
	}

	return prioritizeNetworkInterfacesByDefaultRoute(result, defaultRoutes), nil
}

// prioritizeNetworkInterfacesByDefaultRoute keeps the stable interface order
// while moving interfaces that currently own an IPv4 default route first. The
// route lookup is best effort so a platform API failure does not hide usable
// interfaces or change the old fallback behavior.
func prioritizeNetworkInterfacesByDefaultRoute(ifaces []NetworkInterface, defaultRoutes map[int]bool) []NetworkInterface {
	ordered := make([]NetworkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if defaultRoutes[iface.SystemIndex] {
			ordered = append(ordered, iface)
		}
	}
	for _, iface := range ifaces {
		if !defaultRoutes[iface.SystemIndex] {
			ordered = append(ordered, iface)
		}
	}
	for index := range ordered {
		ordered[index].Index = index + 1
	}
	return ordered
}

func shouldListNetworkInterface(iface net.Interface, hasIPv4 bool) bool {
	return shouldListNetworkInterfaceWithAddresses(iface, hasIPv4, nil)
}

func shouldListNetworkInterfaceWithAddresses(iface net.Interface, hasIPv4 bool, addresses []net.IP) bool {
	if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
		return false
	}
	if iface.Flags&net.FlagPointToPoint != 0 || !hasIPv4 {
		return false
	}
	if isLikelyVirtualInterfaceName(iface.Name) {
		return false
	}
	return !isActiveLikelyHotspotInterfaceWithAddresses(iface.Name, iface.Flags, addresses)
}

func interfaceHasIPv4(iface net.Interface) bool {
	return hasUsableIPv4(interfaceIPv4Addresses(iface))
}

func interfaceIPv4Addresses(iface net.Interface) []net.IP {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	return ipv4AddressesFromAddrs(addrs)
}

func ipv4AddressesFromAddrs(addrs []net.Addr) []net.IP {
	addresses := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip = ip.To4(); ip != nil {
			addresses = append(addresses, ip)
		}
	}
	return addresses
}

func hasUsableIPv4(addresses []net.IP) bool {
	for _, ip := range addresses {
		if isUsableIPv4(ip) {
			return true
		}
	}
	return false
}

func isUsableIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast()
}

func isLikelyVirtualInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if isLikelyHotspotInterfaceName(name) {
		return true
	}
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
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func isLikelyHotspotInterfaceName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, marker := range []string{
		"mobile hotspot",
		"hotspot",
		"hosted network",
		"wi-fi direct",
		"wifi direct",
		"local area connection*",
		"local connection*",
		"本地连接*",
		"移动热点",
		"托管网络",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func hasLikelyTunnelInterface() (bool, error) {
	status, err := inspectNetworkCompatibility()
	return status.tunActive, err
}

func isActiveLikelyTunnelInterface(name string, flags net.Flags, hasIPv4 bool) bool {
	if flags&net.FlagLoopback != 0 || flags&net.FlagUp == 0 || !hasIPv4 {
		return false
	}
	return isLikelyTunnelInterfaceName(name)
}

func hasLikelyHotspotInterface() (bool, error) {
	status, err := inspectNetworkCompatibility()
	return status.hotspotActive, err
}

func isActiveLikelyHotspotInterface(name string, flags net.Flags, hasIPv4 bool) bool {
	if flags&net.FlagLoopback != 0 || flags&net.FlagUp == 0 || !hasIPv4 {
		return false
	}
	return isLikelyHotspotInterfaceName(name)
}

func isActiveLikelyHotspotInterfaceWithAddresses(name string, flags net.Flags, addresses []net.IP) bool {
	if flags&net.FlagLoopback != 0 || flags&net.FlagUp == 0 || !hasUsableIPv4(addresses) {
		return false
	}
	if isLikelyHotspotInterfaceName(name) {
		return true
	}
	for _, ip := range addresses {
		if isLikelyWindowsICSAddress(ip) {
			return true
		}
	}
	return false
}

// Windows ICS commonly assigns one of these gateway addresses to a mobile
// hotspot adapter. Address detection complements the localized adapter-name
// checks because users and OEM tools can rename the adapter.
func isLikelyWindowsICSAddress(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	return ip.Equal(net.IPv4(192, 168, 137, 1)) || ip.Equal(net.IPv4(192, 168, 5, 1))
}

type networkCompatibilityStatus struct {
	tunActive     bool
	hotspotActive bool
	hotspotCIDRs  []string
}

func (s networkCompatibilityStatus) hotspotExclusionHint() string {
	if !s.tunActive || !s.hotspotActive {
		return ""
	}
	if len(s.hotspotCIDRs) == 0 {
		return "TUN+热点：仅本程序认证流量已隔离；热点客户端转发仍由 Clash/Windows ICS 负责"
	}
	return "TUN+热点：仅本程序认证流量已隔离；热点客户端转发仍由 Clash/Windows ICS 负责（检测到热点子网：" + strings.Join(s.hotspotCIDRs, ", ") + "，仅供诊断）"
}

func logNetworkCompatibility(prefix string) {
	status, err := inspectNetworkCompatibility()
	if err != nil {
		log.Printf("%s network compatibility inspection unavailable: %v", prefix, err)
		return
	}
	if message := networkCompatibilityLogMessage(prefix, status); message != "" {
		log.Print(message)
	}
}

func networkCompatibilityLogMessage(prefix string, status networkCompatibilityStatus) string {
	if !status.tunActive || !status.hotspotActive {
		return ""
	}
	if len(status.hotspotCIDRs) > 0 {
		return fmt.Sprintf("%s TUN and Windows hotspot detected; EsurfingGo authentication traffic is isolated. Hotspot client Internet forwarding is outside EsurfingGo and remains controlled by Clash TUN and Windows ICS (detected hotspot subnets for diagnostics: %s)", prefix, strings.Join(status.hotspotCIDRs, ", "))
	}
	return fmt.Sprintf("%s TUN and Windows hotspot detected; EsurfingGo authentication traffic is isolated. Hotspot client Internet forwarding is outside EsurfingGo and remains controlled by Clash TUN and Windows ICS", prefix)
}

// inspectNetworkCompatibility reads the interface list once so TUN and
// hotspot decisions are based on the same snapshot. It never changes routes,
// DNS, proxies, or Internet Connection Sharing settings.
func inspectNetworkCompatibility() (networkCompatibilityStatus, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return networkCompatibilityStatus{}, fmt.Errorf("failed to inspect network interfaces: %w", err)
	}

	status := networkCompatibilityStatus{}
	seenCIDRs := make(map[string]bool)
	for _, iface := range ifaces {
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		addresses := ipv4AddressesFromAddrs(addrs)
		hasIPv4 := hasUsableIPv4(addresses)
		if isActiveLikelyTunnelInterface(iface.Name, iface.Flags, hasIPv4) {
			status.tunActive = true
		}
		if !isActiveLikelyHotspotInterfaceWithAddresses(iface.Name, iface.Flags, addresses) {
			continue
		}
		status.hotspotActive = true
		for _, cidr := range ipv4CIDRsFromAddresses(addrs) {
			if !seenCIDRs[cidr] {
				seenCIDRs[cidr] = true
				status.hotspotCIDRs = append(status.hotspotCIDRs, cidr)
			}
		}
	}
	sort.Strings(status.hotspotCIDRs)
	return status, nil
}

func hotspotInterfaceCIDRs() ([]string, error) {
	status, err := inspectNetworkCompatibility()
	return status.hotspotCIDRs, err
}

func interfaceIPv4CIDRs(iface net.Interface) []string {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	return ipv4CIDRsFromAddresses(addrs)
}

func ipv4CIDRsFromAddresses(addrs []net.Addr) []string {
	seen := make(map[string]bool)
	var result []string
	for _, addr := range addrs {
		var ip net.IP
		var mask net.IPMask
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP.To4()
			mask = value.Mask
		case *net.IPAddr:
			ip = value.IP.To4()
			mask = net.CIDRMask(32, 32)
		}
		if ip == nil {
			continue
		}
		if len(mask) == net.IPv6len {
			mask = mask[net.IPv4len:]
		}
		if len(mask) != net.IPv4len {
			continue
		}
		networkIP := ip.Mask(mask)
		if networkIP == nil {
			continue
		}
		cidr := (&net.IPNet{IP: networkIP, Mask: mask}).String()
		if !seen[cidr] {
			seen[cidr] = true
			result = append(result, cidr)
		}
	}
	sort.Strings(result)
	return result
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

// NewTUNSafeHTTPTransport returns a direct transport bound to the preferred
// usable physical interface when a likely system TUN adapter is present. A
// nil transport means normal OS routing is safe to keep using.
func NewTUNSafeHTTPTransport() (*http.Transport, string, error) {
	active, err := hasLikelyTunnelInterface()
	if err != nil || !active {
		return nil, "", err
	}

	return newPreferredPhysicalHTTPTransport()
}

// NewTUNAwareHTTPTransport returns a transport that follows TUN state changes
// while the client is running. When TUN is active, it binds authentication
// sockets to a preferred physical interface. If that binding cannot be built,
// it fails closed instead of sending authentication traffic through TUN.
func NewTUNAwareHTTPTransport() (http.RoundTripper, error) {
	return newTUNAwareHTTPTransportWithFingerprint(
		http.DefaultTransport,
		hasLikelyTunnelInterface,
		newPreferredPhysicalRoundTripper,
		preferredPhysicalInterfaceFingerprint,
	), nil
}

func newPreferredPhysicalRoundTripper() (http.RoundTripper, string, error) {
	return newPreferredPhysicalHTTPTransport()
}

func newPreferredPhysicalHTTPTransport() (*http.Transport, string, error) {
	ifaces, err := ListNetworkInterfaces()
	if err != nil {
		return nil, "", err
	}
	for i := range ifaces {
		transport, bindErr := newHTTPTransportForInterface(&ifaces[i], nil)
		if bindErr == nil {
			return transport, ifaces[i].Name, nil
		}
	}
	return nil, "", fmt.Errorf("TUN is active but no usable physical IPv4 interface was found")
}

// preferredPhysicalInterfaceFingerprint describes the currently preferred
// physical interface and its usable IPv4 addresses. It is intentionally
// read-only; the fingerprint lets a long-lived client notice an uplink or
// address change while TUN remains enabled and rebuild its socket bindings.
func preferredPhysicalInterfaceFingerprint() (string, error) {
	ifaces, err := ListNetworkInterfaces()
	if err != nil {
		return "", err
	}
	if len(ifaces) == 0 {
		return "", fmt.Errorf("no preferred physical interface found")
	}

	parts := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		fingerprint, fingerprintErr := networkInterfaceFingerprint(iface)
		if fingerprintErr != nil {
			return "", fingerprintErr
		}
		// Include the display order because it reflects the current default
		// route preference on Windows. This catches uplink changes even when
		// the candidate interfaces keep the same names and addresses.
		parts = append(parts, fmt.Sprintf("%d:%s", iface.Index, fingerprint))
	}
	return strings.Join(parts, ";"), nil
}

func networkInterfaceFingerprint(iface NetworkInterface) (string, error) {
	netIface, err := net.InterfaceByName(iface.Name)
	if err != nil {
		return "", fmt.Errorf("interface %q not found: %w", iface.Name, err)
	}
	addresses := interfaceIPv4Addresses(*netIface)
	usable := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if isUsableIPv4(address) {
			usable = append(usable, address.String())
		}
	}
	if len(usable) == 0 {
		return "", fmt.Errorf("interface %q has no usable IPv4 address", iface.Name)
	}
	sort.Strings(usable)
	return fmt.Sprintf("%d|%s|%s", iface.SystemIndex, iface.Name, strings.Join(usable, ",")), nil
}

type tunAwareHTTPTransport struct {
	fallback            http.RoundTripper
	detectTUN           func() (bool, error)
	bindPhysical        func() (http.RoundTripper, string, error)
	physicalFingerprint func() (string, error)

	mu               sync.Mutex
	bound            http.RoundTripper
	interfaceName    string
	boundFingerprint string
	fingerprintKnown bool
	active           bool
}

func newTUNAwareHTTPTransport(
	fallback http.RoundTripper,
	detectTUN func() (bool, error),
	bindPhysical func() (http.RoundTripper, string, error),
) *tunAwareHTTPTransport {
	return newTUNAwareHTTPTransportWithFingerprint(fallback, detectTUN, bindPhysical, nil)
}

func newTUNAwareHTTPTransportWithFingerprint(
	fallback http.RoundTripper,
	detectTUN func() (bool, error),
	bindPhysical func() (http.RoundTripper, string, error),
	physicalFingerprint func() (string, error),
) *tunAwareHTTPTransport {
	if fallback == nil {
		fallback = http.DefaultTransport
	}
	return &tunAwareHTTPTransport{
		fallback:            fallback,
		detectTUN:           detectTUN,
		bindPhysical:        bindPhysical,
		physicalFingerprint: physicalFingerprint,
	}
}

func (t *tunAwareHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.detectTUN == nil {
		return nil, fmt.Errorf("TUN state detector is not configured")
	}
	active, err := t.detectTUN()
	if err != nil {
		return nil, fmt.Errorf("inspect TUN state: %w", err)
	}

	transport, err := t.transportFor(active)
	if err != nil {
		return nil, err
	}
	return transport.RoundTrip(req)
}

func (t *tunAwareHTTPTransport) transportFor(active bool) (http.RoundTripper, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !active {
		if t.bound != nil {
			closeIdleConnections(t.bound)
		}
		t.bound = nil
		t.interfaceName = ""
		t.boundFingerprint = ""
		t.fingerprintKnown = false
		t.active = false
		return t.fallback, nil
	}

	currentFingerprint := ""
	fingerprintKnown := t.physicalFingerprint == nil
	if t.physicalFingerprint != nil {
		fingerprint, fingerprintErr := t.physicalFingerprint()
		if fingerprintErr == nil && fingerprint != "" {
			currentFingerprint = fingerprint
			fingerprintKnown = true
		} else if fingerprintErr != nil {
			log.Printf("[Network] Unable to refresh physical interface snapshot: %v", fingerprintErr)
		}
	}

	if t.active && t.bound != nil {
		if fingerprintKnown && t.fingerprintKnown && currentFingerprint == t.boundFingerprint {
			return t.bound, nil
		}
		if !fingerprintKnown && !t.fingerprintKnown {
			return t.bound, nil
		}
		log.Printf("[Network] Physical interface snapshot changed; rebuilding TUN-safe authentication transport")
		closeIdleConnections(t.bound)
		t.bound = nil
		t.interfaceName = ""
		t.boundFingerprint = ""
		t.fingerprintKnown = false
	}
	if t.bindPhysical == nil {
		return nil, fmt.Errorf("TUN is active but physical transport builder is not configured")
	}

	bound, interfaceName, err := t.bindPhysical()
	if err != nil {
		t.active = true
		t.bound = nil
		t.interfaceName = ""
		return nil, fmt.Errorf("TUN is active but physical authentication transport is unavailable: %w", err)
	}
	if bound == nil {
		t.active = true
		t.bound = nil
		t.interfaceName = ""
		return nil, fmt.Errorf("TUN is active but physical authentication transport is unavailable")
	}

	t.active = true
	t.bound = bound
	t.interfaceName = interfaceName
	if !fingerprintKnown && t.physicalFingerprint != nil {
		fingerprint, fingerprintErr := t.physicalFingerprint()
		if fingerprintErr == nil && fingerprint != "" {
			currentFingerprint = fingerprint
			fingerprintKnown = true
		} else if fingerprintErr != nil {
			log.Printf("[Network] Unable to establish physical interface snapshot after rebind: %v", fingerprintErr)
		}
	}
	t.boundFingerprint = currentFingerprint
	t.fingerprintKnown = fingerprintKnown
	log.Printf("[Network] TUN-safe authentication transport bound to %s", interfaceName)
	return bound, nil
}

func closeIdleConnections(transport http.RoundTripper) {
	if closer, ok := transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
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
	return newHTTPTransportForInterfaceWithMode(iface, interfaceBindingForcedRoute, nil)
}

func newHTTPTransportForInterface(iface *NetworkInterface, observeMode func(interfaceBindingMode)) (*http.Transport, error) {
	return newHTTPTransportForInterfaceWithMode(iface, interfaceBindingSourceAddress, observeMode)
}

func newHTTPTransportForInterfaceWithMode(iface *NetworkInterface, mode interfaceBindingMode, observeMode func(interfaceBindingMode)) (*http.Transport, error) {
	if observeMode != nil {
		observeMode(mode)
	}

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
		if isUsableIPv4(ip) {
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
	if mode == interfaceBindingForcedRoute {
		if err := configureDialerForInterface(dialer, netIface.Index); err != nil {
			return nil, fmt.Errorf("failed to bind socket to interface %q: %w", iface.Name, err)
		}
	}

	// Windows' native resolver can send DNS through a TUN adapter even when
	// the eventual TCP connection is bound to the physical interface. Use the
	// Go resolver and bind its UDP/TCP DNS sockets to the same interface.
	dnsUDPDialer := &net.Dialer{
		LocalAddr: &net.UDPAddr{IP: localIP},
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if mode == interfaceBindingForcedRoute {
		if err := configureDialerForInterface(dnsUDPDialer, netIface.Index); err != nil {
			return nil, fmt.Errorf("failed to bind DNS socket to interface %q: %w", iface.Name, err)
		}
	}
	dnsTCPDialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: localIP},
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if mode == interfaceBindingForcedRoute {
		if err := configureDialerForInterface(dnsTCPDialer, netIface.Index); err != nil {
			return nil, fmt.Errorf("failed to bind DNS TCP socket to interface %q: %w", iface.Name, err)
		}
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
