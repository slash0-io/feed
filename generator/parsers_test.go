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

	// Stripe API publishes bare IPs. Lossless aggregation may merge adjacent
	// pairs, but the covered address count must EXACTLY equal the number of
	// unique published IPs — the zero-overshoot invariant.
	body, ep = get("stripe", "api")
	raw, _ = parsers[ep.Format](body, "*")
	v4, v6 := normalize(raw, func(string, ...any) {})
	if len(v6) != 0 {
		t.Errorf("stripe api: unexpected ipv6 %v", v6)
	}
	unique := map[string]bool{}
	for _, ip := range raw {
		unique[strings.TrimSpace(ip)] = true
	}
	var covered uint64
	for _, c := range v4 {
		p := netip.MustParsePrefix(c)
		covered += uint64(1) << (32 - p.Bits())
	}
	if covered != uint64(len(unique)) {
		t.Errorf("stripe api: %d addresses covered by %d CIDRs, want exactly %d published IPs (overshoot!)",
			covered, len(v4), len(unique))
	}

	// Intercom outbound-webhook selection must be a strict subset of all.
	body, ep = get("intercom", "us")
	outbound, _ := parsers[ep.Format](body, "service=INTERCOM-OUTBOUND")
	allIC, _ := parsers[ep.Format](body, "*")
	if len(outbound) == 0 || len(outbound) >= len(allIC) {
		t.Errorf("intercom: outbound %d, all %d — want strict subset", len(outbound), len(allIC))
	}

	// Anthropic docs page: section scoping must isolate inbound ranges and
	// keep phased-out addresses out of the outbound capture.
	body, ep = get("anthropic", "docs")
	raw, err = parsers[ep.Format](body, "section=Inbound IP addresses")
	if err != nil {
		t.Fatal(err)
	}
	v4in, v6in := normalize(raw, func(string, ...any) {})
	if strings.Join(v4in, ",") != "160.79.104.0/23" || strings.Join(v6in, ",") != "2607:6bc0::/48" {
		t.Errorf("anthropic inbound = %v %v, want [160.79.104.0/23] [2607:6bc0::/48]", v4in, v6in)
	}
	raw, err = parsers[ep.Format](body, "section=Outbound IP addresses;exclude=Phased out")
	if err != nil {
		t.Fatal(err)
	}
	v4out, _ := normalize(raw, func(string, ...any) {})
	if !contains(v4out, "160.79.104.0/21") {
		t.Errorf("anthropic outbound missing 160.79.104.0/21: %v", v4out)
	}
	if contains(v4out, "34.162.46.92/32") {
		t.Errorf("anthropic outbound contains phased-out 34.162.46.92/32: %v", v4out)
	}

	// AppDynamics puts both directions under one region heading: the addresses
	// its platform connects FROM sit beside the Synthetic Server addresses an
	// agent connects TO. A heading select cannot tell them apart, so the
	// from= marker must, and the two purposes must stay disjoint.
	body, ep = get("appdynamics", "docs")
	rawSrc, err := parsers[ep.Format](body, "section=*;from=All traffic originating from")
	if err != nil {
		t.Fatal(err)
	}
	v4src, _ := normalize(rawSrc, func(string, ...any) {})
	rawAgents, err := parsers[ep.Format](body, "section=Synthetic Hosted Agents")
	if err != nil {
		t.Fatal(err)
	}
	v4agents, _ := normalize(rawAgents, func(string, ...any) {})
	if len(v4src) != 27 {
		t.Errorf("appdynamics platform-sources = %d ranges, want 27", len(v4src))
	}
	for _, dest := range []string{"52.40.35.5/32", "52.201.103.47/32", "52.48.243.82/32"} {
		if contains(v4src, dest) {
			t.Errorf("appdynamics platform-sources contains Synthetic Server destination %s", dest)
		}
	}
	// Smartlook is a different vendor entirely and must never be published here.
	for _, sl := range []string{"52.39.171.210/32", "52.59.31.101/32"} {
		if contains(v4src, sl) || contains(v4agents, sl) {
			t.Errorf("appdynamics publishes third-party Smartlook address %s", sl)
		}
	}
	for _, a := range v4agents {
		if contains(v4src, a) {
			t.Errorf("appdynamics purposes overlap on %s: ingress sources and hosted agents must be disjoint", a)
		}
	}
	// A marker that matches nothing must fail loudly. Publishing zero ranges is
	// how a page reorganisation becomes a silent mass removal.
	if _, err := parsers[ep.Format](body, "section=*;from=no such lead-in sentence"); err == nil {
		t.Error("html-cidr-extract: unmatched from= marker returned no error")
	}

	// Syntax highlighters wrap each octet in its own element, so stripping
	// tags to a space yields "3 . 217 . 146 . 166". WorkOS publishes that way
	// and would otherwise parse to nothing.
	body, ep = get("workos", "docs")
	rawWO, err := parsers[ep.Format](body, "section=Create an IP allowlist")
	if err != nil {
		t.Fatal(err)
	}
	v4wo, _ := normalize(rawWO, func(string, ...any) {})
	if len(v4wo) != 9 {
		t.Errorf("workos = %d ranges, want 9: %v", len(v4wo), v4wo)
	}
	if !contains(v4wo, "3.217.146.166/32") {
		t.Errorf("workos missing 3.217.146.166/32, octet repair failed: %v", v4wo)
	}
	// The repair must only close whitespace that already sits around a dot
	// between digits, so numbers in neighbouring cells are never welded into a
	// plausible-looking address.
	for _, fake := range extractRanges("<td>10</td><td>20</td><td>30</td><td>40</td>") {
		t.Errorf("adjacent cells fabricated an address: %q", fake)
	}

	// GitLab docs: the "IP range" section is exactly two dedicated ranges.
	body, ep = get("gitlab", "docs")
	raw, err = parsers[ep.Format](body, "section=IP range")
	if err != nil {
		t.Fatal(err)
	}
	v4gl, _ := normalize(raw, func(string, ...any) {})
	if strings.Join(v4gl, ",") != "34.74.90.64/28,34.74.226.0/24" {
		t.Errorf("gitlab = %v, want [34.74.90.64/28 34.74.226.0/24]", v4gl)
	}

	// Azure service tags (reduced fixture): tag selection must resolve.
	body, ep = get("azure", "service-tags")
	storage, err := parsers[ep.Format](body, "tag=Storage")
	if err != nil || len(storage) == 0 {
		t.Errorf("azure tag=Storage: %d ranges, err=%v", len(storage), err)
	}
	if _, err := parsers[ep.Format](body, "tag=NoSuchTag"); err == nil {
		t.Error("azure: expected error for unknown tag")
	}

	// Buildkite meta: three webhook /32s in the archived fixture.
	body, ep = get("buildkite", "meta")
	raw, _ = parsers[ep.Format](body, "webhook_ips")
	v4bk, _ := normalize(raw, func(string, ...any) {})
	if len(v4bk) != 3 || !contains(v4bk, "100.24.182.113/32") {
		t.Errorf("buildkite webhooks = %v, want 3 /32s incl 100.24.182.113/32", v4bk)
	}

	// Tenable reuses the AWS ip-ranges schema; scanner selection must resolve.
	body, ep = get("tenable", "ip-ranges")
	raw, _ = parsers[ep.Format](body, "service=tenable-scanners")
	if len(raw) < 30 || !contains(raw, "13.115.104.128/25") {
		t.Errorf("tenable scanners: %d ranges, want >=30 incl 13.115.104.128/25", len(raw))
	}

	// Elastic Cloud: both direction keys populated; unknown key errors.
	body, ep = get("elastic-cloud", "ips")
	ecIn, _ := parsers[ep.Format](body, "key=ingress_to_elastic")
	ecOut, _ := parsers[ep.Format](body, "key=egress_from_elastic")
	if len(ecIn) == 0 || len(ecOut) == 0 {
		t.Errorf("elastic-cloud: ingress %d, egress %d — want both non-empty", len(ecIn), len(ecOut))
	}
	if _, err := parsers[ep.Format](body, "key=nope"); err == nil {
		t.Error("elastic-cloud: expected error for unknown key")
	}

	// DocuSign: usage= selects across BOTH top-level arrays, so email_outbound
	// resolves even though it lives in email_ranges rather than ranges. The two
	// usages legitimately overlap (192.103.120.0/22 carries EU Connect traffic
	// and CLM email), so they are not asserted disjoint.
	body, ep = get("docusign", "ip-ranges")
	connect, err := parsers[ep.Format](body, "usage=connect_outbound")
	if err != nil {
		t.Fatal(err)
	}
	email, err := parsers[ep.Format](body, "usage=email_outbound")
	if err != nil {
		t.Fatal(err)
	}
	if len(connect) == 0 || len(email) == 0 {
		t.Fatalf("docusign: connect=%d email=%d, want both non-empty", len(connect), len(email))
	}
	// email_ranges is the second array; proving one of its entries resolves is
	// what distinguishes this from a parser that only ever reads `ranges`.
	inConnect := map[string]bool{}
	for _, c := range connect {
		inConnect[c] = true
	}
	onlyEmail := 0
	for _, c := range email {
		if !inConnect[c] {
			onlyEmail++
		}
	}
	if onlyEmail == 0 {
		t.Error("docusign: no email_outbound range is unique to email_ranges; is the second array being read?")
	}
	if _, err := parsers[ep.Format](body, "region=EU"); err == nil {
		t.Error("docusign-ranges: expected error for a non-usage select")
	}

	// Klaviyo: the JSON:API singleton yields exactly the two published blocks.
	body, ep = get("klaviyo", "allowlist")
	raw, err = parsers[ep.Format](body, "")
	if err != nil {
		t.Fatal(err)
	}
	v4kl, _ := normalize(raw, func(string, ...any) {})
	if strings.Join(v4kl, ",") != "207.186.206.0/24,207.211.192.0/20" {
		t.Errorf("klaviyo = %v, want [207.186.206.0/24 207.211.192.0/20]", v4kl)
	}
	// A payload for any other resource id is not the allowlist we asked for.
	if _, err := parsers[ep.Format]([]byte(`{"data":{"id":"other"}}`), ""); err == nil {
		t.Error("klaviyo-allowlist: expected error for unexpected resource id")
	}

	// Twilio SIP: signaling folds to the eight regional /30s; media is the /18.
	body, ep = get("twilio-sip", "docs")
	raw, _ = parsers[ep.Format](body, "section=Regional signaling IP gateways")
	v4tw, _ := normalize(raw, func(string, ...any) {})
	if len(v4tw) != 8 || !contains(v4tw, "54.172.60.0/30") {
		t.Errorf("twilio-sip signaling = %v, want 8 /30s incl 54.172.60.0/30", v4tw)
	}
	raw, _ = parsers[ep.Format](body, "section=Global media IP gateways")
	v4tm, _ := normalize(raw, func(string, ...any) {})
	if strings.Join(v4tm, ",") != "168.86.128.0/18" {
		t.Errorf("twilio-sip media = %v, want [168.86.128.0/18]", v4tm)
	}

	// Plaid: exactly the four published webhook source IPs.
	body, ep = get("plaid", "docs")
	raw, _ = parsers[ep.Format](body, "section=Configuring webhooks")
	v4pl, _ := normalize(raw, func(string, ...any) {})
	if want := "52.21.26.131/32,52.21.47.157/32,52.41.247.19/32,52.88.82.239/32"; strings.Join(v4pl, ",") != want {
		t.Errorf("plaid webhooks = %v, want %s", v4pl, want)
	}

	// Netskope: boundary addresses fold into the five dataplane blocks and the
	// AI Red Team subsection stays excluded.
	body, ep = get("netskope", "docs")
	raw, _ = parsers[ep.Format](body, "section=NewEdge IP Ranges for Allowlisting;exclude=AI Red Team")
	v4ns, _ := normalize(raw, func(string, ...any) {})
	if want := "8.36.116.0/24,8.39.144.0/24,31.186.239.0/24,162.10.0.0/17,163.116.128.0/17"; strings.Join(v4ns, ",") != want {
		t.Errorf("netskope dataplane = %v, want %s", v4ns, want)
	}

	// IBM Cloud: the front-end selection is public space only — the page's
	// RFC1918 back-end sections must not bleed into the output.
	body, ep = get("ibm-cloud", "docs")
	raw, _ = parsers[ep.Format](body, "section=Front-end (public) network")
	v4ib, _ := normalize(raw, func(string, ...any) {})
	if len(v4ib) < 30 {
		t.Errorf("ibm-cloud frontend: %d ranges, want >= 30", len(v4ib))
	}
	for _, c := range v4ib {
		p := netip.MustParsePrefix(c)
		if p.Addr().IsPrivate() {
			t.Errorf("ibm-cloud frontend: private range %s in output", c)
		}
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

func TestLosslessAggregation(t *testing.T) {
	v4, v6 := normalize(
		[]string{
			"8.8.8.0/24", "8.8.8.128/25", // subsumed: /25 inside /24
			"1.1.0.0/24", "1.1.1.0/24", // exact siblings -> 1.1.0.0/23
			"9.9.9.0/25", "9.9.10.0/25", // NOT siblings (different parents) -> untouched
			"2600::/33", "2600:0:8000::/33", // v6 siblings -> 2600::/32
		},
		func(string, ...any) { t.Error("unexpected warning") },
	)
	if want := "1.1.0.0/23,8.8.8.0/24,9.9.9.0/25,9.9.10.0/25"; strings.Join(v4, ",") != want {
		t.Errorf("v4 = %v, want %s", v4, want)
	}
	if want := "2600::/32"; strings.Join(v6, ",") != want {
		t.Errorf("v6 = %v, want %s", v6, want)
	}
	// Cascade: four /26 quarters collapse all the way to the /24.
	v4, _ = normalize([]string{"7.7.7.0/26", "7.7.7.64/26", "7.7.7.128/26", "7.7.7.192/26"}, func(string, ...any) {})
	if want := "7.7.7.0/24"; strings.Join(v4, ",") != want {
		t.Errorf("cascade v4 = %v, want %s", v4, want)
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
