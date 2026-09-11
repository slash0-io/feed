package main

import (
	"math/big"
	"strings"
	"testing"

	"github.com/slash0-io/feed/feedschema"
)

// Fixtures use documentation space (198.51.100.0/24, 203.0.113.0/24) because
// normalize drops RFC1918, which would make a range test pass vacuously.

func sizeOfCIDRs(t *testing.T, cidrs []string) *big.Int {
	t.Helper()
	total := new(big.Int)
	one := big.NewInt(1)
	for _, iv := range coalesceIntervals(intervals(cidrs)) {
		n := new(big.Int).Sub(iv[1], iv[0])
		total.Add(total, n.Add(n, one))
	}
	return total
}

// A normalized list routinely holds prefixes that are adjacent without being
// siblings (a /25, then a /26, then a /27), because lossless aggregation only
// merges exact halves. Coalescing has to join them. Extending a copy of the
// array element instead dropped the follower entirely and understated every
// coverage figure in this file.
func TestCoalesceIntervalsMergesAdjacentRuns(t *testing.T) {
	cidrs := []string{"203.0.113.0/25", "203.0.113.128/26", "203.0.113.192/27"}
	got := coalesceIntervals(intervals(cidrs))
	if len(got) != 1 {
		t.Fatalf("got %d intervals, want 1 merged run", len(got))
	}
	if want := big.NewInt(224); sizeOfCIDRs(t, cidrs).Cmp(want) != 0 {
		t.Errorf("covered %s addresses, want %s", sizeOfCIDRs(t, cidrs), want)
	}
}

func TestCoalesceIntervalsDoesNotMutateInput(t *testing.T) {
	in := intervals([]string{"198.51.100.0/25", "198.51.100.128/25"})
	before := in[0][1].String()
	coalesceIntervals(in)
	if in[0][1].String() != before {
		t.Errorf("input interval mutated: %s -> %s", before, in[0][1])
	}
}

func TestLostCoverageIgnoresReexpression(t *testing.T) {
	prev := []string{"203.0.113.0/25", "203.0.113.128/26", "203.0.113.192/27"}
	next := []string{"203.0.113.0/24"} // strictly wider, same addresses covered
	had, lost := lostCoverage(prev, next)
	if want := big.NewInt(224); had.Cmp(want) != 0 {
		t.Errorf("had %s addresses, want %s", had, want)
	}
	if lost.Sign() != 0 {
		t.Errorf("lost %s addresses, want 0: a superset cannot lose coverage", lost)
	}
}

// The guardrail exists to stop a truncated upstream from propagating. It must
// never fire on a vendor re-expressing the same space, which is what
// quarantined azure for two days in August. Here next is a strict SUPERSET of
// prev, expressed as a run of adjacent prefixes.
func TestQuarantineIgnoresSupersetWithAdjacentRun(t *testing.T) {
	var prevRanges []string
	for i := 64; i < 96; i += 4 {
		prevRanges = append(prevRanges, strings.Replace("203.0.113.X/32", "X", itoa(i), 1))
	}
	prev := map[string]feedschema.Purpose{
		"all": {Direction: "egress", IPv4: prevRanges},
	}
	next := map[string]feedschema.Purpose{
		"all": {Direction: "egress", IPv4: []string{"203.0.113.0/26", "203.0.113.64/27", "203.0.113.96/28"}},
	}
	if reason := quarantineReason(prev, next, 0.5, 8); reason != "" {
		t.Errorf("quarantined a superset: %s", reason)
	}
}

func itoa(i int) string { return big.NewInt(int64(i)).String() }

func TestAddrSetMinus(t *testing.T) {
	ours := newAddrSet(32, []string{"198.51.100.0/25"})
	doc := newAddrSet(32, []string{"198.51.100.0/24"})
	gap := doc.minus(ours)
	if got, want := gap.size().String(), "128"; got != want {
		t.Fatalf("gap size %s, want %s", got, want)
	}
	if got := gap.cidrs(); len(got) != 1 || got[0] != "198.51.100.128/25" {
		t.Errorf("gap cidrs %v, want [198.51.100.128/25]", got)
	}
	if !doc.minus(doc).empty() {
		t.Error("a set minus itself must be empty")
	}
}

// cidrs() must render exactly the addresses in the set, never a wider prefix:
// overshoot in a published allowlist is attacker-rentable space.
func TestAddrSetCIDRsAreExact(t *testing.T) {
	for _, in := range [][]string{
		{"198.51.100.1/32"},
		{"198.51.100.3/32", "198.51.100.4/32", "198.51.100.5/32"},
		{"2001:db8::/33"},
	} {
		s4, s6 := splitFamilies(in)
		for _, set := range []addrSet{s4, s6} {
			if set.empty() {
				continue
			}
			round := newAddrSet(set.bits, set.cidrs())
			if !round.minus(set).empty() || !set.minus(round).empty() {
				t.Errorf("%v: cidrs() round trip changed the address set (%v)", in, set.cidrs())
			}
		}
	}
}
