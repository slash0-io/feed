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
	p4 = aggregate(p4)
	p6 = aggregate(p6)
	v4, v6 = make([]string, 0, len(p4)), make([]string, 0, len(p6)) // [] not null in JSON
	for _, p := range p4 {
		v4 = append(v4, p.String())
	}
	for _, p := range p6 {
		v6 = append(v6, p.String())
	}
	return v4, v6
}

// aggregate performs LOSSLESS aggregation on a sorted, deduped prefix list:
// prefixes contained in another published prefix are dropped, and sibling
// pairs (the two exact halves of a common parent) are merged into that
// parent. The covered address set is always exactly preserved — this product
// never widens a published range (deliberate: overshoot space in shared cloud
// ranges is attacker-rentable).
func aggregate(prefixes []netip.Prefix) []netip.Prefix {
	for changed := true; changed; {
		changed = false

		// Drop subsumed prefixes. Sorted order puts a covering prefix
		// immediately before anything it contains.
		kept := prefixes[:0]
		for _, p := range prefixes {
			if n := len(kept); n > 0 {
				top := kept[n-1]
				if top.Bits() <= p.Bits() && top.Contains(p.Addr()) {
					changed = true
					continue
				}
			}
			kept = append(kept, p)
		}
		prefixes = kept

		// Merge exact sibling halves into their parent.
		merged := prefixes[:0]
		for i := 0; i < len(prefixes); i++ {
			p := prefixes[i]
			if i+1 < len(prefixes) && p.Bits() > 0 && p.Bits() == prefixes[i+1].Bits() {
				parent := netip.PrefixFrom(p.Addr(), p.Bits()-1).Masked()
				if parent == netip.PrefixFrom(prefixes[i+1].Addr(), p.Bits()-1).Masked() && p.Addr() != prefixes[i+1].Addr() {
					merged = append(merged, parent)
					i++
					changed = true
					continue
				}
			}
			merged = append(merged, p)
		}
		prefixes = merged
	}
	return prefixes
}
