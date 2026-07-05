package main

import (
	"fmt"
	"os"

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
	Notes          string     `yaml:"notes"`
	Endpoints      []Endpoint `yaml:"endpoints"`
	Verified       string     `yaml:"verified"`
	NeedsAttention string     `yaml:"needs_attention"`
}

type Endpoint struct {
	ID        string        `yaml:"id"`
	URL       string        `yaml:"url"`
	Format    string        `yaml:"format"`
	Region    string        `yaml:"region"`
	Detection Detection     `yaml:"detection"`
	Purposes  []PurposeDecl `yaml:"purposes"`
}

type Detection struct {
	Poll string `yaml:"poll"`
	Push string `yaml:"push"`
}

type PurposeDecl struct {
	Key       string `yaml:"key"`
	Direction string `yaml:"direction"`
	Select    string `yaml:"select"`
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
	return &r, nil
}
