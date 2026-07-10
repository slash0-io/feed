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
	Path           string        `json:"path"`
	SHA256         string        `json:"sha256"`
}

type PurposeMeta struct {
	Key       string `json:"key"`
	Direction string `json:"direction"`
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
