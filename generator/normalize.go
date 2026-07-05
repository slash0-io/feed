package main

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// canonCIDR parses s as a CIDR or bare IP and returns the canonical (masked)
// prefix. Bare IPs become /32 or /128.
func canonCIDR(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Prefix{}, fmt.Errorf("empty range")
	}
	if !strings.Contains(s, "/") {
		a, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		return netip.PrefixFrom(a, a.BitLen()), nil
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return p.Masked(), nil
}

// dropReason returns a non-empty reason when a prefix must never appear in a
// published allowlist, regardless of what the upstream feed claims.
func dropReason(p netip.Prefix) string {
	a := p.Addr()
	switch {
	case p.Bits() == 0:
		return "default route"
	case a.IsPrivate():
		return "private address space"
	case a.IsLoopback():
		return "loopback"
	case a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast():
		return "link-local"
	case a.IsMulticast():
		return "multicast"
	case a.IsUnspecified():
		return "unspecified"
	}
	return ""
}

// normalize validates, dedupes, and sorts raw range strings into canonical
// IPv4 and IPv6 CIDR lists. Invalid or non-publishable entries are reported
// through warn and skipped.
func normalize(raw []string, warn func(format string, args ...any)) (v4, v6 []string) {
	seen := map[netip.Prefix]bool{}
	var p4, p6 []netip.Prefix
	for _, s := range raw {
		p, err := canonCIDR(s)
		if err != nil {
			warn("invalid range %q: %v", s, err)
			continue
		}
		if r := dropReason(p); r != "" {
			warn("dropping %s: %s", p, r)
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		if p.Addr().Is4() {
			p4 = append(p4, p)
		} else {
			p6 = append(p6, p)
		}
	}
	less := func(a, b netip.Prefix) bool {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c < 0
		}
		return a.Bits() < b.Bits()
	}
	sort.Slice(p4, func(i, j int) bool { return less(p4[i], p4[j]) })
	sort.Slice(p6, func(i, j int) bool { return less(p6[i], p6[j]) })
	for _, p := range p4 {
		v4 = append(v4, p.String())
	}
	for _, p := range p6 {
		v6 = append(v6, p.String())
	}
	return v4, v6
}
