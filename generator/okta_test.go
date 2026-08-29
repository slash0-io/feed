package main

import (
	"strings"
	"testing"
)

const oktaBody = `{
  "us_cell_1":  {"ip_ranges": ["192.0.2.0/25", "192.0.2.128/25"]},
  "emea_cell_1":{"ip_ranges": ["198.51.100.0/24"]},
  "us_cell_99": {"ip_ranges": ["203.0.113.0/24"]}
}`

// Okta serves each org from one cell, so a purpose has to be able to name the
// cells it covers rather than taking the whole document.
func TestOktaCellSelection(t *testing.T) {
	got, err := parseOktaCells([]byte(oktaBody), "cells=emea_cell_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "198.51.100.0/24" {
		t.Fatalf("selected %v, want just the emea_cell_1 range", got)
	}
}

// The catch-all purpose still takes everything, so splitting loses nothing.
func TestOktaWildcardTakesEveryCell(t *testing.T) {
	got, err := parseOktaCells([]byte(oktaBody), "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("wildcard got %d ranges, want all 4", len(got))
	}
}

// A cell Okta stops publishing must fail rather than silently shrinking the
// purpose to nothing.
func TestOktaUnknownCellFails(t *testing.T) {
	_, err := parseOktaCells([]byte(oktaBody), "cells=us_cell_404")
	if err == nil {
		t.Fatal("a cell absent from the document must fail the build")
	}
	if !strings.Contains(err.Error(), "us_cell_404") {
		t.Fatalf("error should name the missing cell, got %v", err)
	}
}

// The reason the check exists: Okta's allowlisting page runs behind their JSON.
// A cell no purpose claims would otherwise sit only in the catch-all, where
// nobody would notice it, and customers in it would have no purpose to use.
func TestOktaUnclaimedCellFailsTheBuild(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "production-us", Select: "cells=us_cell_1"},
		{Key: "production-germany", Select: "cells=emea_cell_1"},
	}
	err := checkOktaCellCoverage([]byte(oktaBody), decls)
	if err == nil {
		t.Fatal("us_cell_99 is claimed by no purpose; that must fail")
	}
	if !strings.Contains(err.Error(), "us_cell_99") {
		t.Fatalf("error should name the unclaimed cell, got %v", err)
	}
	// The catch-all must not count as coverage, or the check is pointless.
	if strings.Contains(err.Error(), "all") && !strings.Contains(err.Error(), "catch-all") {
		t.Fatalf("unexpected mention of the catch-all purpose: %v", err)
	}
}

func TestOktaFullCoveragePasses(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "production-us", Select: "cells=us_cell_1,us_cell_99"},
		{Key: "production-germany", Select: "cells=emea_cell_1"},
	}
	if err := checkOktaCellCoverage([]byte(oktaBody), decls); err != nil {
		t.Fatalf("every cell is claimed, so this must pass: %v", err)
	}
}

// A service that has not been partitioned at all keeps working: the check only
// applies once some purpose starts naming cells.
func TestOktaCoverageSkippedWhenNotPartitioned(t *testing.T) {
	if err := checkOktaCellCoverage([]byte(oktaBody), []PurposeDecl{{Key: "all", Select: "*"}}); err != nil {
		t.Fatalf("an unpartitioned endpoint must not trip the check: %v", err)
	}
}

const dbxBody = `{"prefixes":[
 {"platform":"aws","type":"inbound","ipv4Prefixes":["192.0.2.0/24"],"ipv6Prefixes":[]},
 {"platform":"aws","type":"outbound","ipv4Prefixes":["198.51.100.0/24"],"ipv6Prefixes":[]},
 {"platform":"azure","type":"outbound","ipv4Prefixes":["203.0.113.0/24"],"ipv6Prefixes":[]}
]}`

// Databricks tags every prefix with a platform, and a customer uses one, so the
// union is far broader than anyone needs.
func TestDatabricksSelectsPlatformAndType(t *testing.T) {
	got, err := parseDatabricksRanges([]byte(dbxBody), "platform=aws;type=inbound")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "192.0.2.0/24" {
		t.Fatalf("got %v, want only the aws inbound prefix", got)
	}
}

