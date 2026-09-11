package main

// The audit report puts our published claims about one service beside the
// vendor document they were read from, and says which parts of that document
// we do not consume.
//
//	go run ./generator -audit stripe
//	go run ./generator -audit stripe -audit-out ../internal-docs/audits
//	go run ./generator -audit-links
//
// It decides nothing. Every line is either a fact read off the fetched body or
// a question for the person auditing. The checks it mechanises are the ones a
// human does badly at scale: comparing address sets in both directions, and
// noticing the table further down the page that nobody selected. Those two are
// what hid Zscaler's announced ranges and Duo's second product.

import (
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// auditSweepCap bounds the whole-document sweep. Azure's ServiceTags JSON is
// ~30 MB of prefixes and sweeping it says nothing useful, since the parser
// consumes the same file by key.
const auditSweepCap = 16 << 20

// auditSample bounds how many prefixes are printed per finding. The counts are
// exact; only the listing is trimmed.
const auditSample = 25

type auditPurpose struct {
	decl   PurposeDecl
	v4, v6 []string
	err    error
}

type auditEndpoint struct {
	ep       Endpoint
	from     string
	body     []byte
	fetchErr error
	partErr  error
	purposes []auditPurpose
}

func runAudit(slug string, opts buildOptions, out io.Writer) error {
	reg, err := LoadRegistry(opts.SourcesPath)
	if err != nil {
		return err
	}
	var svc *SourceService
	for i := range reg.Services {
		if reg.Services[i].Slug == slug {
			svc = &reg.Services[i]
		}
	}
	if svc == nil {
		for _, np := range reg.NonPublishers {
			if np.Slug == slug {
				return auditNonPublisher(np, out)
			}
		}
		return fmt.Errorf("no service or non-publisher %q in %s", slug, opts.SourcesPath)
	}

	fetcher := NewFetcher(opts.FixturesDir)
	eps := make([]auditEndpoint, 0, len(svc.Endpoints))
	for _, ep := range svc.Endpoints {
		res := auditEndpoint{ep: ep}
		res.body, res.from, res.fetchErr = fetcher.Get(*svc, ep)
		if res.fetchErr == nil {
			if check, ok := partitionChecks[ep.Format]; ok {
				res.partErr = check(res.body, ep.Purposes)
			}
			parse, ok := parsers[ep.Format]
			for _, decl := range ep.Purposes {
				p := auditPurpose{decl: decl}
				if !ok {
					p.err = errUnimplemented{ep.Format}
				} else if raw, err := parse(res.body, decl.Select); err != nil {
					p.err = err
				} else {
					p.v4, p.v6 = normalize(raw, func(string, ...any) {})
				}
				res.purposes = append(res.purposes, p)
			}
		}
		eps = append(eps, res)
	}

	var b strings.Builder
	writeAuditReport(&b, svc, eps)
	report := b.String()
	if _, err := io.WriteString(out, report); err != nil {
		return err
	}
	if opts.AuditOut != "" {
		return writeAuditRecord(opts.AuditOut, svc, eps, report)
	}
	return nil
}

func writeAuditReport(w io.Writer, svc *SourceService, eps []auditEndpoint) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }

	p("## %s (%s)", svc.Slug, svc.Name)
	p("")
	p("category: %s   classification: %s", svc.Category, svc.Classification)
	p("provenance: %s", orDash(svc.Provenance))
	p("verified: %s", verifiedAge(svc.Verified))
	if svc.Cadence != "" {
		p("cadence: %q", svc.Cadence)
	}
	if svc.Notes != "" {
		p("notes: %s", svc.Notes)
	}
	if svc.NeedsAttention != "" {
		p("needs_attention: %s", svc.NeedsAttention)
	}
	if svc.Guard != nil {
		p("GUARD OVERRIDE ACTIVE: max_removed_frac=%v (%s)", *svc.Guard.MaxRemovedFrac, svc.Guard.Reason)
	}

	// Purposes, directions, aggregate flags, and the quota cost of each.
	p("")
	p("### purposes")
	p("")
	dirs := map[string]int{}
	var allV4, allV6 []string
	p("| key | direction | evidence | aggregate | endpoint | v4 | v6 | v4 addresses |")
	p("|---|---|---|---|---|---|---|---|")
	for _, e := range eps {
		for _, pu := range e.purposes {
			if pu.err != nil {
				p("| %s | %s | | | %s | \\- | \\- | PARSE ERROR: %v |", pu.decl.Key, pu.decl.Direction, e.ep.ID, pu.err)
				continue
			}
			dirs[pu.decl.Direction]++
			allV4 = append(allV4, pu.v4...)
			allV6 = append(allV6, pu.v6...)
			set4, _ := splitFamilies(pu.v4)
			evidence := "**MISSING**"
			if pu.decl.DirectionEvidence != "" {
				evidence = "cited"
			}
			p("| %s | %s | %s | %s | %s | %d | %d | %s |",
				pu.decl.Key, pu.decl.Direction, evidence, yesNo(pu.decl.Aggregate), e.ep.ID,
				len(pu.v4), len(pu.v6), comma(set4.size()))
		}
	}
	p("")
	var dirParts []string
	for _, d := range sortedKeys(dirs) {
		dirParts = append(dirParts, fmt.Sprintf("%d %s", dirs[d], d))
	}
	p("directions: %s", strings.Join(dirParts, ", "))
	if dirs["both"] > 0 {
		p("")
		p("CHALLENGE: %d purposes claim `both`. That is the loosest claim available", dirs["both"])
		p("and the one most likely to be a default rather than a finding. Direction is")
		p("from the WORKLOAD's point of view, and vendor labels routinely invert.")
	}
	if svc.Notice != "" && svc.NoticeEvidence == "" {
		p("MISSING: notice is set with no notice_evidence")
	}
	for _, e := range eps {
		if e.ep.Detection.Push != "" && e.ep.Detection.PushEvidence == "" {
			p("MISSING: %s detection.push is set with no push_evidence", e.ep.ID)
		}
	}

	// Union check: is one purpose the union of the others? Marking it
	// aggregate is what keeps a subscription wildcard from paying for the same
	// addresses twice.
	writeUnionCheck(w, eps)

	// Coverage, per endpoint, in both directions.
	for _, e := range eps {
		p("")
		p("### endpoint %s", e.ep.ID)
		p("")
		p("url: %s", e.ep.URL)
		if e.from != "" && e.from != e.ep.URL {
			p("fetched from: %s", e.from)
		}
		p("format: %s   poll: %s   render: %s", e.ep.Format, orDash(e.ep.Detection.Poll), orDash(e.ep.Render))
		if len(e.ep.Headers) > 0 {
			p("headers: %v", e.ep.Headers)
		}
		if e.ep.Detection.Push != "" {
			p("push: %s (%s)", e.ep.Detection.Push, orDash(e.ep.Detection.PushKind))
			p("push_evidence: %s", orDash(e.ep.Detection.PushEvidence))
		}
		if e.fetchErr != nil {
			p("")
			p("FETCH FAILED: %v", e.fetchErr)
			continue
		}
		p("body: %s bytes, sha256 %s", comma(big.NewInt(int64(len(e.body)))), sha256hex(e.body)[:16])
		if e.partErr != nil {
			p("PARTITION CHECK FAILED: %v", e.partErr)
		} else if _, ok := partitionChecks[e.ep.Format]; ok {
			p("partition check: pass (every vendor key is claimed by some purpose)")
		}

		var ourV4, ourV6 []string
		for _, pu := range e.purposes {
			ourV4 = append(ourV4, pu.v4...)
			ourV6 = append(ourV6, pu.v6...)
		}
		if len(e.body) > auditSweepCap {
			p("")
			p("document sweep SKIPPED: body is %s bytes, over the %d MB audit cap.",
				comma(big.NewInt(int64(len(e.body)))), auditSweepCap>>20)
			p("Compare this endpoint by hand, or by key against the parser.")
			continue
		}
		doc4, doc6 := splitFamilies(extractRanges(string(e.body)))
		our4, our6 := splitFamilies(append(append([]string{}, ourV4...), ourV6...))
		p("")
		p("Every address the document mentions, against every address we publish")
		p("from it. Both sides are address sets: a vendor re-expressing the same")
		p("space with different prefixes is not a difference.")
		p("")
		writeCoverage(w, "IPv4", doc4, our4)
		writeCoverage(w, "IPv6", doc6, our6)

		if looksHTML(e.body) {
			writePageStructure(w, e)
		} else {
			writeLinkedFiles(w, e)
		}
	}
}

