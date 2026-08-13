package main

import (
	"fmt"
	"sort"
	"strings"
)

// Machine-derived tables for research/SOURCES.md. Only the regions between the
// GEN markers are rewritten, so the hand-written methodology prose survives.
//
// These tables used to be maintained by hand and drifted badly: Stripe's row
// read "advance notice documented" with no period and no link, PagerDuty's read
// "support notices" without recording that the notice covers their REST API IPs
// rather than the webhook ranges we publish, the notice periods Microsoft 365,
// CircleCI and Atlassian document were missing outright, and DocuSign appeared
// twice. Every cell here now derives from sources.yaml, and every claim about a
// named vendor renders as a link to the vendor page that states it.

const (
	sourcesCatalogStart = "<!-- GEN:catalog -->"
	sourcesCatalogEnd   = "<!-- /GEN:catalog -->"
	sourcesNonPubStart  = "<!-- GEN:nonpublishers -->"
	sourcesNonPubEnd    = "<!-- /GEN:nonpublishers -->"
)

// pollLabels matches the wording used on the public vendor report, so the same
// mechanism is never described two different ways.
var pollLabels = map[string]string{
	"cond-get":  "conditional GET",
	"hash":      "full download",
	"docs-page": "page extraction",
}

var documentLabels = map[string]string{
	"json": "JSON",
	"xml":  "XML",
	"csv":  "CSV",
	"text": "text",
	"html": "docs page",
}

// directionOf summarizes which way a service's ranges point, from the
// workload's point of view. A purpose declared "both" counts as each.
func directionOf(svc SourceService) string {
	var egress, ingress bool
	for _, ep := range svc.Endpoints {
		for _, p := range ep.Purposes {
			switch p.Direction {
			case "egress":
				egress = true
			case "ingress":
				ingress = true
			case "both":
				egress, ingress = true, true
			}
		}
	}
	switch {
	case egress && ingress:
		return "egress + ingress"
	case ingress:
		return "ingress"
	case egress:
		return "egress"
	}
	return ""
}

// mdLink renders text as a link when evidence is present. Callers only pass
// evidence that publicationFor has already required, so a missing URL here
// means the claim itself is absent, not that a citation was dropped.
func mdLink(text, url string) string {
	if url == "" {
		return text
	}
	return fmt.Sprintf("[%s](%s)", text, url)
}

func renderSourcesCatalogTable(reg *Registry) (string, error) {
	services := append([]SourceService(nil), reg.Services...)
	sort.Slice(services, func(i, j int) bool {
		if services[i].Name != services[j].Name {
			return services[i].Name < services[j].Name
		}
		return services[i].Slug < services[j].Slug
	})

	var b strings.Builder
	b.WriteString("| Service | Document | Change detection | Advance notice | Vendor change signal | Direction |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, s := range services {
		pub, err := publicationFor(s)
		if err != nil {
			return "", err
		}
		doc, ok := documentLabels[pub.DocumentType]
		if !ok {
			return "", fmt.Errorf("%s: no label for documentType %q", s.Slug, pub.DocumentType)
		}
		poll, ok := pollLabels[pub.PollMode]
		if !ok {
			return "", fmt.Errorf("%s: no label for pollMode %q", s.Slug, pub.PollMode)
		}

		notice := "none documented"
		if pub.Notice != "" {
			notice = mdLink(pub.Notice, pub.NoticeEvidence)
		}
		signal := "none"
		if pub.ChangeSignal != nil {
			signal = mdLink(pub.ChangeSignal.Detail, pub.ChangeSignal.Evidence)
			if pub.ChangeSignal.Kind == "docs-repo" {
				signal += " (docs repo)"
			}
		}
		dir := directionOf(s)
		if dir == "" {
			return "", fmt.Errorf("%s: no purpose declares a direction", s.Slug)
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s | %s | %s | %s | %s |\n",
			s.Name, s.Provenance, doc, poll, notice, signal, dir)
	}
	return b.String(), nil
}

func renderSourcesNonPublisherTable(reg *Registry) (string, error) {
	nps := append([]NonPublisherY(nil), reg.NonPublishers...)
	sort.Slice(nps, func(i, j int) bool { return nps[i].Name < nps[j].Name })

	var b strings.Builder
	b.WriteString("| Service | Vendor's stated position |\n|---|---|\n")
	for _, np := range nps {
		// A recorded negative is still a claim about a named vendor, so it
		// carries the page that states it, same as every other row.
		if np.Evidence == "" {
			return "", fmt.Errorf("non-publisher %s: no evidence URL", np.Slug)
		}
		fmt.Fprintf(&b, "| [%s](%s) | %s |\n",
			np.Name, np.Evidence, strings.TrimSpace(np.VendorPosition))
	}
	return b.String(), nil
}

// replaceMarked swaps the content between start and end, keeping the markers.
func replaceMarked(body, start, end, content string) (string, error) {
	i := strings.Index(body, start)
	if i < 0 {
		return "", fmt.Errorf("marker %s not found", start)
	}
	j := strings.Index(body, end)
	if j < 0 {
		return "", fmt.Errorf("marker %s not found", end)
	}
	if j < i {
		return "", fmt.Errorf("marker %s appears before %s", end, start)
	}
	return body[:i+len(start)] + "\n\n" + content + "\n" + body[j:], nil
}

// rewriteSourcesDoc returns body with both generated tables refreshed.
func rewriteSourcesDoc(body string, reg *Registry) (string, error) {
	catalog, err := renderSourcesCatalogTable(reg)
	if err != nil {
		return "", err
	}
	nonPub, err := renderSourcesNonPublisherTable(reg)
	if err != nil {
		return "", err
	}
	out, err := replaceMarked(body, sourcesCatalogStart, sourcesCatalogEnd, catalog)
	if err != nil {
		return "", err
	}
	out, err = replaceMarked(out, sourcesNonPubStart, sourcesNonPubEnd, nonPub)
	if err != nil {
		return "", err
	}
	// The style rule is enforced here rather than trusted, because vendor
	// position strings flow into this file straight from sources.yaml.
	if strings.Contains(out, "—") {
		return "", fmt.Errorf("research doc contains an em dash; restructure the sentence")
	}
	return out, nil
}
