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
	"aws-ip-ranges":     parseAWSIPRanges,
	"google-prefixes":   parseGooglePrefixes,
	"cloudflare-api":    parseCloudflareAPI,
	"fastly-list":       parseFastlyList,
	"github-meta":       parseGitHubMeta,
	"datadog-ranges":    parseDatadogRanges,
	"stripe-list":       parseStripeList,
	"atlassian-ranges":  parseAtlassianRanges,
	"okta-cells":        parseOktaCells,
	"salesforce-ranges": parseSalesforceRanges,
	"auth0-regions":     parseAuth0Regions,
	"oci-ranges":        parseOCIRanges,
	"geofeed-csv":       parseGeofeedCSV,
	"cidr-lines":        parseLines,
	"ip-lines":          parseLines,
	"json-ip-array":     parseJSONIPArray,
	"intercom-ranges":   parseIntercomRanges,
	"circleci-list":     parseCircleCIList,
	"braintree-ips":     parseBraintreeIPs,
	"zscaler-cenr":      parseZscalerCENR,
	"databricks-ranges": parseDatabricksRanges,
	"o365-endpoints":    parseO365Endpoints,
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