func writeCoverage(w io.Writer, family string, doc, ours addrSet) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	if doc.empty() && ours.empty() {
		return
	}
	gaps := doc.minus(ours)
	over := ours.minus(doc)
	switch {
	case gaps.empty() && over.empty():
		p("coverage %s: EXACT. %s addresses, identical in both directions.", family, comma(ours.size()))
	default:
		p("coverage %s: %s addresses published, %s addresses in the document.",
			family, comma(ours.size()), comma(doc.size()))
	}
	if !gaps.empty() {
		list := sortCIDRs(gaps.cidrs())
		p("")
		p("  IN THE DOCUMENT, NOT PUBLISHED: %s addresses in %d ranges.",
			comma(gaps.size()), len(list))
		p("  Either the vendor partitions something we do not carry, or the page")
		p("  contains addresses that are not ranges at all (examples, RFC1918")
		p("  explanations, SVG path coordinates). Read them in context.")
		writeSample(w, list)
	}
	if !over.empty() {
		list := sortCIDRs(over.cidrs())
		p("")
		p("  PUBLISHED, NOT IN THE DOCUMENT: %s addresses in %d ranges.",
			comma(over.size()), len(list))
		p("  We are publishing addresses this document does not state. If another")
		p("  endpoint feeds the same purpose that is expected; otherwise it is an")
		p("  overshoot and consumers are allowing more than the vendor asked for.")
		writeSample(w, list)
	}
}