// A platform or type Databricks stops publishing must fail rather than quietly
// producing a smaller purpose.
func TestDatabricksEmptySelectionFails(t *testing.T) {
	if _, err := parseDatabricksRanges([]byte(dbxBody), "platform=gcp;type=inbound"); err == nil {
		t.Fatal("a select matching no prefixes must fail the build")
	}
}

func TestDatabricksRejectsUnknownSelectKey(t *testing.T) {
	if _, err := parseDatabricksRanges([]byte(dbxBody), "region=us-east-1"); err == nil {
		t.Fatal("an unknown select key must fail rather than matching everything")
	}
}

// The catch-all still takes every platform and type.
func TestDatabricksWildcardTakesEverything(t *testing.T) {
	got, err := parseDatabricksRanges([]byte(dbxBody), "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("wildcard got %d, want 3", len(got))
	}
}

const auth0Body = `{"regions":{
 "US":{"ipv4_cidrs":["192.0.2.0/24"]},
 "EU":{"ipv4_cidrs":["198.51.100.0/24"]},
 "JP":{"ipv4_cidrs":["203.0.113.0/24"]}
}}`

// An Auth0 tenant lives in one region, so a purpose has to be able to name it.
func TestAuth0RegionSelection(t *testing.T) {
	got, err := parseAuth0Regions([]byte(auth0Body), "regions=EU")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "198.51.100.0/24" {
		t.Fatalf("got %v, want only the EU range", got)
	}
}

func TestAuth0UnknownRegionFails(t *testing.T) {
	if _, err := parseAuth0Regions([]byte(auth0Body), "regions=ZZ"); err == nil {
		t.Fatal("a region absent from the document must fail the build")
	}
}

// A region Auth0 adds must not land only in the aggregate, where a tenant
// there would have no purpose to select.
func TestAuth0UnclaimedRegionFailsTheBuild(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "united-states", Select: "regions=US"},
		{Key: "europe", Select: "regions=EU"},
	}
	err := checkAuth0RegionCoverage([]byte(auth0Body), decls)
	if err == nil {
		t.Fatal("JP is claimed by no purpose; that must fail")
	}
	if !strings.Contains(err.Error(), "JP") {
		t.Fatalf("error should name the unclaimed region, got %v", err)
	}
}

func TestAuth0FullCoveragePasses(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "united-states", Select: "regions=US"},
		{Key: "europe", Select: "regions=EU"},
		{Key: "japan", Select: "regions=JP"},
	}
	if err := checkAuth0RegionCoverage([]byte(auth0Body), decls); err != nil {
		t.Fatalf("every region is claimed: %v", err)
	}
}

const gcBody = `{"prefixes":[
 {"ipv4Prefix":"192.0.2.0/24","service":"Google Cloud","scope":"us-central1"},
 {"ipv6Prefix":"2001:db8::/32","service":"Google Cloud","scope":"us-central1"},
 {"ipv4Prefix":"198.51.100.0/24","service":"Google Cloud","scope":"europe-west1"}
]}`

// A workload talks to resources in one or two regions, not all 48.
func TestGoogleScopeSelection(t *testing.T) {
	got, err := parseGooglePrefixes([]byte(gcBody), "scope=europe-west1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "198.51.100.0/24" {
		t.Fatalf("got %v, want only the europe-west1 range", got)
	}
}

// The bot files (openai, anthropic) share this parser and carry no scope, so
// the wildcard must keep taking everything.
func TestGoogleWildcardUnaffectedByScopes(t *testing.T) {
	got, err := parseGooglePrefixes([]byte(gcBody), "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("wildcard got %d, want all 3", len(got))
	}
}

// A region Google adds or renames must reach a human before it reaches
// consumers, because a purpose key becomes an immutable prefix list name.
func TestGoogleUnclaimedScopeFailsTheBuild(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "us-central1", Select: "scope=us-central1"},
	}
	err := checkGoogleScopeCoverage([]byte(gcBody), decls)
	if err == nil {
		t.Fatal("europe-west1 is claimed by no purpose; that must fail")
	}
	if !strings.Contains(err.Error(), "europe-west1") {
		t.Fatalf("error should name the unclaimed scope, got %v", err)
	}
}

