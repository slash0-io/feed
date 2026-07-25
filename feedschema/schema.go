// Package feedschema defines the public feed format (v1). This is the
// canonical copy; terraform-provider-egress vendors it as internal/feedschema.
// Schema v1 is frozen: additive changes only.
package feedschema

const SchemaVersion = 1

// Index is dist/v1/index.json — the catalog listing.
type Index struct {
	SchemaVersion int            `json:"schemaVersion"`
	GeneratedAt   string         `json:"generatedAt"`
	SyncToken     string         `json:"syncToken"`
	Services      []IndexService `json:"services"`
	NonPublishers []NonPublisher `json:"nonPublishers,omitempty"`
}

type IndexService struct {
	Slug           string        `json:"slug"`
	Name           string        `json:"name"`
	Category       string        `json:"category"`
	Classification string        `json:"classification"`
	Purposes       []PurposeMeta `json:"purposes"`
	Publication    *Publication  `json:"publication,omitempty"`
	Path           string        `json:"path"`
	SHA256         string        `json:"sha256"`
}

// Publication describes how a vendor publishes its ranges, as distinct from
// what the ranges are: the shape of the upstream document, whether change can
// be detected cheaply, whether the vendor commits to advance notice, and
// whether any out-of-band change signal exists. A service with several
// endpoints reports its weakest one, since integration difficulty is set by
// the hardest source to track.
type Publication struct {
	// DocumentType is the upstream body's shape: json, csv, text, or html.
	// html means the ranges are only available embedded in a documentation
	// page and must be extracted from it.
	DocumentType string `json:"documentType"`
	// PollMode is how change is detected. cond-get means the server honors
	// If-None-Match / If-Modified-Since, so an unchanged check is nearly
	// free. hash means no cache validators, so the full body is fetched and
	// content-hashed. docs-page means an HTML page is fetched and the ranges
	// extracted before comparison.
	PollMode string `json:"pollMode"`
	// Cadence is the vendor's documented or observed update rhythm.
	Cadence string `json:"cadence,omitempty"`
	// Notice is the advance warning the vendor commits to before newly
	// published ranges carry traffic. Set only where the vendor documents a
	// period; absent means no committed lead time, which is the common case.
	Notice string `json:"notice,omitempty"`
	// NoticeEvidence is the vendor page stating that period. Always present
	// when Notice is, so a consumer can check the claim at its source.
	NoticeEvidence string `json:"noticeEvidence,omitempty"`
	// ChangeSignal is an out-of-band way to learn about a change instead of
	// discovering it by polling.
	ChangeSignal *ChangeSignal `json:"changeSignal,omitempty"`
}

type ChangeSignal struct {
	// Kind is vendor for a signal the vendor operates (a notification topic,
	// a status-page subscription, a version endpoint), or docs-repo for a
	// commit feed on the public repository behind the vendor's documentation
	// page. A docs-repo signal is derived, not offered.
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	// Evidence is the vendor page documenting the signal, or the docs source
	// file for a docs-repo signal.
	Evidence string `json:"evidence"`
}

type PurposeMeta struct {
	Key       string `json:"key"`
	Direction string `json:"direction"`
	// Post-aggregation range counts — the quota cost of using this purpose
	// (every CIDR consumes one security-group rule; IPv4/IPv6 quotas are
	// separate).
	IPv4Count int `json:"ipv4Count"`
	IPv6Count int `json:"ipv6Count"`
}

// NonPublisher records a service that does not publish pinnable ranges, with
// the vendor's stated position — published so consumers can surface it.
type NonPublisher struct {
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Evidence       string `json:"evidence"`
	VendorPosition string `json:"vendorPosition"`
}

// Service is dist/v1/services/<slug>.json.
type Service struct {
	SchemaVersion  int                `json:"schemaVersion"`
	Slug           string             `json:"slug"`
	Name           string             `json:"name"`
	Category       string             `json:"category"`
	Classification string             `json:"classification"`
	GeneratedAt    string             `json:"generatedAt"`
	SyncToken      string             `json:"syncToken"`
	Provenance     string             `json:"provenance,omitempty"`
	Sources        []SourceRecord     `json:"sources"`
	Purposes       map[string]Purpose `json:"purposes"`
}

// SourceRecord is the provenance chain: where the ranges came from and the
// hash of the upstream body they were derived from.
type SourceRecord struct {
	URL         string `json:"url"`
	RetrievedAt string `json:"retrievedAt"`
	SHA256      string `json:"sha256"`
}

type Purpose struct {
	Direction string   `json:"direction"`
	IPv4      []string `json:"ipv4"`
	IPv6      []string `json:"ipv6"`
}

// ChangelogEntry is one element of dist/v1/changelog.json — newest first,
// length-capped by the publisher. Records what each publish actually changed.
type ChangelogEntry struct {
	PublishedAt string          `json:"publishedAt"`
	SyncToken   string          `json:"syncToken"`
	Changes     []ServiceChange `json:"changes"`
}

type ServiceChange struct {
	Slug    string `json:"slug"`
	Purpose string `json:"purpose"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}
