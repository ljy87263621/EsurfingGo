package main

import (
	"net"
	"testing"
)

func TestIsLikelyVirtualInterfaceName(t *testing.T) {
	for _, name := range []string{"Mihomo", "Clash TUN", "Wintun", "WireGuard", "Npcap Loopback Adapter"} {
		if !isLikelyVirtualInterfaceName(name) {
			t.Fatalf("isLikelyVirtualInterfaceName(%q) = false, want true", name)
		}
	}
	if !isLikelyHotspotInterfaceName("移动热点") {
		t.Fatal("localized mobile hotspot adapter should be recognized")
	}
	for _, name := range []string{"以太网", "WLAN", "Intel(R) Wi-Fi 6E AX211 160MHz"} {
		if isLikelyVirtualInterfaceName(name) {
			t.Fatalf("isLikelyVirtualInterfaceName(%q) = true, want false", name)
		}
	}
	if isLikelyHotspotInterfaceName("以太网") {
		t.Fatal("physical Ethernet adapter should not be recognized as a hotspot")
	}
}

func TestIsLikelyTunnelInterfaceNameDoesNotTreatWiFiDirectAsClashTUN(t *testing.T) {
	if isLikelyTunnelInterfaceName("本地连接* 2") {
		t.Fatal("Wi-Fi Direct adapter should not activate TUN-safe routing")
	}
}

func TestIsActiveLikelyTunnelInterfaceRequiresUpIPv4(t *testing.T) {
	for _, test := range []struct {
		name    string
		flags   net.Flags
		hasIPv4 bool
	}{
		{name: "Teredo Tunneling Pseudo-Interface", flags: 0, hasIPv4: false},
		{name: "Clash TUN", flags: net.FlagUp, hasIPv4: false},
	} {
		if isActiveLikelyTunnelInterface(test.name, test.flags, test.hasIPv4) {
			t.Fatalf("isActiveLikelyTunnelInterface(%q, %v, %v) = true, want false", test.name, test.flags, test.hasIPv4)
		}
	}

	if !isActiveLikelyTunnelInterface("Clash TUN", net.FlagUp, true) {
		t.Fatal("an enabled tunnel adapter with IPv4 should activate TUN-safe routing")
	}
}