func writeSample(w io.Writer, list []string) {
	shown := list
	if len(shown) > auditSample {
		shown = shown[:auditSample]
	}
	for _, c := range shown {
		fmt.Fprintf(w, "    %s\n", c)
	}
	if len(list) > len(shown) {
		fmt.Fprintf(w, "    ... and %d more\n", len(list)-len(shown))
	}
}

func writeUnionCheck(w io.Writer, eps []auditEndpoint) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	type namedPurpose struct {
		key       string
		aggregate bool
		v4, v6    addrSet
	}
	var all []namedPurpose
	for _, e := range eps {
		for _, pu := range e.purposes {
			if pu.err != nil {
				continue
			}
			s4, s6 := splitFamilies(append(append([]string{}, pu.v4...), pu.v6...))
			all = append(all, namedPurpose{pu.decl.Key, pu.decl.Aggregate, s4, s6})
		}
	}
	if len(all) < 2 {
		return
	}

	// Purposes publishing the identical address set are reported once, as a
	// group. Datadog names nine products whose ranges are all one /20, and
	// listing every pairwise containment would bury the finding that matters:
	// a consumer pays the security-group quota once per purpose, so identical
	// purposes cost N times for one set of addresses.
	sizeOf := func(n namedPurpose) *big.Int {
		return new(big.Int).Add(n.v4.size(), n.v6.size())
	}
	equal := func(a, b namedPurpose) bool {
		return a.v4.minus(b.v4).empty() && b.v4.minus(a.v4).empty() &&
			a.v6.minus(b.v6).empty() && b.v6.minus(a.v6).empty()
	}
	var lines []string
	grouped := map[string]bool{}
	for i, a := range all {
		if grouped[a.key] || sizeOf(a).Sign() == 0 {
			continue
		}
		group := []string{a.key}
		for _, b := range all[i+1:] {
			if !grouped[b.key] && equal(a, b) {
				group = append(group, b.key)
				grouped[b.key] = true
			}
		}
		if len(group) > 1 {
			grouped[a.key] = true
			lines = append(lines, fmt.Sprintf(
				"- `%s` publish IDENTICAL address sets (%s addresses each). The vendor names them separately, but a consumer pays the security-group quota once per purpose, so this costs %d times for one set of addresses.",
				strings.Join(group, "`, `"), comma(sizeOf(a)), len(group)))
		}
	}

	// An aggregate need not swallow every other purpose. Duo's mfa is the union
	// of ten residency areas and deliberately excludes trusted-endpoints, a
	// different product on the same page. So report containment and let the
	// person auditing decide what it means, rather than asserting a flag is
	// wrong because one purpose sits outside it.
	for _, cand := range all {
		var covered, outside []string
		for _, o := range all {
			// An empty set is contained in everything, which says nothing.
			if o.key == cand.key || sizeOf(o).Sign() == 0 || equal(cand, o) {
				continue
			}
			if o.v4.minus(cand.v4).empty() && o.v6.minus(cand.v6).empty() {
				covered = append(covered, o.key)
			} else {
				outside = append(outside, o.key)
			}
		}
		switch {
		case cand.aggregate && len(covered) == 0:
			lines = append(lines, fmt.Sprintf(
				"- `%s` is marked aggregate but contains no other purpose. The flag is wrong: a wildcard subscription is skipping it for nothing.", cand.key))
		case cand.aggregate:
			line := fmt.Sprintf("- `%s` is marked aggregate and contains %d of %d other purposes (%s).",
				cand.key, len(covered), len(covered)+len(outside), strings.Join(covered, ", "))
			if len(outside) > 0 {
				line += fmt.Sprintf(" Outside it: %s. Confirm that is deliberate rather than a purpose added after the aggregate was defined.",
					strings.Join(outside, ", "))
			}
			lines = append(lines, line)
		case len(covered) >= 1:
			lines = append(lines, fmt.Sprintf(
				"- `%s` contains %s and is NOT marked aggregate. A wildcard subscription therefore takes it AND everything inside it, spending a consumer's security-group quota twice on the same addresses. Marking it aggregate removes it from wildcards, so do that only if the parts are the sensible default.",
				cand.key, strings.Join(covered, ", ")))
		}
	}
	if len(lines) == 0 {
		return
	}
	p("")
	p("### containment and aggregate flags")
	p("")
	for _, l := range lines {
		p("%s", l)
	}
}

