package main

import (
	"net/netip"
	"strings"
	"testing"
)

const fixturesDir = "../testdata/fixtures"

// TestRegistryAgainstFixtures drives every implemented service/endpoint/
// purpose in sources.yaml against the archived upstream fixtures: parse must
// succeed and normalization must yield at least one publishable range.
func TestRegistryAgainstFixtures(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Services) < 25 {
		t.Fatalf("registry suspiciously small: %d services", len(reg.Services))
	}
	fetcher := NewFetcher(fixturesDir)

	for _, svc := range reg.Services {
		for _, ep := range svc.Endpoints {
			parse, ok := parsers[ep.Format]
			if !ok {
				continue // docs-page / azure formats: not in v1
			}
			body, _, err := fetcher.Get(svc, ep)
			if err != nil {
				t.Errorf("%s/%s: %v", svc.Slug, ep.ID, err)
				continue
			}
			for _, decl := range ep.Purposes {
				raw, err := parse(body, decl.Select)
				if err != nil {
					t.Errorf("%s/%s purpose %s: parse: %v", svc.Slug, ep.ID, decl.Key, err)
					continue
				}
				v4, v6 := normalize(raw, func(string, ...any) {})
				if len(v4)+len(v6) == 0 {
					t.Errorf("%s/%s purpose %s: zero ranges", svc.Slug, ep.ID, decl.Key)
				}
				for _, s := range append(append([]string{}, v4...), v6...) {
					if _, err := netip.ParsePrefix(s); err != nil {
						t.Errorf("%s/%s purpose %s: bad output %q", svc.Slug, ep.ID, decl.Key, s)
					}
				}
			}
		}
	}
}

// Spot checks against values observed in the archived fixtures.
func TestParserSpotChecks(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	byslug := map[string]SourceService{}
	for _, s := range reg.Services {
		byslug[s.Slug] = s
	}
	fetcher := NewFetcher(fixturesDir)

	get := func(slug, epID string) ([]byte, Endpoint) {
		t.Helper()
		svc, ok := byslug[slug]
		if !ok {
			t.Fatalf("no service %s in registry", slug)
		}
		for _, ep := range svc.Endpoints {
			if ep.ID == epID {
				body, _, err := fetcher.Get(svc, ep)
				if err != nil {
					t.Fatal(err)
				}
				return body, ep
			}
		}
		t.Fatalf("no endpoint %s/%s", slug, epID)
		return nil, Endpoint{}
	}

	// Cloudflare's well-known edge range.
	body, ep := get("cloudflare", "api")
	raw, err := parsers[ep.Format](body, "*")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(raw, "173.245.48.0/20") {
		t.Error("cloudflare: expected 173.245.48.0/20 in edge ranges")
	}

	// AWS: the full set is large; the S3 selection is a strict subset.
	body, ep = get("aws", "main")
	all, _ := parsers[ep.Format](body, "*")
	s3, _ := parsers[ep.Format](body, "service=S3")
	if len(all) < 1000 {
		t.Errorf("aws all: got %d ranges, want >= 1000", len(all))
	}
	if len(s3) == 0 || len(s3) >= len(all) {
		t.Errorf("aws s3: got %d ranges, want 0 < n < %d", len(s3), len(all))
	}

	// Stripe API IPs are bare IPs; normalize must emit /32s.
	body, ep = get("stripe", "api")
	raw, _ = parsers[ep.Format](body, "*")
	v4, v6 := normalize(raw, func(string, ...any) {})
	if len(v6) != 0 {
		t.Errorf("stripe api: unexpected ipv6 %v", v6)
	}
	for _, c := range v4 {
		if !strings.HasSuffix(c, "/32") {
			t.Errorf("stripe api: %s not a /32", c)
		}
	}

	// Intercom outbound-webhook selection must be a strict subset of all.
	body, ep = get("intercom", "us")
	outbound, _ := parsers[ep.Format](body, "service=INTERCOM-OUTBOUND")
	allIC, _ := parsers[ep.Format](body, "*")
	if len(outbound) == 0 || len(outbound) >= len(allIC) {
		t.Errorf("intercom: outbound %d, all %d — want strict subset", len(outbound), len(allIC))
	}
}

func TestNormalize(t *testing.T) {
	warns := 0
	v4, v6 := normalize(
		[]string{"1.2.3.4", "1.2.3.4/32", "10.0.0.0/8", "0.0.0.0/0", "2600::/32", "bogus", "8.8.8.0/24"},
		func(string, ...any) { warns++ },
	)
	if want := []string{"1.2.3.4/32", "8.8.8.0/24"}; strings.Join(v4, ",") != strings.Join(want, ",") {
		t.Errorf("v4 = %v, want %v", v4, want)
	}
	if want := []string{"2600::/32"}; strings.Join(v6, ",") != strings.Join(want, ",") {
		t.Errorf("v6 = %v, want %v", v6, want)
	}
	if warns != 3 { // 10.0.0.0/8 private, 0.0.0.0/0 default, bogus invalid
		t.Errorf("warns = %d, want 3", warns)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
