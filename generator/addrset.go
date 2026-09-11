package main

import (
	"math/big"
	"net/netip"
	"sort"
	"strings"
)

// Address-set arithmetic, shared by the audit report.
//
// Every comparison in the audit is done on address sets, never on CIDR
// strings. A vendor is free to re-express the same space with different
// prefixes: string diffs invented missing addresses for Fivetran and Retool,
// and quarantined azure for two days when Microsoft summarised 15385 prefixes
// into 2016 supernets covering strictly more space.

// addrSet is a coalesced, sorted set of inclusive [lo, hi] address intervals
// for ONE family. Mixing families in one set is meaningless, so the family is
// carried alongside as a bit width.
type addrSet struct {
	bits      int // 32 or 128
	intervals [][2]*big.Int
}

func newAddrSet(bits int, cidrs []string) addrSet {
	return addrSet{bits: bits, intervals: coalesceIntervals(intervals(cidrs))}
}

// splitFamilies sorts mixed CIDR strings into an IPv4 and an IPv6 set.
func splitFamilies(cidrs []string) (v4, v6 addrSet) {
	var s4, s6 []string
	for _, c := range cidrs {
		p, err := canonCIDR(c)
		if err != nil {
			continue
		}
		if p.Addr().Is4() {
			s4 = append(s4, p.String())
		} else {
			s6 = append(s6, p.String())
		}
	}
	return newAddrSet(32, s4), newAddrSet(128, s6)
}

func (s addrSet) empty() bool { return len(s.intervals) == 0 }

// size is the number of addresses the set covers.
func (s addrSet) size() *big.Int {
	total := new(big.Int)
	one := big.NewInt(1)
	for _, iv := range s.intervals {
		n := new(big.Int).Sub(iv[1], iv[0])
		n.Add(n, one)
		total.Add(total, n)
	}
	return total
}

// minus returns the addresses in s that other does not cover.
func (s addrSet) minus(other addrSet) addrSet {
	one := big.NewInt(1)
	var out [][2]*big.Int
	for _, iv := range s.intervals {
		lo := new(big.Int).Set(iv[0])
		hi := iv[1]
		for _, n := range other.intervals {
			if n[1].Cmp(lo) < 0 {
				continue
			}
			if n[0].Cmp(hi) > 0 {
				break
			}
			if n[0].Cmp(lo) > 0 {
				out = append(out, [2]*big.Int{new(big.Int).Set(lo), new(big.Int).Sub(n[0], one)})
			}
			if next := new(big.Int).Add(n[1], one); next.Cmp(lo) > 0 {
				lo = next
			}
			if lo.Cmp(hi) > 0 {
				break
			}
		}
		if lo.Cmp(hi) <= 0 {
			out = append(out, [2]*big.Int{new(big.Int).Set(lo), new(big.Int).Set(hi)})
		}
	}
	return addrSet{bits: s.bits, intervals: out}
}

// union merges two sets of the same family.
func (s addrSet) union(other addrSet) addrSet {
	merged := make([][2]*big.Int, 0, len(s.intervals)+len(other.intervals))
	merged = append(merged, s.intervals...)
	merged = append(merged, other.intervals...)
	return addrSet{bits: s.bits, intervals: coalesceIntervals(merged)}
}

// cidrs renders the set as the minimal list of CIDR prefixes covering exactly
// it, so a coverage gap can be read as ranges rather than as an address count.
func (s addrSet) cidrs() []string {
	one := big.NewInt(1)
	var out []string
	for _, iv := range s.intervals {
		lo := new(big.Int).Set(iv[0])
		hi := iv[1]
		for lo.Cmp(hi) <= 0 {
			// The largest aligned block starting at lo, shrunk until it fits
			// inside what is left of the interval.
			bits := 0
			if lo.Sign() != 0 {
				if tz := int(lo.TrailingZeroBits()); tz < s.bits {
					bits = s.bits - tz
				}
			}
			remaining := new(big.Int).Sub(hi, lo)
			remaining.Add(remaining, one)
			for {
				if new(big.Int).Lsh(one, uint(s.bits-bits)).Cmp(remaining) <= 0 {
					break
				}
				bits++
			}
			out = append(out, netip.PrefixFrom(bigToAddr(lo, s.bits), bits).String())
			lo = new(big.Int).Add(lo, new(big.Int).Lsh(one, uint(s.bits-bits)))
		}
	}
	return out
}

func bigToAddr(v *big.Int, bits int) netip.Addr {
	n := bits / 8
	b := v.Bytes()
	buf := make([]byte, n)
	if len(b) <= n {
		copy(buf[n-len(b):], b)
	}
	if n == 4 {
		return netip.AddrFrom4([4]byte(buf))
	}
	return netip.AddrFrom16([16]byte(buf))
}

// comma formats a big integer with thousands separators, because address
// counts are the numbers most often misread in these reports.
func comma(v *big.Int) string {
	s := v.String()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if neg {
		return "-" + out
	}
	return out
}

// sortCIDRs orders prefixes numerically rather than lexically, so a sample of
// a coverage gap reads in address order.
func sortCIDRs(in []string) []string {
	out := append([]string(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		a, errA := canonCIDR(out[i])
		b, errB := canonCIDR(out[j])
		if errA != nil || errB != nil {
			return out[i] < out[j]
		}
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c < 0
		}
		return a.Bits() < b.Bits()
	})
	return out
}
