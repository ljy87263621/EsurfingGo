package main

import "testing"

func TestBoundResolverNetworkUsesIPv4ForPhysicalInterface(t *testing.T) {
	for _, network := range []string{"udp", "udp4", "tcp", "tcp4"} {
		got := boundResolverNetwork(network)
		want := network
		if network == "udp" {
			want = "udp4"
		}
		if got != want {
			t.Fatalf("boundResolverNetwork(%q) = %q, want %q", network, got, want)
		}
	}
}