// --- page structure: what is on the page that we do not consume -------------

var (
	reTable = regexp.MustCompile(`(?is)<table[^>]*>.*?</table>`)
	reHref  = regexp.MustCompile(`(?i)href=["']([^"'\s>]+)["']`)
	reDataF = regexp.MustCompile(`(?i)\.(json|txt|csv|xml|ya?ml)(\?|$)`)
)

func looksHTML(body []byte) bool {
	head := strings.ToLower(string(body[:min(len(body), 4096)]))
	return strings.Contains(head, "<html") || strings.Contains(head, "<!doctype html") ||
		strings.Contains(head, "<table") || strings.Contains(head, "<body")
}

type pageSection struct {
	level      int
	title      string
	start, end int
}

// pageSections splits an HTML document into non-overlapping regions, one per
// heading plus a preamble. Non-overlapping matters here: every range in the
// document must be attributed to exactly one section, or a table that appears
// under two nested headings would be counted twice.
func pageSections(html string) []pageSection {
	marks := reHeading.FindAllStringSubmatchIndex(html, -1)
	if len(marks) == 0 {
		return []pageSection{{level: 0, title: "(whole document)", start: 0, end: len(html)}}
	}
	out := []pageSection{}
	if marks[0][0] > 0 {
		out = append(out, pageSection{level: 0, title: "(before the first heading)", start: 0, end: marks[0][0]})
	}
	for i, m := range marks {
		end := len(html)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		out = append(out, pageSection{
			level: int(html[m[2]] - '0'),
			title: strings.TrimSpace(reTagStrip.ReplaceAllString(html[m[4]:m[5]], " ")),
			start: m[1],
			end:   end,
		})
	}
	return out
}