// A scope value that cannot be a purpose key must fail rather than becoming a
// prefix list name that can never be renamed.
func TestGoogleRejectsUnusableScopeKey(t *testing.T) {
	body := `{"prefixes":[{"ipv4Prefix":"192.0.2.0/24","scope":"US Central (new!)"}]}`
	err := checkGoogleScopeCoverage([]byte(body), []PurposeDecl{
		{Key: "all", Select: "*"},
		{Key: "x", Select: "scope=whatever"},
	})
	if err == nil || !strings.Contains(err.Error(), "unusable as a purpose key") {
		t.Fatalf("a scope with spaces and punctuation must be refused, got %v", err)
	}
}

const duoBody = `<html>
<p>allowed to reach all of the following Duo MFA service IP blocks for the data residency area for your deployment</p>
<table><thead><tr><th>Data Residency (Jurisdiction)</th><th>IP Ranges</th><th>Duo Deployments</th></tr></thead>
<tbody>
<tr><td>U.S.</td><td>192.0.2.0/25 192.0.2.128/25</td><td>DUO1, DUO2</td></tr>
<tr><td>EU</td><td>198.51.100.0/26</td><td>DUO3</td></tr>
<tr><td>Central Europe (Germany / Switzerland)</td><td>203.0.113.0/26</td><td>DUO38</td></tr>
</tbody></table>
<p>Trusted Endpoints If your organization is applying Trusted Endpoints policies</p>
<table><thead><tr><th>Data Residency (Jurisdiction)</th><th>IP Range(s)</th></tr></thead>
<tbody><tr><td>U.S.</td><td>203.0.113.128/29</td></tr></tbody></table>
</html>`

// The page carries two tables under the same heading, one per product. Picking
// the first would serve MFA ranges as Trusted Endpoints, or the reverse.
func TestDuoSelectsTheNamedTable(t *testing.T) {
	mfa, err := parseDuoResidency([]byte(duoBody), "table=Duo Deployments")
	if err != nil {
		t.Fatal(err)
	}
	te, err := parseDuoResidency([]byte(duoBody), "table=Trusted Endpoints")
	if err != nil {
		t.Fatal(err)
	}
	if len(mfa) != 4 {
		t.Fatalf("mfa table got %v, want the 4 MFA ranges", mfa)
	}
	if len(te) != 1 || te[0] != "203.0.113.128/29" {
		t.Fatalf("trusted endpoints table got %v", te)
	}
}

// Regression: a substring match for "EU" also hits "Central Europe", which
// silently doubled mfa-eu. Areas are matched by prefix.
func TestDuoAreaMatchIsPrefixNotSubstring(t *testing.T) {
	eu, err := parseDuoResidency([]byte(duoBody), "table=Duo Deployments;area=EU")
	if err != nil {
		t.Fatal(err)
	}
	if len(eu) != 1 || eu[0] != "198.51.100.0/26" {
		t.Fatalf("area=EU got %v, want only the EU row (not Central Europe)", eu)
	}
}

// A residency area Duo adds must reach a human, not sit only in the union.
func TestDuoUnclaimedAreaFailsTheBuild(t *testing.T) {
	decls := []PurposeDecl{
		{Key: "mfa", Select: "table=Duo Deployments"},
		{Key: "mfa-us", Select: "table=Duo Deployments;area=U.S."},
		{Key: "mfa-eu", Select: "table=Duo Deployments;area=EU"},
	}
	err := checkDuoAreaCoverage([]byte(duoBody), decls)
	if err == nil || !strings.Contains(err.Error(), "Central Europe") {
		t.Fatalf("the unclaimed Central Europe row must fail the build, got %v", err)
	}
}

// A table marker that stops matching must fail rather than falling through to
// the other product's table.
func TestDuoUnknownTableFails(t *testing.T) {
	if _, err := parseDuoResidency([]byte(duoBody), "table=Nonexistent Product"); err == nil {
		t.Fatal("an unmatched table marker must fail the build")
	}
}
