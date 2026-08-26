package main

import (
	"strings"
	"testing"

	"github.com/slash0-io/feed/feedschema"
)

func purpose(v4 ...string) feedschema.Purpose { return feedschema.Purpose{IPv4: v4} }

// The Azure regression, reduced. On 2026-08-24 Microsoft re-expressed the
// AzureCloud service tag as fewer, broader supernets covering a strict
// superset of the previous space. Comparing CIDR strings called that "61% of
// ranges removed" and quarantined the service for two days.
func TestGuardrailIgnoresPureReexpression(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": purpose(
		"192.0.2.0/26", "192.0.2.64/26", "192.0.2.128/26", "192.0.2.192/26",
		"198.51.100.0/25", "198.51.100.128/25",
	)}
	// Same addresses, two prefixes instead of six.
	next := map[string]feedschema.Purpose{"all": purpose("192.0.2.0/24", "198.51.100.0/24")}

	if r := quarantineReason(prev, next, 0.5, 4); r != "" {
		t.Fatalf("re-expressing the same space must not quarantine, got %q", r)
	}
}

// Broadening is not loss either: the new set covers everything the old one did
// and more, which is what Azure actually published.
func TestGuardrailAllowsStrictSuperset(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": purpose(
		"192.0.2.0/26", "192.0.2.64/26", "192.0.2.128/26", "192.0.2.192/26",
	)}
	next := map[string]feedschema.Purpose{"all": purpose("192.0.0.0/16")}

	if r := quarantineReason(prev, next, 0.5, 4); r != "" {
		t.Fatalf("a strict superset must not quarantine, got %q", r)
	}
}

// The guardrail still has to catch what it exists for: an upstream body that
// really did lose most of its addresses.
func TestGuardrailCatchesRealAddressLoss(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": purpose(
		"192.0.2.0/26", "192.0.2.64/26", "192.0.2.128/26", "192.0.2.192/26",
		"198.51.100.0/24", "203.0.113.0/24",
	)}
	next := map[string]feedschema.Purpose{"all": purpose("192.0.2.0/26")}

	r := quarantineReason(prev, next, 0.5, 4)
	if r == "" {
		t.Fatal("losing most of the published space must quarantine")
	}
	if !strings.Contains(r, "no longer covered") {
		t.Fatalf("reason should name the coverage loss, got %q", r)
	}
}

// A truncated body that keeps IPv6 while dropping every IPv4 range must still
// trip. One IPv6 /32 dwarfs the entire IPv4 internet by address count, so a
// combined fraction would hide this completely.
func TestGuardrailJudgesFamiliesSeparately(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": {
		IPv4: []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"},
		IPv6: []string{"2001:db8::/32"},
	}}
	next := map[string]feedschema.Purpose{"all": {
		IPv4: []string{"192.0.2.0/28"},
		IPv6: []string{"2001:db8::/32"},
	}}

	r := quarantineReason(prev, next, 0.5, 4)
	if r == "" {
		t.Fatal("losing nearly all IPv4 must quarantine even when IPv6 is intact")
	}
	if !strings.Contains(r, "IPv4") {
		t.Fatalf("reason should name the affected family, got %q", r)
	}
}

// A purpose vanishing outright is still the loudest case.
func TestGuardrailCatchesDisappearedPurpose(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": purpose(
		"192.0.2.0/26", "192.0.2.64/26", "192.0.2.128/26", "192.0.2.192/26",
	)}
	if r := quarantineReason(prev, map[string]feedschema.Purpose{}, 0.5, 4); !strings.Contains(r, "disappeared") {
		t.Fatalf("a vanished purpose must quarantine, got %q", r)
	}
}

// Small purposes stay exempt, so a two-range service churning normally does
// not trip the guardrail.
func TestGuardrailSkipsSmallPurposes(t *testing.T) {
	prev := map[string]feedschema.Purpose{"all": purpose("192.0.2.0/24", "198.51.100.0/24")}
	next := map[string]feedschema.Purpose{"all": purpose("192.0.2.0/24")}

	if r := quarantineReason(prev, next, 0.5, 8); r != "" {
		t.Fatalf("purposes below minCount must be exempt, got %q", r)
	}
}