func writePageStructure(w io.Writer, e auditEndpoint) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	html := reScriptStyle.ReplaceAllString(string(e.body), " ")

	var ourV4, ourV6 []string
	for _, pu := range e.purposes {
		ourV4 = append(ourV4, pu.v4...)
		ourV6 = append(ourV6, pu.v6...)
	}
	our4, our6 := splitFamilies(append(append([]string{}, ourV4...), ourV6...))

	classify := func(fragment string) (total int, unconsumed []string) {
		v4, v6 := normalize(extractRanges(fragment), func(string, ...any) {})
		for _, c := range append(append([]string{}, v4...), v6...) {
			total++
			s4, s6 := splitFamilies([]string{c})
			if !s4.minus(our4).empty() || !s6.minus(our6).empty() {
				unconsumed = append(unconsumed, c)
			}
		}
		return total, unconsumed
	}

	p("")
	p("#### page structure not consumed")
	p("")
	p("Sections of the page that contain addresses, and whether those addresses")
	p("end up published. This is the check that found Duo's second product table")
	p("and the Zscaler range set the vendor advises every customer to use.")
	p("")
	p("| section | ranges | not published |")
	p("|---|---|---|")
	any := false
	for _, s := range pageSections(html) {
		total, unconsumed := classify(html[s.start:s.end])
		if total == 0 {
			continue
		}
		any = true
		note := "none"
		if len(unconsumed) > 0 {
			note = fmt.Sprintf("**%d**: %s", len(unconsumed), strings.Join(firstN(sortCIDRs(unconsumed), 4), ", "))
		}
		p("| h%d %s | %d | %s |", s.level, mdCell(s.title), total, note)
	}
	if !any {
		p("| (no section of this page yields a valid address) | 0 | |")
	}

	tables := reTable.FindAllStringIndex(html, -1)
	if len(tables) > 0 {
		p("")
		p("| table | nearest heading | ranges | not published |")
		p("|---|---|---|---|")
		secs := pageSections(html)
		for i, t := range tables {
			total, unconsumed := classify(html[t[0]:t[1]])
			heading := ""
			for _, s := range secs {
				if s.start <= t[0] {
					heading = s.title
				}
			}
			note := "none"
			if len(unconsumed) > 0 {
				note = fmt.Sprintf("**%d**", len(unconsumed))
			}
			p("| %d | %s | %d | %s |", i+1, mdCell(heading), total, note)
		}
	}

	writeLinkedFiles(w, e)
}

// writeLinkedFiles lists machine-readable files the page links, and says which
// of them we already fetch. A vendor's JSON beside the HTML is routinely the
// better source and sometimes a different one: Svix's carries a region the
// rendered page never shows.
func writeLinkedFiles(w io.Writer, e auditEndpoint) {
	p := func(format string, args ...any) { fmt.Fprintf(w, format+"\n", args...) }
	base, err := url.Parse(e.ep.URL)
	if err != nil {
		return
	}
	seen := map[string]bool{}
	var links []string
	for _, m := range reHref.FindAllStringSubmatch(string(e.body), -1) {
		href := strings.TrimSpace(m[1])
		if !reDataF.MatchString(href) {
			continue
		}
		u, err := base.Parse(href)
		if err != nil {
			continue
		}
		s := u.String()
		if seen[s] {
			continue
		}
		seen[s] = true
		links = append(links, s)
	}
	if len(links) == 0 {
		return
	}
	sort.Strings(links)
	p("")
	p("linked machine-readable files on this page:")
	for _, l := range links {
		mark := ""
		if l == e.ep.URL {
			mark = "  (this endpoint)"
		}
		p("- %s%s", l, mark)
	}
	p("")
	p("Read the href, never the link text. Svix renders `webhooks-ips.json` and")
	p("links `/webhook-ips.json`.")
}

// --- non-publishers ---------------------------------------------------------

func auditNonPublisher(np NonPublisherY, out io.Writer) error {
	p := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	p("## %s (%s), recorded as a NON-PUBLISHER", np.Slug, np.Name)
	p("")
	p("vendor_position: %s", np.VendorPosition)
	p("evidence: %s", np.Evidence)
	p("")
	check := checkURL(auditClient(), np.Evidence)
	p("evidence URL: %s", check.describe())
	p("")
	p("This is a public claim that a named company does not publish something.")
	p("We have been wrong three times (Duo, HubSpot, Anthropic), and each time")
	p("the vendor's own page linked the ranges. Two questions to answer from the")
	p("page, not from a search summary:")
	p("")
	p("1. Does the page still state what we say it states, positively, about")
	p("   what the vendor DOES support? Absence cannot be evidenced.")
	p("2. Does the vendor publish ranges anywhere reachable, including a file")
	p("   linked from this page or a support article behind a different URL?")
	return nil
}

