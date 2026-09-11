package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry mirrors feed/sources.yaml. Unknown YAML fields are ignored.
type Registry struct {
	SchemaVersion int             `yaml:"schema_version"`
	Services      []SourceService `yaml:"services"`
	NonPublishers []NonPublisherY `yaml:"non_publishers"`
}

type SourceService struct {
	Slug           string     `yaml:"slug"`
	Name           string     `yaml:"name"`
	Category       string     `yaml:"category"`
	Classification string     `yaml:"classification"`
	Provenance     string     `yaml:"provenance"`
	Cadence        string     `yaml:"cadence"`
	Notice         string     `yaml:"notice"`
	NoticeEvidence string     `yaml:"notice_evidence"`
	Notes          string     `yaml:"notes"`
	Endpoints      []Endpoint `yaml:"endpoints"`
	Verified       string     `yaml:"verified"`
	NeedsAttention string     `yaml:"needs_attention"`
	// Guard relaxes the mass-removal guardrail for this service only, for
	// the case where a vendor has genuinely published a large shrink and a
	// human has verified it. Global thresholds stay put for everyone else.
	Guard *GuardOverride `yaml:"guard"`
}

// GuardOverride is deliberately noisy: the generator warns on every run while
// one is set, because an override left in place silently disarms the only
// protection against a vendor publishing a truncated list.
type GuardOverride struct {
	MaxRemovedFrac *float64 `yaml:"max_removed_frac"`
	Reason         string   `yaml:"reason"`
}

type Endpoint struct {
	ID        string        `yaml:"id"`
	URL       string        `yaml:"url"`
	Format    string        `yaml:"format"`
	Region    string        `yaml:"region"`
	Detection Detection     `yaml:"detection"`
	Purposes  []PurposeDecl `yaml:"purposes"`
	// Headers are sent verbatim with the request. Used for APIs that pin
	// their contract to a version header, where relying on the server's
	// default would let the vendor change the payload shape under us.
	Headers map[string]string `yaml:"headers"`
	// Render names a browser engine to load the page with, for the vendors
	// whose ranges exist only after JavaScript runs. Currently "chrome" is
	// the only value. Leave unset for everything else: a plain fetch is
	// cheaper, has no external dependency, and is what the fixtures replay.
	Render string `yaml:"render"`
}

type Detection struct {
	Poll string `yaml:"poll"`
	Push string `yaml:"push"`
	// PushKind qualifies Push: "vendor" for a signal the vendor operates,
	// "docs-repo" for a commit feed on the repository behind their docs page.
	PushKind string `yaml:"push_kind"`
	// PushEvidence is the vendor page documenting the signal.
	PushEvidence string `yaml:"push_evidence"`
}

type PurposeDecl struct {
	Key       string `yaml:"key"`
	Direction string `yaml:"direction"`
	Select    string `yaml:"select"`
	// DirectionEvidence is the vendor page whose text establishes which way
	// traffic flows for this purpose. Direction is the most-wrong field in the
	// catalog: it was recorded as `both` for auth0 and databricks when the
	// vendors state otherwise, and vendor labels invert often enough that
	// reading one literally is a coin toss. Optional while the validation
	// expedition fills it in; it becomes required once every purpose carries
	// one, exactly like notice_evidence. The sentence itself lives in
	// internal-docs/audits/<slug>.md, which is deliberately in no repo.
	DirectionEvidence string `yaml:"direction_evidence"`
	// Aggregate marks this purpose as the union of the others on the service,
	// which keeps it out of subscription wildcards. See feedschema.Purpose.
	Aggregate bool `yaml:"aggregate"`
}

type NonPublisherY struct {
	Slug           string `yaml:"slug"`
	Name           string `yaml:"name"`
	Evidence       string `yaml:"evidence"`
	VendorPosition string `yaml:"vendor_position"`
}

func LoadRegistry(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Registry
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if r.SchemaVersion != 1 {
		return nil, fmt.Errorf("%s: unsupported schema_version %d", path, r.SchemaVersion)
	}
	// A recorded negative is still a public claim about a named vendor, so it
	// ships with the page that states it. These were reframed 2026-07-30 from
	// unciteable absence claims ("no official ranges page") to positive claims
	// about what the vendor DOES document, because vendors publish what they
	// support rather than what they do not.
	for _, np := range r.NonPublishers {
		if !strings.HasPrefix(np.Evidence, "http") {
			return nil, fmt.Errorf("non-publisher %s: evidence must be a vendor URL, got %q",
				np.Slug, np.Evidence)
		}
		if err := checkCitationURL(np.Evidence); err != nil {
			return nil, fmt.Errorf("non-publisher %s evidence: %w", np.Slug, err)
		}
	}
	for _, svc := range r.Services {
		for _, field := range []struct{ name, value string }{
			{"provenance", svc.Provenance},
			{"notice_evidence", svc.NoticeEvidence},
		} {
			if field.value == "" {
				continue
			}
			if err := checkCitationURL(field.value); err != nil {
				return nil, fmt.Errorf("%s %s: %w", svc.Slug, field.name, err)
			}
		}
		for _, ep := range svc.Endpoints {
			if ev := ep.Detection.PushEvidence; ev != "" {
				if err := checkCitationURL(ev); err != nil {
					return nil, fmt.Errorf("%s/%s push_evidence: %w", svc.Slug, ep.ID, err)
				}
			}
			for _, decl := range ep.Purposes {
				if decl.DirectionEvidence == "" {
					continue
				}
				if err := checkCitationURL(decl.DirectionEvidence); err != nil {
					return nil, fmt.Errorf("%s/%s purpose %s direction_evidence: %w",
						svc.Slug, ep.ID, decl.Key, err)
				}
			}
			switch ep.Render {
			case "", "chrome":
			default:
				return nil, fmt.Errorf("%s/%s: unknown render engine %q (only \"chrome\")",
					svc.Slug, ep.ID, ep.Render)
			}
			// Chrome takes its headers from the page load, not from our
			// process, so silently dropping them would leave a pinned API
			// version or auth header looking applied when it is not.
			if ep.Render != "" && len(ep.Headers) > 0 {
				return nil, fmt.Errorf("%s/%s: headers cannot be sent with render: %s",
					svc.Slug, ep.ID, ep.Render)
			}
		}
		g := svc.Guard
		if g == nil {
			continue
		}
		// A relaxed guardrail without a stated reason is how one gets left in
		// place forever, so it is a build error rather than a warning.
		if strings.TrimSpace(g.Reason) == "" {
			return nil, fmt.Errorf("%s: guard override needs a reason", svc.Slug)
		}
		if g.MaxRemovedFrac == nil {
			return nil, fmt.Errorf("%s: guard override sets no max_removed_frac", svc.Slug)
		}
		if f := *g.MaxRemovedFrac; f < 0 || f > 1 {
			return nil, fmt.Errorf("%s: guard max_removed_frac %v out of range 0..1", svc.Slug, f)
		}
	}
	return &r, nil
}

// checkCitationURL rejects a citation that is not purely a URL. These values
// publish into the feed, CATALOG.md, research/SOURCES.md and every service
// page on the site, where anything trailing the URL breaks the link on all
// four at once: salesforce carried "...?id=000384438 (IP Addresses and Domains
// to Allow)" and rendered dead everywhere it appeared.
func checkCitationURL(raw string) error {
	if strings.ContainsAny(raw, " \t\n") {
		return fmt.Errorf("must be a bare URL with no label or whitespace, got %q", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("unparseable URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("not an http(s) URL: %q", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("URL has no host: %q", raw)
	}
	return nil
}
