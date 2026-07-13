package httpmulti

import (
	"net"
	"net/netip"
	"testing"
)

type remoteAddrConn struct {
	net.Conn
	remote net.Addr
}

func (c remoteAddrConn) RemoteAddr() net.Addr {
	return c.remote
}

type testAddr string

func (a testAddr) Network() string { return "test" }
func (a testAddr) String() string  { return string(a) }

func TestNewCIDRMatcherAndMatchAddr(t *testing.T) {
	matcher, err := NewCIDRMatcher([]string{"192.0.2.0/24", "2001:db8::/32"})
	if err != nil {
		t.Fatalf("NewCIDRMatcher() error = %v", err)
	}

	tests := []struct {
		addr netip.Addr
		want bool
	}{
		{addr: netip.MustParseAddr("192.0.2.17"), want: true},
		{addr: netip.MustParseAddr("192.0.3.17"), want: false},
		{addr: netip.MustParseAddr("2001:db8::1"), want: true},
		{addr: netip.MustParseAddr("2001:db9::1"), want: false},
	}
	for _, test := range tests {
		if got := matcher.MatchAddr(test.addr); got != test.want {
			t.Errorf("MatchAddr(%s) = %t, want %t", test.addr, got, test.want)
		}
	}

	if _, err := NewCIDRMatcher([]string{"192.0.2.1"}); err == nil {
		t.Fatal("NewCIDRMatcher() accepted a CIDR without a prefix length")
	}
	if _, err := NewCIDRMatcher([]string{"not-a-cidr"}); err == nil {
		t.Fatal("NewCIDRMatcher() accepted an invalid CIDR")
	}
}

func TestCIDRMatcherMatchConn(t *testing.T) {
	matcher, err := NewCIDRMatcher([]string{"192.0.2.0/24", "2001:db8::/32"})
	if err != nil {
		t.Fatalf("NewCIDRMatcher() error = %v", err)
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	tests := []struct {
		name   string
		remote net.Addr
		want   bool
	}{
		{name: "allowed ipv4", remote: testAddr("192.0.2.12:443"), want: true},
		{name: "allowed ipv6", remote: testAddr("[2001:db8::2]:443"), want: true},
		{name: "outside configured networks", remote: testAddr("198.51.100.4:443")},
		{name: "missing port", remote: testAddr("192.0.2.12")},
		{name: "invalid host", remote: testAddr("not-an-ip:443")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := remoteAddrConn{Conn: left, remote: test.remote}
			if got := matcher.MatchConn(conn); got != test.want {
				t.Errorf("MatchConn(%q) = %t, want %t", test.remote, got, test.want)
			}
		})
	}
}

func TestConnReadReplaysPeekedByte(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	go func() {
		_, _ = clientConn.Write([]byte("ello"))
	}()

	conn := &Conn{Conn: serverConn, b: 'H', f: true}
	buffer := make([]byte, len("Hello"))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != len(buffer) || string(buffer) != "Hello" {
		t.Fatalf("Read() = %q, %d bytes; want Hello, %d bytes", buffer, n, len(buffer))
	}

	go func() {
		_, _ = clientConn.Write([]byte("!"))
	}()
	n, err = conn.Read(buffer[:1])
	if err != nil {
		t.Fatalf("subsequent Read() error = %v", err)
	}
	if n != 1 || buffer[0] != '!' {
		t.Fatalf("subsequent Read() = %q, %d bytes; want !", buffer[:n], n)
	}
}
