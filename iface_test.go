package main

import (
	"net"
	"strings"
	"testing"
)

func TestShouldListNetworkInterfaceRejectsTUNAndVirtualNames(t *testing.T) {
	for _, name := range []string{
		"Mihomo",
		"Clash TUN",
		"Wintun",
		"本地连接* 2",
		"Npcap Loopback Adapter",
		"vEthernet (WSL (Hyper-V firewall))",
		"Hyper-V Virtual Ethernet Adapter",
		"Microsoft Wi-Fi Direct Virtual Adapter #2",
		"Local Area Connection* 2",
		"Mobile Hotspot",
	} {
		iface := net.Interface{
			Name:  name,
			Flags: net.FlagUp | net.FlagBroadcast,
		}
		if shouldListNetworkInterface(iface, true) {
			t.Fatalf("shouldListNetworkInterface(%q) = true, want false", name)
		}
	}
}

func TestShouldListNetworkInterfaceKeepsPhysicalIPv4Interface(t *testing.T) {
	iface := net.Interface{
		Name:  "以太网",
		Flags: net.FlagUp | net.FlagBroadcast,
	}
	if !shouldListNetworkInterface(iface, true) {
		t.Fatal("physical interface with IPv4 should be listed")
	}
}

func TestShouldListNetworkInterfaceRejectsNonIPv4Interface(t *testing.T) {
	iface := net.Interface{
		Name:  "以太网",
		Flags: net.FlagUp | net.FlagBroadcast,
	}
	if shouldListNetworkInterface(iface, false) {
		t.Fatal("interface without IPv4 should not be listed")
	}
}

func TestIsUsableIPv4RejectsNonRoutableAddresses(t *testing.T) {
	for _, address := range []string{"0.0.0.0", "127.0.0.1", "169.254.190.233", "224.0.0.1"} {
		if isUsableIPv4(net.ParseIP(address)) {
			t.Fatalf("isUsableIPv4(%q) = true, want false", address)
		}
	}
}

func TestIsUsableIPv4KeepsPrivateRoutableAddress(t *testing.T) {
	if !isUsableIPv4(net.ParseIP("10.10.8.105")) {
		t.Fatal("private IPv4 address should be usable for a bound transport")
	}
}

func TestIsLikelyWindowsICSAddressOnlyMatchesKnownHotspotGateways(t *testing.T) {
	for _, address := range []string{"192.168.137.1", "192.168.5.1"} {
		if !isLikelyWindowsICSAddress(net.ParseIP(address)) {
			t.Fatalf("isLikelyWindowsICSAddress(%q) = false, want true", address)
		}
	}
	for _, address := range []string{"192.168.137.2", "192.168.6.1", "10.10.8.105"} {
		if isLikelyWindowsICSAddress(net.ParseIP(address)) {
			t.Fatalf("isLikelyWindowsICSAddress(%q) = true, want false", address)
		}
	}
}

func TestShouldListNetworkInterfaceRejectsRenamedWindowsHotspotByAddress(t *testing.T) {
	iface := net.Interface{
		Name:  "Shared uplink",
		Flags: net.FlagUp | net.FlagBroadcast,
	}
	addresses := []net.IP{net.ParseIP("192.168.137.1")}
	if shouldListNetworkInterfaceWithAddresses(iface, true, addresses) {
		t.Fatal("Windows hotspot gateway should be excluded even when its adapter was renamed")
	}

	physical := net.Interface{
		Name:  "以太网",
		Flags: net.FlagUp | net.FlagBroadcast,
	}
	if !shouldListNetworkInterfaceWithAddresses(physical, true, []net.IP{net.ParseIP("10.10.8.105")}) {
		t.Fatal("physical interface should remain selectable")
	}
}

func TestIPv4CIDRsFromAddressesNormalizesDeduplicatesAndSorts(t *testing.T) {
	addresses := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.137.2").To4(), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.5.1").To4(), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.137.1").To4(), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
	}

	got := ipv4CIDRsFromAddresses(addresses)
	want := []string{"192.168.137.0/24", "192.168.5.0/24"}
	if len(got) != len(want) {
		t.Fatalf("CIDR count = %d, want %d (%v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("CIDR[%d] = %q, want %q; all = %v", index, got[index], want[index], got)
		}
	}
}

func TestNetworkCompatibilityStatusExplainsHotspotForwardingBoundary(t *testing.T) {
	status := networkCompatibilityStatus{
		tunActive:     true,
		hotspotActive: true,
		hotspotCIDRs:  []string{"192.168.137.0/24"},
	}

	got := status.hotspotExclusionHint()
	for _, want := range []string{
		"TUN+热点",
		"仅本程序认证流量已隔离",
		"热点客户端转发仍由 Clash/Windows ICS 负责",
		"192.168.137.0/24",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hotspot boundary hint %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "排除") {
		t.Fatalf("hotspot boundary hint must not imply CIDR exclusion is a complete fix: %q", got)
	}

	status.tunActive = false
	if got := status.hotspotExclusionHint(); got != "" {
		t.Fatalf("hint without TUN = %q, want empty", got)
	}
}

func TestNetworkCompatibilityLogMessageExplainsSystemBoundary(t *testing.T) {
	status := networkCompatibilityStatus{
		tunActive:     true,
		hotspotActive: true,
		hotspotCIDRs:  []string{"192.168.5.0/24"},
	}
	message := networkCompatibilityLogMessage("[test]", status)
	for _, want := range []string{
		"[test]",
		"192.168.5.0/24",
		"EsurfingGo authentication traffic is isolated",
		"Hotspot client Internet forwarding is outside EsurfingGo",
		"Clash TUN and Windows ICS",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("network compatibility message %q does not contain %q", message, want)
		}
	}
	if strings.Contains(message, "exclude hotspot subnets") {
		t.Fatalf("network compatibility message must not present exclusion as a fix: %q", message)
	}
}

func TestPrioritizeNetworkInterfacesByDefaultRoute(t *testing.T) {
	ifaces := []NetworkInterface{
		{Index: 1, Name: "Mobile Hotspot", SystemIndex: 14},
		{Index: 2, Name: "Ethernet", SystemIndex: 13},
		{Index: 3, Name: "Virtual Adapter", SystemIndex: 35},
	}

	ordered := prioritizeNetworkInterfacesByDefaultRoute(ifaces, map[int]bool{13: true})
	if len(ordered) != len(ifaces) {
		t.Fatalf("prioritized interface count = %d, want %d", len(ordered), len(ifaces))
	}
	if ordered[0].Name != "Ethernet" {
		t.Fatalf("first prioritized interface = %q, want Ethernet", ordered[0].Name)
	}
	if ordered[1].Name != "Mobile Hotspot" || ordered[2].Name != "Virtual Adapter" {
		t.Fatalf("non-default interface order changed unexpectedly: %#v", ordered)
	}
	for index, iface := range ordered {
		want := index + 1
		if iface.Index != want {
			t.Fatalf("prioritized interface %q has display index %d, want %d", iface.Name, iface.Index, want)
		}
	}
}

func TestPrioritizeNetworkInterfacesWithoutRouteInfoKeepsOrder(t *testing.T) {
	ifaces := []NetworkInterface{
		{Index: 1, Name: "First", SystemIndex: 1},
		{Index: 2, Name: "Second", SystemIndex: 2},
	}

	ordered := prioritizeNetworkInterfacesByDefaultRoute(ifaces, nil)
	if ordered[0].Name != "First" || ordered[1].Name != "Second" {
		t.Fatalf("interface order changed without route information: %#v", ordered)
	}
}
