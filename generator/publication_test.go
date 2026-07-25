package main

import "testing"

func ep(format, poll, push, kind string) Endpoint {
	ev := ""
	if push != "" {
		ev = "https://vendor.example/docs"
	}
	return Endpoint{Format: format, Detection: Detection{
		Poll: poll, Push: push, PushKind: kind, PushEvidence: ev}}
}

func TestPublicationReportsWeakestEndpoint(t *testing.T) {
	pub, err := publicationFor(SourceService{
		Slug: "new-relic",
		Endpoints: []Endpoint{
			ep("html-cidr-extract", "docs-page", "", ""),
			ep("json-cidr-map", "cond-get", "", ""),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pub.DocumentType != "html" {
		t.Errorf("documentType = %q, want html (the weaker of html/json)", pub.DocumentType)
	}
	if pub.PollMode != "docs-page" {
		t.Errorf("pollMode = %q, want docs-page (the weaker of docs-page/cond-get)", pub.PollMode)
	}
}

func TestPublicationPrefersVendorChangeSignal(t *testing.T) {
	pub, err := publicationFor(SourceService{
		Slug: "mixed-signals",
		Endpoints: []Endpoint{
			ep("html-cidr-extract", "docs-page", "commits feed", "docs-repo"),
			ep("json-cidr-map", "cond-get", "status page subscription", "vendor"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pub.ChangeSignal == nil || pub.ChangeSignal.Kind != "vendor" {
		t.Fatalf("changeSignal = %+v, want the vendor-operated one", pub.ChangeSignal)
	}
}

func TestPublicationNoSignalWhenNonePublished(t *testing.T) {
	pub, err := publicationFor(SourceService{
		Slug:      "quiet",
		Endpoints: []Endpoint{ep("geofeed-csv", "hash", "", "")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pub.ChangeSignal != nil {
		t.Errorf("changeSignal = %+v, want nil", pub.ChangeSignal)
	}
	if pub.DocumentType != "csv" {
		t.Errorf("documentType = %q, want csv", pub.DocumentType)
	}
}

func TestPublicationRejectsUnmappedFormat(t *testing.T) {
	if _, err := publicationFor(SourceService{
		Slug:      "future",
		Endpoints: []Endpoint{ep("some-new-parser", "cond-get", "", "")},
	}); err == nil {
		t.Fatal("want an error for an unmapped format, got nil")
	}
}

func TestPublicationRejectsNoticeWithoutEvidence(t *testing.T) {
	if _, err := publicationFor(SourceService{
		Slug:      "uncited",
		Notice:    "30 days",
		Endpoints: []Endpoint{ep("json-cidr-map", "cond-get", "", "")},
	}); err == nil {
		t.Fatal("want an error for notice without notice_evidence, got nil")
	}
}

func TestPublicationRejectsPushWithoutEvidence(t *testing.T) {
	if _, err := publicationFor(SourceService{
		Slug: "uncited-signal",
		Endpoints: []Endpoint{{Format: "json-cidr-map", Detection: Detection{
			Poll: "cond-get", Push: "some signal", PushKind: "vendor"}}},
	}); err == nil {
		t.Fatal("want an error for push without push_evidence, got nil")
	}
}

func TestPublicationRejectsPushWithoutKind(t *testing.T) {
	if _, err := publicationFor(SourceService{
		Slug:      "sloppy",
		Endpoints: []Endpoint{ep("json-cidr-map", "cond-get", "some signal", "")},
	}); err == nil {
		t.Fatal("want an error for push without push_kind, got nil")
	}
}

// Every format used by the live registry must have a documentType mapping, and
// every push must be qualified. This is what stops a registry edit from
// shipping an unclassified service to the public feed.
func TestEveryRegistryServiceHasAPublication(t *testing.T) {
	reg, err := LoadRegistry("../sources.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, svc := range reg.Services {
		pub, err := publicationFor(svc)
		if err != nil {
			t.Errorf("%s: %v", svc.Slug, err)
			continue
		}
		if pub.DocumentType == "" || pub.PollMode == "" {
			t.Errorf("%s: incomplete publication %+v", svc.Slug, pub)
		}
		if pub.Notice != "" && pub.NoticeEvidence == "" {
			t.Errorf("%s: notice with no evidence URL", svc.Slug)
		}
		if pub.ChangeSignal != nil && pub.ChangeSignal.Evidence == "" {
			t.Errorf("%s: change signal with no evidence URL", svc.Slug)
		}
	}
}
