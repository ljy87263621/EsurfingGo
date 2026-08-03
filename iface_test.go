package main

import (
	"net"
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
