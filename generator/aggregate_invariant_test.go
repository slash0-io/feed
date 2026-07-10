package main

import (
	"net/netip"
	"sort"
	"testing"
)

type addrInterval struct{ lo, hi netip.Addr } // inclusive

func prefixInterval(p netip.Prefix) addrInterval {
	lo := p.Addr()
	hi := lastAddr(p)
	return addrInterval{lo, hi}
}

// lastAddr returns the highest address in p.
func lastAddr(p netip.Prefix) netip.Addr {
	a := p.Addr().As16()
	bits := p.Bits()
	if p.Addr().Is4() {
		bits += 96
	}
	for i := bits; i < 128; i++ {
		a[i/8] |= 1 << (7 - i%8)
	}
	addr := netip.AddrFrom16(a)
	if p.Addr().Is4() {
		addr = addr.Unmap()
		return netip.AddrFrom4(addr.As4())
	}
	return addr
}

// mergeIntervals produces the canonical union of a set of prefixes as sorted,
// disjoint, non-adjacent address intervals.
func mergeIntervals(prefixes []netip.Prefix) []addrInterval {
	ivs := make([]addrInterval, 0, len(prefixes))
	for _, p := range prefixes {
		ivs = append(ivs, prefixInterval(p))
	}
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].lo.Compare(ivs[j].lo) < 0 })
	var out []addrInterval
	for _, iv := range ivs {
		if n := len(out); n > 0 {
			prev := &out[n-1]
			// Extend when overlapping or exactly adjacent.
			if iv.lo.Compare(prev.hi) <= 0 || prev.hi.Next() == iv.lo {
				if iv.hi.Compare(prev.hi) > 0 {
					prev.hi = iv.hi
				}
				continue
			}
		}
		out = append(out, iv)
	}
	return out
}

// TestAggregationInvariantAcrossFixtures proves, for every purpose of every
// service in the archived fixtures, that lossless aggregation preserves the
// covered address set EXACTLY: the interval union of the raw published ranges
// equals the interval union of the normalized output. No loss, no overshoot.
func TestAggregationInvariantAcrossFixtures(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewFetcher(fixturesDir)
	checked := 0
	for _, svc := range reg.Services {
		for _, ep := range svc.Endpoints {
			parse, ok := parsers[ep.Format]
			if !ok {
				continue
			}
			body, _, err := fetcher.Get(svc, ep)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range ep.Purposes {
				raw, err := parse(body, decl.Select)
				if err != nil {
					t.Fatal(err)
				}
				// Ground truth: every raw range that survives validation,
				// un-aggregated.
				var rawPrefixes []netip.Prefix
				for _, s := range raw {
					p, err := canonCIDR(s)
					if err != nil || dropReason(p) != "" {
						continue
					}
					rawPrefixes = append(rawPrefixes, p)
				}
				v4, v6 := normalize(raw, func(string, ...any) {})
				var outPrefixes []netip.Prefix
				for _, s := range append(append([]string{}, v4...), v6...) {
					outPrefixes = append(outPrefixes, netip.MustParsePrefix(s))
				}
				want := mergeIntervals(rawPrefixes)
				got := mergeIntervals(outPrefixes)
				if len(want) != len(got) {
					t.Fatalf("%s/%s: %d union intervals raw vs %d aggregated", svc.Slug, decl.Key, len(want), len(got))
				}
				for i := range want {
					if want[i] != got[i] {
						t.Fatalf("%s/%s: interval %d: raw [%s-%s] vs aggregated [%s-%s]",
							svc.Slug, decl.Key, i, want[i].lo, want[i].hi, got[i].lo, got[i].hi)
					}
				}
				checked++
			}
		}
	}
	if checked < 50 {
		t.Fatalf("only %d purposes checked — fixture coverage regressed?", checked)
	}
	t.Logf("zero-loss/zero-overshoot proven for %d purposes", checked)
}