// --- link audit -------------------------------------------------------------

type urlCheck struct {
	url    string
	status string
	err    error
	body   []byte
}

func (c urlCheck) ok() bool { return c.err == nil && strings.HasPrefix(c.status, "200") }

// unverified separates "we could not read the page right now" from "the
// citation is dead". A rate limit or a 5xx says nothing about whether the claim
// still stands, and treating it as rot would make the weekly job cry wolf:
// github.com is cited many times over, and one -audit-links run turned a
// perfectly live sentry-docs URL into a 429 purely through its own concurrency.
func (c urlCheck) unverified() bool {
	if c.ok() {
		return false
	}
	// A certificate that this machine cannot build a chain for is a property of
	// this machine as often as of the vendor. geofeed.constant.com chains to
	// Sectigo Public Server Authentication Root R46, which the macOS trust store
	// lacks and the CI runner has, so vultr fails here and fetches cleanly in
	// the publish workflow every ten minutes. Every other transport failure
	// (DNS gone, connection refused, timeout) stays a hard failure.
	if c.err != nil {
		return strings.Contains(c.err.Error(), "x509:")
	}
	return strings.HasPrefix(c.status, "429") || strings.HasPrefix(c.status, "5")
}

func (c urlCheck) describe() string {
	switch {
	case c.err != nil && c.unverified():
		return "UNREAD (certificate chain, may be this machine): " + firstLine(c.err.Error())
	case c.err != nil:
		return "FAILED: " + firstLine(c.err.Error())
	case c.unverified():
		return c.status + " (could not read, not proof of rot)"
	}
	return c.status
}

// auditClient identifies itself as a browser. Evidence URLs are pages a human
// follows to check a claim, so what a browser sees is the thing being
// verified, not what our generator's User-Agent sees. Some hosts refuse both
// (help.twilio.com 403s browsers too), which the report states rather than
// hides.
func auditClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// auditLinkRetryDelays paces retries of a citation check. A shared runner
// hitting one host several times over collects 429s that have nothing to do
// with the citation being valid.
var auditLinkRetryDelays = []time.Duration{2 * time.Second, 8 * time.Second}

func checkURL(client *http.Client, u string) urlCheck {
	for attempt := 0; ; attempt++ {
		c := checkURLOnce(client, u)
		if c.ok() || !c.unverified() || attempt >= len(auditLinkRetryDelays) {
			return c
		}
		time.Sleep(auditLinkRetryDelays[attempt])
	}
}

func checkURLOnce(client *http.Client, u string) urlCheck {
	c := urlCheck{url: u}
	if !strings.HasPrefix(u, "http") {
		c.err = fmt.Errorf("not an http(s) URL")
		return c
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		c.err = err
		return c
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		c.err = err
		return c
	}
	defer resp.Body.Close()
	c.status = resp.Status
	c.body, _ = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return c
}

type auditURL struct {
	url   string
	owner string // "<slug> provenance", "<slug> notice_evidence", ...
	claim string // the claim the URL is cited for, when there is one
}

func collectAuditURLs(reg *Registry) []auditURL {
	var out []auditURL
	for _, svc := range reg.Services {
		if svc.Provenance != "" {
			out = append(out, auditURL{svc.Provenance, svc.Slug + " provenance", ""})
		}
		if svc.NoticeEvidence != "" {
			out = append(out, auditURL{svc.NoticeEvidence, svc.Slug + " notice_evidence", svc.Notice})
		}
		for _, ep := range svc.Endpoints {
			if ep.Detection.PushEvidence != "" {
				out = append(out, auditURL{ep.Detection.PushEvidence, svc.Slug + "/" + ep.ID + " push_evidence", ep.Detection.Push})
			}
		}
	}
	for _, np := range reg.NonPublishers {
		out = append(out, auditURL{np.Evidence, np.Slug + " non-publisher evidence", np.VendorPosition})
	}
	return out
}

