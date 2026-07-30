package main

import (
	"fmt"
	"regexp"
	"strings"
)

// html-cidr-extract: pulls IP ranges out of an official vendor docs page.
//
// Select syntax: "section=<substring>[;exclude=<substring>][;from=<substring>]".
//   - section matches heading text case-insensitively; the captured region
//     runs from that heading to the next heading of the same or higher level,
//     so subsections are included.
//   - exclude removes any matching sub-heading's region from the capture
//     (e.g. Anthropic's "Phased out IP addresses").
//   - from narrows the capture to the text following each occurrence of a
//     marker sentence, up to the next heading. This exists because vendors
//     routinely put two directions under one heading: AppDynamics lists the
//     addresses its platform connects FROM under the same region heading as
//     the addresses an agent connects TO, distinguished only by the lead-in
//     "All traffic originating from the <region> Datacenter environment".
//     Selecting on the heading alone would publish both under one direction.
//   - "section=*" captures the whole document.
//
// Candidate tokens are validated with canonCIDR; anything that doesn't parse
// as an IP or CIDR is silently discarded — HTML is noisy by design, and the
// zero-ranges guardrail in the build still catches a select that matches
// nothing real.

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)\s*>`)
	reHeading     = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]\s*>`)
	reTagStrip    = regexp.MustCompile(`(?s)<[^>]+>`)
	reCandidate   = regexp.MustCompile(`[0-9A-Fa-f:.]+(?:/[0-9]{1,3})?`)
)

type headingMark struct {
	level      int
	title      string
	start, end int // byte offsets of the heading element itself
}

func parseHTMLCIDRExtract(body []byte, sel string) ([]string, error) {
	section, exclude, from, err := parseSectionSelect(sel)
	if err != nil {
		return nil, err
	}

	html := reScriptStyle.ReplaceAllString(string(body), " ")
	if section == "*" {
		if from != "" {
			return extractAfterMarkers(html, from)
		}
		return extractRanges(html), nil
	}

	var marks []headingMark
	for _, m := range reHeading.FindAllStringSubmatchIndex(html, -1) {
		level := int(html[m[2]] - '0')
		title := strings.TrimSpace(reTagStrip.ReplaceAllString(html[m[4]:m[5]], " "))
		marks = append(marks, headingMark{level: level, title: title, start: m[0], end: m[1]})
	}

	// regionEnd returns where heading i's hierarchical region stops: the next
	// heading at the same or a higher level.
	regionEnd := func(i int) int {
		for j := i + 1; j < len(marks); j++ {
			if marks[j].level <= marks[i].level {
				return marks[j].start
			}
		}
		return len(html)
	}

	var out []string
	matched := false
	for i, h := range marks {
		if !containsFold(h.title, section) {
			continue
		}
		matched = true
		regStart, regEnd := h.end, regionEnd(i)
		region := html[regStart:regEnd]

		if exclude != "" {
			// Cut excluded sub-regions out of the capture.
			var kept strings.Builder
			cursor := 0
			for j := i + 1; j < len(marks) && marks[j].start < regEnd; j++ {
				if !containsFold(marks[j].title, exclude) {
					continue
				}
				exStart := marks[j].start - regStart
				exEnd := regionEnd(j) - regStart
				if exEnd > len(region) {
					exEnd = len(region)
				}
				if exStart > cursor {
					kept.WriteString(region[cursor:exStart])
				}
				cursor = exEnd
			}
			if cursor < len(region) {
				kept.WriteString(region[cursor:])
			}
			region = kept.String()
		}
		if from != "" {
			sub, err := extractAfterMarkers(region, from)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
			continue
		}
		out = append(out, extractRanges(region)...)
	}
	if !matched {
		return nil, fmt.Errorf("html-cidr-extract: no heading matches section %q", section)
	}
	return out, nil
}

// extractAfterMarkers captures, for every occurrence of marker, the ranges in
// the text between it and the next heading. A marker that matches nothing is an
// error rather than an empty result: silently publishing zero ranges is how a
// vendor's page reorganisation turns into an unnoticed mass removal.
func extractAfterMarkers(fragment, marker string) ([]string, error) {
	lower := strings.ToLower(fragment)
	needle := strings.ToLower(marker)
	var out []string
	found := false
	for pos := 0; ; {
		i := strings.Index(lower[pos:], needle)
		if i < 0 {
			break
		}
		found = true
		start := pos + i + len(needle)
		end := len(fragment)
		if h := reHeading.FindStringIndex(fragment[start:]); h != nil {
			end = start + h[0]
		}
		out = append(out, extractRanges(fragment[start:end])...)
		pos = start
	}
	if !found {
		return nil, fmt.Errorf("html-cidr-extract: no text matches from %q", marker)
	}
	return out, nil
}

func parseSectionSelect(sel string) (section, exclude, from string, err error) {
	for _, part := range strings.Split(sel, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok && part == "*" {
			section = "*"
			continue
		}
		switch strings.TrimSpace(k) {
		case "section":
			section = strings.TrimSpace(v)
		case "exclude":
			exclude = strings.TrimSpace(v)
		case "from":
			from = strings.TrimSpace(v)
		default:
			return "", "", "", fmt.Errorf("html-cidr-extract: unknown select key %q", k)
		}
	}
	if section == "" {
		return "", "", "", fmt.Errorf("html-cidr-extract: select must include section=")
	}
	return section, exclude, from, nil
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// extractRanges strips markup from an HTML fragment and returns every token
// that validates as an IP or CIDR.
func extractRanges(fragment string) []string {
	text := reTagStrip.ReplaceAllString(fragment, " ")
	for entity, repl := range map[string]string{
		"&amp;": "&", "&nbsp;": " ", "&lt;": "<", "&gt;": ">", "&#x2F;": "/", "&#47;": "/",
	} {
		text = strings.ReplaceAll(text, entity, repl)
	}
	var out []string
	for _, tok := range reCandidate.FindAllString(text, -1) {
		tok = strings.Trim(tok, ".:")
		// Require a separator so bare integers and hex words never qualify.
		if !strings.ContainsAny(tok, ".:") {
			continue
		}
		if _, err := canonCIDR(tok); err != nil {
			continue
		}
		out = append(out, tok)
	}
	return out
}
