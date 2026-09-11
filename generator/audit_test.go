package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The page below is the shape that hid Zscaler's announced ranges: a second
// table, under its own heading, that no purpose selects. The report has to
// name it. This is the check the expedition depends on most, because it is the
// one a human skips when the first table matched.
const auditPageTwoTables = `<html><body>
<h2>Cloud enforcement node ranges</h2>
<table><tr><td>198.51.100.0/24</td></tr></table>
<h2>Announced ranges we advise all customers to allow</h2>
<table><tr><td>203.0.113.0/24</td></tr></table>
</body></html>`

func auditFixture(t *testing.T, purposes []auditPurpose) (*SourceService, []auditEndpoint) {
	t.Helper()
	svc := &SourceService{
		Slug: "example", Name: "Example", Category: "test",
		Classification: "dedicated", Provenance: "https://example.test/ips",
		Verified: "2026-08-01",
	}
	ep := Endpoint{ID: "docs", URL: "https://example.test/ips", Format: "html-cidr-extract"}
	for _, p := range purposes {
		ep.Purposes = append(ep.Purposes, p.decl)
	}
	svc.Endpoints = []Endpoint{ep}
	return svc, []auditEndpoint{{ep: ep, from: ep.URL, body: []byte(auditPageTwoTables), purposes: purposes}}
}

func TestAuditNamesTheSectionWeDoNotConsume(t *testing.T) {
	svc, eps := auditFixture(t, []auditPurpose{{
		decl: PurposeDecl{Key: "cenr", Direction: "egress", Select: "section=Cloud enforcement"},
		v4:   []string{"198.51.100.0/24"},
	}})
	var b strings.Builder
	writeAuditReport(&b, svc, eps)
	got := b.String()

	if !strings.Contains(got, "IN THE DOCUMENT, NOT PUBLISHED") || !strings.Contains(got, "203.0.113.0/24") {
		t.Errorf("report did not surface the unpublished range:\n%s", got)
	}
	if !strings.Contains(got, "Announced ranges we advise all customers to allow") {
		t.Errorf("report did not name the section holding it:\n%s", got)
	}
}

func TestAuditReportsExactCoverage(t *testing.T) {
	svc, eps := auditFixture(t, []auditPurpose{{
		decl: PurposeDecl{Key: "all", Direction: "egress", Select: "*"},
		v4:   []string{"198.51.100.0/24", "203.0.113.0/24"},
	}})
	var b strings.Builder
	writeAuditReport(&b, svc, eps)
	got := b.String()
	if !strings.Contains(got, "coverage IPv4: EXACT") {
		t.Errorf("want exact coverage, got:\n%s", got)
	}
	if strings.Contains(got, "NOT PUBLISHED") {
		t.Errorf("reported a gap where the sets are identical:\n%s", got)
	}
}

// A purpose that is the union of the others has to carry aggregate: true, or a
// subscription wildcard takes the union AND every slice, spending a consumer's
// security-group quota twice on the same addresses.
func TestAuditFlagsUnmarkedAggregate(t *testing.T) {
	purposes := []auditPurpose{
		{decl: PurposeDecl{Key: "all", Direction: "egress"}, v4: []string{"198.51.100.0/24", "203.0.113.0/24"}},
		{decl: PurposeDecl{Key: "cenr", Direction: "egress"}, v4: []string{"198.51.100.0/24"}},
		{decl: PurposeDecl{Key: "announced", Direction: "egress"}, v4: []string{"203.0.113.0/24"}},
	}
	svc, eps := auditFixture(t, purposes)
	var b strings.Builder
	writeAuditReport(&b, svc, eps)
	if got := b.String(); !strings.Contains(got, "`all` contains cenr, announced and is NOT marked aggregate") {
		t.Errorf("unmarked aggregate not flagged:\n%s", got)
	}

	purposes[0].decl.Aggregate = true
	svc, eps = auditFixture(t, purposes)
	b.Reset()
	writeAuditReport(&b, svc, eps)
	if got := b.String(); !strings.Contains(got, "`all` is marked aggregate and contains 2 of 2 other purposes (cenr, announced).") {
		t.Errorf("correctly marked aggregate not confirmed:\n%s", got)
	}
}

func TestAuditChallengesBothDirections(t *testing.T) {
	svc, eps := auditFixture(t, []auditPurpose{{
		decl: PurposeDecl{Key: "all", Direction: "both"},
		v4:   []string{"198.51.100.0/24", "203.0.113.0/24"},
	}})
	var b strings.Builder
	writeAuditReport(&b, svc, eps)
	if got := b.String(); !strings.Contains(got, "CHALLENGE") {
		t.Errorf("a `both` purpose was not challenged:\n%s", got)
	}
}

func TestPageSectionsDoNotOverlap(t *testing.T) {
	secs := pageSections(auditPageTwoTables)
	if len(secs) < 2 {
		t.Fatalf("got %d sections, want the preamble plus two headings", len(secs))
	}
	for i := 1; i < len(secs); i++ {
		if secs[i].start < secs[i-1].end {
			t.Errorf("section %q starts before %q ends: a range would be counted twice",
				secs[i].title, secs[i-1].title)
		}
	}
}

func TestClaimTokensOnlyExtractsWhatCanBeChecked(t *testing.T) {
	for claim, want := range map[string]string{
		"7 days, by mailing list": "7 days",
		"30 days advance notice":  "30 days",
		"SNS topic arn:aws:sns:us-east-1:745490931007:atlassian-public-ip-changes": "arn:aws:sns:us-east-1:745490931007:atlassian-public-ip-changes",
	} {
		got := claimTokens(claim)
		if len(got) == 0 || got[0] != want {
			t.Errorf("claimTokens(%q) = %v, want %q first", claim, got, want)
		}
	}
	// Prose has nothing checkable in it, and pretending otherwise would report
	// a claim as verified because an unrelated word matched.
	if got := claimTokens("changes are announced on the status page"); len(got) != 0 {
		t.Errorf("claimTokens found tokens in prose: %v", got)
	}
}