// reClaimPeriod picks the one part of a claim string that can be checked
// mechanically: a stated period. Everything else in these claims is prose, and
// pretending a substring search verifies it would be worse than saying plainly
// that a human has to read the page.
var reClaimPeriod = regexp.MustCompile(`(?i)\b\d+\s*(?:business\s+)?(?:day|days|hour|hours|week|weeks|month|months)\b`)

var reARN = regexp.MustCompile(`arn:[a-zA-Z0-9:_./-]+`)

func claimTokens(claim string) []string {
	var out []string
	out = append(out, reClaimPeriod.FindAllString(claim, -1)...)
	out = append(out, reARN.FindAllString(claim, -1)...)
	return out
}

// numberWords lets a period written in digits match a vendor who spells it out.
// Every notice claim in the catalog uses digits; Microsoft writes "at least one
// week", LaunchDarkly "two weeks in advance" and Stripe "seven days' notice".
// Without this, three correct claims report as missing on every weekly run,
// which is how a checker teaches people to ignore it.
var numberWords = map[string]string{
	"1": "one", "2": "two", "3": "three", "4": "four", "5": "five", "6": "six",
	"7": "seven", "8": "eight", "9": "nine", "10": "ten", "11": "eleven",
	"12": "twelve", "14": "fourteen", "15": "fifteen", "20": "twenty",
	"24": "twenty-four", "30": "thirty", "45": "forty-five", "48": "forty-eight",
	"60": "sixty", "90": "ninety",
}

var reLeadingNumber = regexp.MustCompile(`^(\d+)(\s*)(.*)$`)

// periodVariants returns the forms of a token worth looking for on a page.
func periodVariants(token string) []string {
	out := []string{token}
	m := reLeadingNumber.FindStringSubmatch(strings.TrimSpace(token))
	if m == nil {
		return out
	}
	if word, ok := numberWords[m[1]]; ok {
		out = append(out, word+" "+m[3])
	}
	return out
}

// claimTokenPresent reports whether any spelling of the token appears in text,
// which must already be lowercased and whitespace-collapsed.
func claimTokenPresent(text, token string) bool {
	for _, v := range periodVariants(token) {
		if strings.Contains(text, strings.ToLower(v)) {
			return true
		}
	}
	return false
}

func runAuditLinks(opts buildOptions, out io.Writer) error {
	reg, err := LoadRegistry(opts.SourcesPath)
	if err != nil {
		return err
	}
	urls := collectAuditURLs(reg)

	// One check per distinct URL: several services cite the same page, and the
	// same page is often both provenance and notice evidence.
	byURL := map[string][]auditURL{}
	var order []string
	for _, u := range urls {
		if _, seen := byURL[u.url]; !seen {
			order = append(order, u.url)
		}
		byURL[u.url] = append(byURL[u.url], u)
	}

	client := auditClient()
	results := make([]urlCheck, len(order))
	sem := make(chan struct{}, 6)
	done := make(chan int, len(order))
	for i, u := range order {
		go func(i int, u string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = checkURL(client, u)
			done <- i
		}(i, u)
	}
	for range order {
		<-done
	}

	p := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	p("# evidence link audit")
	p("")
	p("%d citations across %d distinct URLs, fetched %s.",
		len(urls), len(order), time.Now().UTC().Format(time.RFC3339))
	p("")
	failed, unverified, unchecked := 0, 0, 0
	p("| status | citation | claim still on the page | url |")
	p("|---|---|---|---|")
	for i, u := range order {
		r := results[i]
		switch {
		case r.ok():
		case r.unverified():
			unverified++
		default:
			failed++
		}
		text := strings.ToLower(pageText(r.body))
		for _, cite := range byURL[u] {
			verdict := "no token to check"
			if toks := claimTokens(cite.claim); len(toks) > 0 {
				var missing []string
				for _, t := range toks {
					if !claimTokenPresent(text, t) {
						missing = append(missing, t)
					}
				}
				if len(missing) == 0 {
					verdict = "yes: " + strings.Join(toks, ", ")
				} else {
					verdict = "**NOT FOUND: " + strings.Join(missing, ", ") + "**"
				}
			} else if cite.claim != "" {
				unchecked++
			}
			if !r.ok() {
				verdict = "(page not fetched)"
			}
			p("| %s | %s | %s | %s |", mdCell(r.describe()), cite.owner, verdict, u)
		}
	}
	p("")
	p("%d of %d URLs are dead or forbidden. %d more could not be read this run "+
		"(rate limit, 5xx, or a certificate this machine cannot verify), which is "+
		"not evidence either way.", failed, len(order), unverified)
	p("")
	p("A period found on the page is not the claim verified: it means the number")
	p("still appears somewhere in the text. Whether it covers the ranges we")
	p("publish is a reading question. PagerDuty's 30 days covers their REST API")
	p("addresses, not the webhook ranges in this feed, which is why they score")
	p("nothing for notice.")
	if failed > 0 {
		return fmt.Errorf("%d evidence URLs are dead or forbidden", failed)
	}
	return nil
}

