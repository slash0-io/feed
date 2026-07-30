package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// A parseFunc extracts raw range strings (CIDRs or bare IPs) from an upstream
// body, honoring the purpose's select expression. Selection semantics are
// format-specific; "*" or "" means everything.
type parseFunc func(body []byte, sel string) ([]string, error)

var parsers = map[string]parseFunc{
	"aws-ip-ranges":      parseAWSIPRanges,
	"google-prefixes":    parseGooglePrefixes,
	"cloudflare-api":     parseCloudflareAPI,
	"fastly-list":        parseFastlyList,
	"github-meta":        parseGitHubMeta,
	"hubspot-ranges":     parseHubSpotRanges,
	"datadog-ranges":     parseDatadogRanges,
	"stripe-list":        parseStripeList,
	"atlassian-ranges":   parseAtlassianRanges,
	"okta-cells":         parseOktaCells,
	"salesforce-ranges":  parseSalesforceRanges,
	"auth0-regions":      parseAuth0Regions,
	"oci-ranges":         parseOCIRanges,
	"geofeed-csv":        parseGeofeedCSV,
	"cidr-lines":         parseLines,
	"ip-lines":           parseLines,
	"json-ip-array":      parseJSONIPArray,
	"intercom-ranges":    parseIntercomRanges,
	"circleci-list":      parseCircleCIList,
	"braintree-ips":      parseBraintreeIPs,
	"zscaler-cenr":       parseZscalerCENR,
	"databricks-ranges":  parseDatabricksRanges,
	"o365-endpoints":     parseO365Endpoints,
	"html-cidr-extract":  parseHTMLCIDRExtract,
	"azure-service-tags": parseAzureServiceTags,
	"json-cidr-map":      parseJSONCIDRMap,
	"zendesk-ips":        parseZendeskIPs,
	"elastic-ips":        parseElasticIPs,
	"docusign-ranges":    parseDocuSignRanges,
	"klaviyo-allowlist":  parseKlaviyoAllowlist,
}

// selKV splits "service=S3" into ("service", "S3"). all=true for "*" or "".
func selKV(sel string) (key, val string, all bool) {
	sel = strings.TrimSpace(sel)
	if sel == "" || sel == "*" {
		return "", "", true
	}
	if k, v, ok := strings.Cut(sel, "="); ok {
		return strings.TrimSpace(k), strings.TrimSpace(v), false
	}
	return sel, "", false
}

func parseAWSIPRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		Prefixes []struct {
			IPPrefix string `json:"ip_prefix"`
			Service  string `json:"service"`
		} `json:"prefixes"`
		IPv6Prefixes []struct {
			IPv6Prefix string `json:"ipv6_prefix"`
			Service    string `json:"service"`
		} `json:"ipv6_prefixes"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, p := range d.Prefixes {
		if all || strings.EqualFold(p.Service, want) {
			out = append(out, p.IPPrefix)
		}
	}
	for _, p := range d.IPv6Prefixes {
		if all || strings.EqualFold(p.Service, want) {
			out = append(out, p.IPv6Prefix)
		}
	}
	return out, nil
}

func parseGooglePrefixes(body []byte, _ string) ([]string, error) {
	var d struct {
		Prefixes []map[string]string `json:"prefixes"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, p := range d.Prefixes {
		if v := p["ipv4Prefix"]; v != "" {
			out = append(out, v)
		}
		if v := p["ipv6Prefix"]; v != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

func parseCloudflareAPI(body []byte, _ string) ([]string, error) {
	var d struct {
		Result struct {
			IPv4CIDRs []string `json:"ipv4_cidrs"`
			IPv6CIDRs []string `json:"ipv6_cidrs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return append(d.Result.IPv4CIDRs, d.Result.IPv6CIDRs...), nil
}

func parseFastlyList(body []byte, _ string) ([]string, error) {
	var d struct {
		Addresses     []string `json:"addresses"`
		IPv6Addresses []string `json:"ipv6_addresses"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	return append(d.Addresses, d.IPv6Addresses...), nil
}

// parseGitHubMeta selects one top-level key of api.github.com/meta (hooks,
// web, api, git, pages, actions, dependabot, ...), each an array of CIDRs.
func parseGitHubMeta(body []byte, sel string) ([]string, error) {
	var d map[string]json.RawMessage
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, _, all := selKV(sel)
	if all {
		return nil, fmt.Errorf("github-meta requires an explicit key select (e.g. \"hooks\")")
	}
	raw, ok := d[key]
	if !ok {
		return nil, fmt.Errorf("github-meta: no key %q", key)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("github-meta key %q: %w", key, err)
	}
	return out, nil
}

func parseDatadogRanges(body []byte, sel string) ([]string, error) {
	var d map[string]json.RawMessage
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, _, all := selKV(sel)
	if all {
		return nil, fmt.Errorf("datadog-ranges requires a product select (e.g. \"agents\")")
	}
	raw, ok := d[key]
	if !ok {
		return nil, fmt.Errorf("datadog-ranges: no product %q", key)
	}
	var prod struct {
		PrefixesIPv4 []string `json:"prefixes_ipv4"`
		PrefixesIPv6 []string `json:"prefixes_ipv6"`
	}
	if err := json.Unmarshal(raw, &prod); err != nil {
		return nil, err
	}
	return append(prod.PrefixesIPv4, prod.PrefixesIPv6...), nil
}

// parseStripeList handles {"API": [...]}, {"WEBHOOKS": [...]}, etc. — a
// single-key object of bare IPs.
func parseStripeList(body []byte, _ string) ([]string, error) {
	var d map[string][]string
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, ips := range d {
		out = append(out, ips...)
	}
	return out, nil
}

func parseAtlassianRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		Items []struct {
			CIDR      string   `json:"cidr"`
			Product   []string `json:"product"`
			Direction []string `json:"direction"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, want, all := selKV(sel)
	match := func(vals []string) bool {
		for _, v := range vals {
			if strings.EqualFold(v, want) {
				return true
			}
		}
		return false
	}
	var out []string
	for _, it := range d.Items {
		switch {
		case all:
			out = append(out, it.CIDR)
		case key == "product" && match(it.Product):
			out = append(out, it.CIDR)
		case key == "direction" && match(it.Direction):
			out = append(out, it.CIDR)
		}
	}
	return out, nil
}

func parseOktaCells(body []byte, _ string) ([]string, error) {
	var d map[string]struct {
		IPRanges []string `json:"ip_ranges"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, cell := range d {
		out = append(out, cell.IPRanges...)
	}
	return out, nil
}

func parseAuth0Regions(body []byte, _ string) ([]string, error) {
	var d struct {
		Regions map[string]struct {
			IPv4CIDRs []string `json:"ipv4_cidrs"`
			IPv6CIDRs []string `json:"ipv6_cidrs"`
		} `json:"regions"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, r := range d.Regions {
		out = append(out, r.IPv4CIDRs...)
		out = append(out, r.IPv6CIDRs...)
	}
	return out, nil
}

func parseOCIRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		Regions []struct {
			CIDRs []struct {
				CIDR string   `json:"cidr"`
				Tags []string `json:"tags"`
			} `json:"cidrs"`
		} `json:"regions"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, r := range d.Regions {
		for _, c := range r.CIDRs {
			if all {
				out = append(out, c.CIDR)
				continue
			}
			for _, t := range c.Tags {
				if strings.EqualFold(t, want) {
					out = append(out, c.CIDR)
					break
				}
			}
		}
	}
	return out, nil
}

// parseGeofeedCSV handles RFC 8805 self-published geofeeds (DigitalOcean,
// Linode): CSV with the prefix in the first field, '#' comments.
func parseGeofeedCSV(body []byte, _ string) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		field, _, _ := strings.Cut(line, ",")
		if field = strings.TrimSpace(field); field != "" {
			out = append(out, field)
		}
	}
	return out, sc.Err()
}

// parseLines handles plain text: one CIDR or bare IP per line, '#' comments.
func parseLines(body []byte, _ string) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

func parseJSONIPArray(body []byte, _ string) ([]string, error) {
	var out []string
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseIntercomRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		IPRanges []struct {
			Range   string `json:"range"`
			Service string `json:"service"`
		} `json:"ip_ranges"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, r := range d.IPRanges {
		if all || strings.EqualFold(r.Service, want) {
			out = append(out, r.Range)
		}
	}
	return out, nil
}

// parseCircleCIList selects a group ("jobs", "core", "macOS") from
// {"IPRanges": {<group>: [ips/cidrs]}}.
func parseCircleCIList(body []byte, sel string) ([]string, error) {
	var d struct {
		IPRanges map[string][]string `json:"IPRanges"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, _, all := selKV(sel)
	if all {
		var out []string
		for _, v := range d.IPRanges {
			out = append(out, v...)
		}
		return out, nil
	}
	v, ok := d.IPRanges[key]
	if !ok {
		return nil, fmt.Errorf("circleci-list: no group %q", key)
	}
	return v, nil
}

// parseSalesforceRanges handles ip-ranges.salesforce.com — AWS-style top
// level, but ip_prefix / ipv6_prefix are arrays of strings, not scalars.
func parseSalesforceRanges(body []byte, _ string) ([]string, error) {
	var d struct {
		Prefixes []struct {
			IPPrefix []string `json:"ip_prefix"`
		} `json:"prefixes"`
		IPv6Prefixes []struct {
			IPv6Prefix []string `json:"ipv6_prefix"`
		} `json:"ipv6_prefixes"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	var out []string
	for _, p := range d.Prefixes {
		out = append(out, p.IPPrefix...)
	}
	for _, p := range d.IPv6Prefixes {
		out = append(out, p.IPv6Prefix...)
	}
	return out, nil
}

// parseBraintreeIPs selects an environment ("production" or "sandbox") and
// returns its cidrs + ips. outboundIps (Braintree->you webhook sources) are
// intentionally excluded from the egress purposes declared today.
func parseBraintreeIPs(body []byte, sel string) ([]string, error) {
	var d map[string]struct {
		CIDRs []string `json:"cidrs"`
		IPs   []string `json:"ips"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, _, all := selKV(sel)
	if all {
		return nil, fmt.Errorf("braintree-ips requires an environment select (production|sandbox)")
	}
	env, ok := d[key]
	if !ok {
		return nil, fmt.Errorf("braintree-ips: no environment %q", key)
	}
	return append(append([]string{}, env.CIDRs...), env.IPs...), nil
}

// parseZscalerCENR walks {"<cloud>": {continent: {city: [{range}]}}},
// ignoring the svpnIPs sibling key.
func parseZscalerCENR(body []byte, _ string) ([]string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, err
	}
	var out []string
	for key, raw := range top {
		if key == "svpnIPs" {
			continue
		}
		var continents map[string]map[string][]struct {
			Range string `json:"range"`
		}
		if err := json.Unmarshal(raw, &continents); err != nil {
			return nil, fmt.Errorf("zscaler-cenr cloud %q: %w", key, err)
		}
		for _, cities := range continents {
			for _, entries := range cities {
				for _, e := range entries {
					if e.Range != "" {
						out = append(out, e.Range)
					}
				}
			}
		}
	}
	return out, nil
}

func parseDatabricksRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		Prefixes []struct {
			Type         string   `json:"type"`
			IPv4Prefixes []string `json:"ipv4Prefixes"`
			IPv6Prefixes []string `json:"ipv6Prefixes"`
		} `json:"prefixes"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, p := range d.Prefixes {
		if all || strings.EqualFold(p.Type, want) {
			out = append(out, p.IPv4Prefixes...)
			out = append(out, p.IPv6Prefixes...)
		}
	}
	return out, nil
}

// parseZendeskIPs handles https://<subdomain>.zendesk.com/ips. Select is
// "way=ingress" (ranges you connect to) or "way=egress" (ranges Zendesk
// connects from). Note the inversion: Zendesk names directions from its own
// perspective.
func parseZendeskIPs(body []byte, sel string) ([]string, error) {
	var d struct {
		IPs map[string]struct {
			All      []string `json:"all"`
			Specific []string `json:"specific"`
		} `json:"ips"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	if all {
		return nil, fmt.Errorf("zendesk-ips requires a way select (way=ingress|way=egress)")
	}
	way, ok := d.IPs[want]
	if !ok {
		return nil, fmt.Errorf("zendesk-ips: no way %q", want)
	}
	return append(append([]string{}, way.All...), way.Specific...), nil
}

// parseAzureServiceTags handles the ServiceTags_Public JSON (the fetcher
// resolves the rotating download URL first). Select: "tag=AzureCloud".
func parseAzureServiceTags(body []byte, sel string) ([]string, error) {
	var d struct {
		Values []struct {
			Name       string `json:"name"`
			Properties struct {
				AddressPrefixes []string `json:"addressPrefixes"`
			} `json:"properties"`
		} `json:"values"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	if all {
		return nil, fmt.Errorf("azure-service-tags requires a tag select (e.g. \"tag=AzureCloud\")")
	}
	for _, v := range d.Values {
		if strings.EqualFold(v.Name, want) {
			return v.Properties.AddressPrefixes, nil
		}
	}
	return nil, fmt.Errorf("azure-service-tags: no tag %q", want)
}

// parseJSONCIDRMap handles {"<group>": ["cidr", ...], ...} documents such as
// New Relic's synthetics minion ranges (keyed by location).
func parseJSONCIDRMap(body []byte, sel string) ([]string, error) {
	var d map[string][]string
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, _, all := selKV(sel)
	if !all {
		v, ok := d[key]
		if !ok {
			return nil, fmt.Errorf("json-cidr-map: no key %q", key)
		}
		return v, nil
	}
	var out []string
	for _, v := range d {
		out = append(out, v...)
	}
	return out, nil
}

// parseElasticIPs handles Elastic Cloud's https://ips.cld.elstc.co/ document:
// {"regions": {"<region>": {"egress_from_elastic": [...], "ingress_to_elastic": [...]}}}.
// Select "key=<inner key>" collects that list across every region.
func parseElasticIPs(body []byte, sel string) ([]string, error) {
	var d struct {
		Regions map[string]map[string][]string `json:"regions"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, region := range d.Regions {
		for k, v := range region {
			if all || strings.EqualFold(k, want) {
				out = append(out, v...)
			}
		}
	}
	if !all && len(out) == 0 {
		return nil, fmt.Errorf("elastic-ips: no region has key %q", want)
	}
	return out, nil
}

func parseO365Endpoints(body []byte, sel string) ([]string, error) {
	var d []struct {
		ServiceArea string   `json:"serviceArea"`
		IPs         []string `json:"ips"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	_, want, all := selKV(sel)
	var out []string
	for _, e := range d {
		if all || strings.EqualFold(e.ServiceArea, want) {
			out = append(out, e.IPs...)
		}
	}
	return out, nil
}

// parseDocuSignRanges reads DocuSign's trust-center JSON, which carries a
// syncToken and splits ranges into two top-level arrays: `ranges` (Connect
// webhook sources) and `email_ranges` (sending IPs). Both are arrays of
// objects whose `cidrs` list is qualified by product, region, environment and
// usage.
//
// Select is "usage=<value>", matching the JSON's own vocabulary
// (connect_outbound, email_outbound), and it searches both arrays so a usage
// never has to know which one it lives in. The `domains` key is ignored: it
// lists hostnames for the Akamai-fronted inbound side, which is not pinnable.
//
// Environments (prod, demo, uat) are deliberately NOT filtered. The trust
// page bundles demo ranges into the same sections we already publish, so
// splitting them here would silently drop coverage consumers have today.
func parseDocuSignRanges(body []byte, sel string) ([]string, error) {
	type group struct {
		Product     string   `json:"product"`
		Environment string   `json:"environment"`
		Region      string   `json:"region"`
		Usage       string   `json:"usage"`
		CIDRs       []string `json:"cidrs"`
	}
	var d struct {
		Ranges      []group `json:"ranges"`
		EmailRanges []group `json:"email_ranges"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	key, want, all := selKV(sel)
	if !all && key != "usage" {
		return nil, fmt.Errorf("docusign-ranges: select must be usage=<value>, got %q", sel)
	}
	var out []string
	for _, g := range append(append([]group{}, d.Ranges...), d.EmailRanges...) {
		if all || strings.EqualFold(g.Usage, want) {
			out = append(out, g.CIDRs...)
		}
	}
	if !all && len(out) == 0 {
		return nil, fmt.Errorf("docusign-ranges: no group has usage %q", want)
	}
	return out, nil
}

// parseKlaviyoAllowlist reads Klaviyo's JSON:API ip-allowlist resource, a
// singleton addressed at the fixed id "integration-egress". The endpoint is
// unauthenticated despite living under /client/ (the company_id query
// parameter its docs mention is not required, verified 2026-07-28) and the
// payload is global rather than per-account.
func parseKlaviyoAllowlist(body []byte, _ string) ([]string, error) {
	var d struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Prefixes []string `json:"prefixes"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	if d.Data.ID != "integration-egress" {
		return nil, fmt.Errorf("klaviyo-allowlist: unexpected resource id %q", d.Data.ID)
	}
	if len(d.Data.Attributes.Prefixes) == 0 {
		return nil, fmt.Errorf("klaviyo-allowlist: no prefixes in response")
	}
	return d.Data.Attributes.Prefixes, nil
}

// parseHubSpotRanges reads HubSpot's network-origins endpoint, which returns
// one object per CIDR qualified by service and direction.
//
// Note the direction vocabulary is HubSpot's, not ours: every entry is
// "EGRESS", meaning traffic leaving HubSpot toward the internet. From the
// customer's point of view those are inbound, so the purposes in sources.yaml
// declare direction: ingress. Selecting on it here would invert the meaning,
// so select is on service only.
func parseHubSpotRanges(body []byte, sel string) ([]string, error) {
	var d struct {
		Results []struct {
			CIDR      string `json:"cidr"`
			Direction string `json:"direction"`
			Service   string `json:"service"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	if len(d.Results) == 0 {
		return nil, fmt.Errorf("hubspot-ranges: no results in response")
	}
	key, want, all := selKV(sel)
	if !all && key != "service" {
		return nil, fmt.Errorf("hubspot-ranges: select must be service=<value>, got %q", sel)
	}
	var out []string
	for _, r := range d.Results {
		if all || strings.EqualFold(r.Service, want) {
			out = append(out, r.CIDR)
		}
	}
	if !all && len(out) == 0 {
		return nil, fmt.Errorf("hubspot-ranges: no entry has service %q", want)
	}
	return out, nil
}