// A citation with anything after the URL breaks the link on every surface it
// publishes to at once, and does it silently: the markdown still renders, just
// pointing nowhere. salesforce shipped that way in CATALOG.md, SOURCES.md, the
// feed document and its service page.
func TestCheckCitationURLRejectsLabels(t *testing.T) {
	bad := []string{
		"https://help.example.test/s/articleView?id=1 (IP Addresses to Allow)",
		"see https://example.test/ips",
		"example.test/ips",
		"",
	}
	for _, raw := range bad {
		if err := checkCitationURL(raw); err == nil {
			t.Errorf("checkCitationURL(%q) accepted a value that is not a bare URL", raw)
		}
	}
	for _, raw := range []string{
		"https://help.example.test/s/articleView?id=1&type=1",
		"http://example.test/ips.json",
	} {
		if err := checkCitationURL(raw); err != nil {
			t.Errorf("checkCitationURL(%q) rejected a valid URL: %v", raw, err)
		}
	}
}

// A rate limit is not link rot. One -audit-links run turned a live
// sentry-docs URL into a 429 through its own concurrency, and a weekly job
// that reports that as a dead citation teaches everyone to ignore it.
func TestCheckURLRetriesRateLimitsAndSeparatesThemFromRot(t *testing.T) {
	old := auditLinkRetryDelays
	auditLinkRetryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { auditLinkRetryDelays = old })

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, "the vendor commits to 30 days notice")
	}))
	defer srv.Close()

	c := checkURL(srv.Client(), srv.URL)
	if !c.ok() {
		t.Fatalf("a rate limit that clears on retry must end as 200, got %q", c.describe())
	}
	if hits != 3 {
		t.Errorf("made %d requests, want 3 (two rate limited, one good)", hits)
	}

	// A persistent rate limit is reported as unread, never as dead, and must
	// not be counted against the run.
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer limited.Close()
	c = checkURL(limited.Client(), limited.URL)
	if c.ok() || !c.unverified() {
		t.Errorf("a persistent 429 must be unverified, not ok and not dead: %q", c.describe())
	}

	// A genuine 404 is rot and must still fail.
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer gone.Close()
	c = checkURL(gone.Client(), gone.URL)
	if c.ok() || c.unverified() {
		t.Errorf("a 404 must count as dead: %q", c.describe())
	}
}

// Vendors spell periods out; our claims use digits. Microsoft writes "at least
// one week", LaunchDarkly "two weeks in advance", Stripe "seven days' notice",
// and all three claims are correct.
func TestClaimTokenMatchesSpelledOutPeriods(t *testing.T) {
	cases := []struct {
		text, token string
		want        bool
	}{
		{"when new ip addresses are added to service tags, they aren't used in azure for at least one week.", "1 week", true},
		{"normally we post the update two weeks in advance of the change taking effect.", "2 weeks", true},
		{"we provide seven days notice through that mailing list before making changes.", "7 days", true},
		{"we provide 7 days notice", "7 days", true},
		{"the page says nothing about a notice period", "30 days", false},
	}
	for _, c := range cases {
		if got := claimTokenPresent(c.text, c.token); got != c.want {
			t.Errorf("claimTokenPresent(%q) = %v, want %v", c.token, got, c.want)
		}
	}
}

// A certificate this machine cannot verify is not a dead citation. vultr's
// geofeed chains to a Sectigo root the macOS trust store lacks and the CI
// runner has, so it fetches fine in the publish workflow every ten minutes.
func TestTLSFailureIsUnreadRatherThanDead(t *testing.T) {
	tlsErr := urlCheck{url: "https://geofeed.example.test/", err: fmt.Errorf(`Get "https://geofeed.example.test/": tls: failed to verify certificate: x509: certificate signed by unknown authority`)}
	if !tlsErr.unverified() || tlsErr.ok() {
		t.Errorf("a certificate chain failure must be unread, not dead: %q", tlsErr.describe())
	}
	dnsErr := urlCheck{url: "https://gone.example.test/", err: fmt.Errorf(`Get "https://gone.example.test/": dial tcp: lookup gone.example.test: no such host`)}
	if dnsErr.unverified() {
		t.Errorf("a host that no longer resolves must count as dead: %q", dnsErr.describe())
	}
}

// A record's value is the hand-written half above the separator. Re-running
// -audit-out over a finished one replaced quoted vendor sentences with TODO
// markers, silently, which is how an afternoon of reading disappears.
func TestAuditRecordRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	svc, eps := auditFixture(t, []auditPurpose{{
		decl: PurposeDecl{Key: "all", Direction: "egress"},
		v4:   []string{"198.51.100.0/24"},
	}})

	if err := writeAuditRecord(dir, svc, eps, "report"); err != nil {
		t.Fatalf("first write should succeed: %v", err)
	}
	path := filepath.Join(dir, svc.Slug+".md")
	finished := "# audit: example\n\nverdict: pass\nevidence: \"the vendor sentence\"\n"
	if err := os.WriteFile(path, []byte(finished), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeAuditRecord(dir, svc, eps, "report"); err == nil {
		t.Error("writing over an existing record must fail")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != finished {
		t.Errorf("the finished record was modified:\n%s", got)
	}
}