// --- record -----------------------------------------------------------------

func writeAuditRecord(dir string, svc *SourceService, eps []auditEndpoint, report string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, svc.Slug+".md")
	// Everything above the separator in a record is written by hand, and it is
	// the only part that matters. Re-running the tool over a finished record
	// would replace quoted vendor sentences with TODO markers and say nothing,
	// which is how an afternoon of reading disappears.
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists and is hand-written; refusing to overwrite it. "+
			"Run without -audit-out and copy the section you need, or delete the file first", path)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# audit: %s\n\n", svc.Slug)
	fmt.Fprintf(&b, "audited: %s\n", time.Now().UTC().Format("2006-01-02"))
	fmt.Fprintf(&b, "verdict: TODO (pass | fixed | blocked)\n")
	fmt.Fprintf(&b, "source: %s\n", orDash(svc.Provenance))
	fmt.Fprintf(&b, "\ndirection:\n")
	for _, e := range eps {
		for _, pu := range e.purposes {
			fmt.Fprintf(&b, "  %s -> %s\n", pu.decl.Key, pu.decl.Direction)
			fmt.Fprintf(&b, "    evidence: TODO quote the vendor sentence that establishes this\n")
		}
	}
	fmt.Fprintf(&b, "\npurposes: TODO (matches the vendor's partition | split applied | genuinely single)\n")
	fmt.Fprintf(&b, "  evidence: TODO\n")
	fmt.Fprintf(&b, "\ncoverage: TODO read off the report below\n")
	fmt.Fprintf(&b, "\nnot consumed: TODO what on the page we deliberately skip, and why\n")
	fmt.Fprintf(&b, "\nnotes:\n")
	fmt.Fprintf(&b, "\n---\n\n")
	fmt.Fprintf(&b, "Generated by `go run ./generator -audit %s`. The tool decides nothing:\n", svc.Slug)
	fmt.Fprintf(&b, "the fields above are filled in by reading the vendor's page.\n\n")
	b.WriteString(report)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// --- small helpers ----------------------------------------------------------

func pageText(body []byte) string {
	t := reScriptStyle.ReplaceAllString(string(body), " ")
	t = reTagStrip.ReplaceAllString(t, " ")
	return strings.Join(strings.Fields(t), " ")
}

func verifiedAge(v string) string {
	if v == "" {
		return "(not recorded)"
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return v + " (unparseable)"
	}
	days := int(time.Since(t).Hours() / 24)
	stale := ""
	if days > 180 {
		stale = ", OVER SIX MONTHS"
	}
	return fmt.Sprintf("%s (%d days ago%s)", v, days, stale)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}

// mdCell keeps a vendor's heading from breaking the markdown table it lands in.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 90 {
		s = s[:90] + "..."
	}
	if s == "" {
		return "(untitled)"
	}
	return s
}

func firstN(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return append(append([]string{}, in[:n]...), "...")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
