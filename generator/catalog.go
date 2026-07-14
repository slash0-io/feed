package main

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

// Human-readable catalog rendering. Everything here derives from sources.yaml
// (slugs, purposes, directions, classifications) — not from fetched ranges —
// so output is deterministic and CI can enforce that the committed CATALOG.md
// never drifts from the registry.

var categoryOrder = []struct{ key, title string }{
	{"cloud", "Cloud / IaaS"},
	{"cdn", "CDN / Edge"},
	{"payments", "Payments & fintech"},
	{"observability", "Observability"},
	{"devtools", "Developer platforms & CI"},
	{"data-platform", "Data platforms"},
	{"identity", "Identity & auth"},
	{"comms", "Communications"},
	{"business-saas", "Business SaaS"},
	{"security", "Security"},
	{"ai", "AI APIs"},
}

func servicesByCategory(reg *Registry) map[string][]SourceService {
	m := map[string][]SourceService{}
	for _, s := range reg.Services {
		m[s.Category] = append(m[s.Category], s)
	}
	for _, ss := range m {
		sort.Slice(ss, func(i, j int) bool { return ss[i].Slug < ss[j].Slug })
	}
	return m
}

func purposeSummary(svc SourceService) string {
	var parts []string
	seen := map[string]bool{}
	for _, ep := range svc.Endpoints {
		for _, p := range ep.Purposes {
			if seen[p.Key] {
				continue
			}
			seen[p.Key] = true
			parts = append(parts, fmt.Sprintf("`%s` (%s)", p.Key, p.Direction))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func renderCatalogMarkdown(reg *Registry) string {
	var b strings.Builder
	b.WriteString(`# Service catalog

<!-- GENERATED from sources.yaml — do not edit by hand.
     Regenerate: go run ./generator -catalog CATALOG.md -->

Every service below publishes official IP ranges, verified against the vendor's own
publication. Use the **slug** and a **purpose** key with the Terraform provider:

` + "```hcl" + `
data "egress_ranges" "stripe_api" {
  service = "stripe"   # slug
  purpose = "api"      # purpose key
}
` + "```" + `

**Direction** is read from your workload's point of view: ` + "`egress`" + ` ranges are what you
connect *to* (security-group egress rules); ` + "`ingress`" + ` ranges are what the service
connects *from* — webhook and agent sources that belong in ingress rules.

**Classification**: ` + "`dedicated`" + ` = vendor-owned space, safe to pin · ` + "`mixed`" + ` = partly
shared or dynamic · ` + "`cdn-shared`" + ` = shared CDN ranges (pinning allowlists the whole CDN).

`)
	byCat := servicesByCategory(reg)
	for _, cat := range categoryOrder {
		services := byCat[cat.key]
		if len(services) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", cat.title)
		b.WriteString("| Slug | Service | Classification | Purposes | Source |\n|---|---|---|---|---|\n")
		for _, s := range services {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | [official ↗](%s) |\n",
				s.Slug, s.Name, s.Classification, purposeSummary(s), s.Provenance)
		}
		b.WriteString("\n")
	}

	b.WriteString(`## Services that do NOT publish pinnable ranges

These vendors state that IP allowlisting is unsupported or unreliable for their service.
Published in the feed (` + "`index.json` → `nonPublishers`" + `) so tooling can surface it.

| Service | Vendor position |
|---|---|
`)
	for _, np := range reg.NonPublishers {
		fmt.Fprintf(&b, "| %s | %s |\n", np.Name, strings.TrimSpace(np.VendorPosition))
	}
	b.WriteString("\nEvidence links for every row: [`sources.yaml`](sources.yaml) · methodology: [`research/SOURCES.md`](research/SOURCES.md)\n")
	return b.String()
}

// renderCatalogHTML is the human landing page served at the feed root.
// counts (slug -> purpose -> "v4+v6" summary) come from the built index so
// visitors see the security-group quota cost of a purpose up front.
func renderCatalogHTML(reg *Registry, counts map[string]map[string]string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>egress feed — service IP range catalog</title>
<style>
body{font-family:-apple-system,system-ui,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem;line-height:1.5;color:#1a1f26}
table{border-collapse:collapse;width:100%;margin:.75rem 0 1.5rem;font-size:.9rem}
th,td{text-align:left;padding:.35rem .6rem;border:1px solid #d6dade;vertical-align:top}
th{background:#eef1f4}
code{background:#f1f3f5;padding:1px 4px;border-radius:3px;font-size:.85em}
h1{margin-bottom:.25rem} .sub{color:#5b6572;margin-top:0}
a{color:#1d4ed8;text-decoration:none}
@media(prefers-color-scheme:dark){body{background:#111418;color:#e6e9ee}th{background:#1c2128}th,td{border-color:#30363d}code{background:#1c2128}a{color:#6ea8fe}}
</style></head><body>
<h1>egress feed</h1>
<p class="sub">Official, verified IP ranges for third-party services — machine-readable, with provenance.</p>
<p>
<a href="v1/index.json">v1/index.json</a> ·
<a href="v1/changelog.json">changelog</a> ·
<a href="https://github.com/slash0-io/feed">source &amp; methodology</a> ·
<a href="https://github.com/slash0-io/terraform-provider-egress">Terraform provider</a>
</p>
<p>Use with Terraform: <code>data "egress_ranges" "x" { service = "&lt;slug&gt;"  purpose = "&lt;purpose&gt;" }</code>.
<b>egress</b> purposes are ranges you connect to; <b>ingress</b> purposes are webhook/agent sources.</p>
<p><b>Entry counts matter:</b> every CIDR consumes one security-group rule (default quota: 60 per SG,
IPv4 and IPv6 counted separately). Ranges are losslessly aggregated — published coverage is
preserved exactly, never widened. Purposes with hundreds+ of entries belong in prefix lists or
firewall rule groups, not security groups.</p>
`)
	byCat := servicesByCategory(reg)
	for _, cat := range categoryOrder {
		services := byCat[cat.key]
		if len(services) == 0 {
			continue
		}
		fmt.Fprintf(&b, "<h2>%s</h2>\n<table><tr><th>Slug</th><th>Service</th><th>Classification</th><th>Purposes (entries v4+v6)</th><th>Ranges</th></tr>\n", html.EscapeString(cat.title))
		for _, s := range services {
			var lines []string
			seen := map[string]bool{}
			for _, ep := range s.Endpoints {
				for _, p := range ep.Purposes {
					if seen[p.Key] {
						continue
					}
					seen[p.Key] = true
					line := fmt.Sprintf("<code>%s</code> (%s", p.Key, p.Direction)
					if c := counts[s.Slug][p.Key]; c != "" {
						line += ", " + c
					}
					lines = append(lines, line+")")
				}
			}
			sort.Strings(lines)
			fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td><a href=\"v1/services/%s.json\">json</a></td></tr>\n",
				s.Slug, html.EscapeString(s.Name), s.Classification, strings.Join(lines, "<br>"), s.Slug)
		}
		b.WriteString("</table>\n")
	}
	b.WriteString("<h2>Doesn't publish pinnable ranges</h2>\n<table><tr><th>Service</th><th>Vendor position</th></tr>\n")
	for _, np := range reg.NonPublishers {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>\n", html.EscapeString(np.Name), html.EscapeString(strings.TrimSpace(np.VendorPosition)))
	}
	b.WriteString("</table>\n</body></html>\n")
	return b.String()
}
