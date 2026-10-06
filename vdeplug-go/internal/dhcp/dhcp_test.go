// Tests for the DHCP pool fixes: address arithmetic must carry across
// octets, and the pool must never lease outside its network.
package dhcp

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// dhcpDiscover builds a raw DISCOVER payload from clientMAC with the
// BROADCAST flag set or cleared.
func dhcpDiscover(t *testing.T, clientMAC [6]byte, broadcast bool) []byte {
	t.Helper()
	m, err := dhcpv4.NewDiscovery(net.HardwareAddr(clientMAC[:]))
	if err != nil {
		t.Fatalf("build discover: %v", err)
	}
	if broadcast {
		m.SetBroadcast()
	} else {
		m.SetUnicast()
	}
	return m.ToBytes()
}

func TestPoolCarryAcrossOctets(t *testing.T) {
	// Start near the top of a /24: 10.0.2.250. Old arithmetic added to the
	// last octet only, wrapping .255 -> .0 of the same subnet (duplicates)
	// or handing out foreign addresses.
	p := NewPool(net.ParseIP("10.0.2.250"), 8)
	want := []string{
		"10.0.2.250", "10.0.2.251", "10.0.2.252", "10.0.2.253",
		"10.0.2.254", "10.0.2.255", "10.0.3.0", "10.0.3.1",
	}
	for i, w := range want {
		mac := net.HardwareAddr{0x02, 0, 0, 0, 0, byte(i)}
		ip, err := p.GetOrAssign(mac)
		if err != nil {
			t.Fatalf("lease %d: %v", i, err)
		}
		if ip.String() != w {
			t.Fatalf("lease %d: got %s, want %s", i, ip, w)
		}
	}
	if _, err := p.GetOrAssign(net.HardwareAddr{0x02, 9, 9, 9, 9, 9}); err == nil {
		t.Fatal("pool should be exhausted")
	}
	// A returning MAC gets its stable lease back.
	ip, err := p.GetOrAssign(net.HardwareAddr{0x02, 0, 0, 0, 0, 0})
	if err != nil || ip.String() != "10.0.2.250" {
		t.Fatalf("stable lease: got %s, %v", ip, err)
	}
}

func TestSizeForNetwork(t *testing.T) {
	// Default slirp layout: .15 in a /24 leaves 240 usable (.15..254).
	if n := SizeForNetwork("10.0.2.0/24", net.ParseIP("10.0.2.15"), 256); n != 240 {
		t.Fatalf("10.0.2.15 in /24: got %d, want 240", n)
	}
	// maxSize caps smaller networks' headroom.
	if n := SizeForNetwork("10.0.2.0/24", net.ParseIP("10.0.2.15"), 16); n != 16 {
		t.Fatalf("capped pool: got %d, want 16", n)
	}
	// .250 leaves exactly 5 (.250..254) — never the broadcast .255.
	if n := SizeForNetwork("10.0.2.0/24", net.ParseIP("10.0.2.250"), 256); n != 5 {
		t.Fatalf("top-of-subnet: got %d, want 5", n)
	}
	// Outside the network / not an IP / bad CIDR: disabled, not wrong.
	if n := SizeForNetwork("10.0.2.0/24", net.ParseIP("192.168.1.10"), 256); n != 0 {
		t.Fatalf("foreign start: got %d, want 0", n)
	}
	if n := SizeForNetwork("10.0.2.0/24", nil, 256); n != 0 {
		t.Fatalf("nil start: got %d, want 0", n)
	}
	if n := SizeForNetwork("bogus", net.ParseIP("10.0.2.15"), 256); n != 0 {
		t.Fatalf("bad cidr: got %d, want 0", n)
	}
}

func TestReplyHonorsBroadcastFlag(t *testing.T) {
	pool := NewPool(net.ParseIP("10.0.2.15"), 16)
	srv := New(Options{Network: "10.0.2.0/24", GatewayIP: "10.0.2.2", Nameserver: "10.0.2.3"},
		pool, "server-mac")
	clientMAC := [6]byte{0x02, 0xde, 0xad, 0xbe, 0xef, 0x01}

	discover := dhcpDiscover(t, clientMAC, false) // broadcast flag CLEAR
	frame := srv.HandlePacket(discover, clientMAC)
	if frame == nil {
		t.Fatal("no reply to unicast-flag DISCOVER")
	}
	// Ethernet destination must be the client, not ff:ff:ff:ff:ff:ff.
	if got := frame[0:6]; !bytes6Equal(got, clientMAC) {
		t.Fatalf("unicast-flag reply went to %x, want client MAC %x", got, clientMAC)
	}
	// IP destination must be the offered address.
	if got := net.IP(frame[14+16 : 14+20]); got.String() != "10.0.2.15" {
		t.Fatalf("unicast-flag reply IP dst = %s, want 10.0.2.15", got)
	}

	discover = dhcpDiscover(t, clientMAC, true) // broadcast flag SET
	frame = srv.HandlePacket(discover, clientMAC)
	if frame == nil {
		t.Fatal("no reply to broadcast DISCOVER")
	}
	if binary.BigEndian.Uint64(frame[0:8])>>16 != 0xffffffffffff {
		t.Fatalf("broadcast reply dst = %x, want ff:ff:ff:ff:ff:ff", frame[0:6])
	}
	if got := net.IP(frame[14+16 : 14+20]); got.String() != "255.255.255.255" {
		t.Fatalf("broadcast reply IP dst = %s, want 255.255.255.255", got)
	}
}

func bytes6Equal(b []byte, m [6]byte) bool {
	for i := range m {
		if b[i] != m[i] {
			return false
		}
	}
	return true
}
