package httpmulti

import (
	"fmt"
	"net"
	"net/netip"
)

// CIDRMatcher stores the configured CIDR prefixes.
type CIDRMatcher struct {
	prefixes []netip.Prefix
}

// NewCIDRMatcher parses a list of CIDRs once.
func NewCIDRMatcher(cidrs []string) (*CIDRMatcher, error) {
	m := &CIDRMatcher{
		prefixes: make([]netip.Prefix, 0, len(cidrs)),
	}

	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("CIDR invalide %q: %w", c, err)
		}
		m.prefixes = append(m.prefixes, p)
	}

	return m, nil
}

// MatchAddr checks whether an IP belongs to one of the CIDR prefixes.
func (m *CIDRMatcher) MatchAddr(addr netip.Addr) bool {
	for _, p := range m.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// MatchConn checks the remote address of a net.Conn.
func (m *CIDRMatcher) MatchConn(conn net.Conn) bool {
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return false
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return m.MatchAddr(addr)
}
