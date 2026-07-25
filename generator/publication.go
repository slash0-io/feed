package main

import (
	"fmt"

	"github.com/slash0-io/feed/feedschema"
)

// documentTypes maps every parser format in sources.yaml to the shape of the
// upstream body it reads. The mapping is exhaustive on purpose: a format with
// no entry is a build error rather than a silent default, because these values
// become public claims about how a vendor publishes.
var documentTypes = map[string]string{
	"atlassian-ranges":   "json",
	"auth0-regions":      "json",
	"aws-ip-ranges":      "json",
	"azure-service-tags": "json",
	"braintree-ips":      "json",
	"circleci-list":      "json",
	"cloudflare-api":     "json",
	"databricks-ranges":  "json",
	"datadog-ranges":     "json",
	"elastic-ips":        "json",
	"fastly-list":        "json",
	"github-meta":        "json",
	"google-prefixes":    "json",
	"intercom-ranges":    "json",
	"json-cidr-map":      "json",
	"json-ip-array":      "json",
	"o365-endpoints":     "json",
	"oci-ranges":         "json",
	"okta-cells":         "json",
	"salesforce-ranges":  "json",
	"stripe-list":        "json",
	"zendesk-ips":        "json",
	"zscaler-cenr":       "json",
	"geofeed-csv":        "csv",
	"cidr-lines":         "text",
	"ip-lines":           "text",
	"html-cidr-extract":  "html",
}

// Rank orders each axis from easiest to hardest to track. A service that
// publishes through several endpoints is reported at its weakest, since the
// hardest source to follow is what sets the integration cost.
var documentTypeRank = map[string]int{"json": 0, "csv": 1, "text": 2, "html": 3}
var pollModeRank = map[string]int{"cond-get": 0, "hash": 1, "docs-page": 2}

// publicationFor derives the published Publication block for one service.
func publicationFor(svc SourceService) (*feedschema.Publication, error) {
	if len(svc.Endpoints) == 0 {
		return nil, fmt.Errorf("%s: no endpoints", svc.Slug)
	}
	pub := feedschema.Publication{Cadence: svc.Cadence, Notice: svc.Notice}
	for _, e := range svc.Endpoints {
		dt, ok := documentTypes[e.Format]
		if !ok {
			return nil, fmt.Errorf("%s: format %q has no documentType mapping", svc.Slug, e.Format)
		}
		if _, ok := pollModeRank[e.Detection.Poll]; !ok {
			return nil, fmt.Errorf("%s: unknown detection.poll %q", svc.Slug, e.Detection.Poll)
		}
		if pub.DocumentType == "" || documentTypeRank[dt] > documentTypeRank[pub.DocumentType] {
			pub.DocumentType = dt
		}
		if pub.PollMode == "" || pollModeRank[e.Detection.Poll] > pollModeRank[pub.PollMode] {
			pub.PollMode = e.Detection.Poll
		}
		if e.Detection.Push == "" {
			continue
		}
		switch e.Detection.PushKind {
		case "vendor", "docs-repo":
		default:
			return nil, fmt.Errorf("%s: detection.push set without a valid push_kind (got %q)",
				svc.Slug, e.Detection.PushKind)
		}
		// A vendor-operated signal outranks a docs-repo commit feed.
		if pub.ChangeSignal == nil || (pub.ChangeSignal.Kind == "docs-repo" && e.Detection.PushKind == "vendor") {
			pub.ChangeSignal = &feedschema.ChangeSignal{Kind: e.Detection.PushKind, Detail: e.Detection.Push}
		}
	}
	return &pub, nil
}
