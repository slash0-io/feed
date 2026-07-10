# Upstream IP-Range Sources — Research & Verification

*Research date: 2026-07-04. Companion machine registry: [`sources.yaml`](../sources.yaml).*

This document records, for every service in the catalog: **how the ranges are obtained** (the official source and its provenance), **how often they change** (documented cadence where the vendor states one, observed behavior otherwise), and **how we detect changes fast enough to republish immediately**.

## Methodology

1. **Official sources only.** Every entry cites the vendor's own publication (docs page, JSON/CSV endpoint, or API). Community aggregators (e.g. `rezmoss/cloud-provider-ip-addresses`, `tobilg/public-cloud-provider-ip-ranges`) were used only to discover leads, never as a data source — our feed's provenance chain must terminate at the vendor.
2. **Live verification.** Every machine-readable endpoint in `sources.yaml` was fetched on the research date; HTTP status, payload shape, and caching headers were recorded. Response bodies are archived as parser fixtures.
3. **Conditional-GET probing.** For every endpoint that returned an `ETag` or `Last-Modified` header, we replayed the request with `If-None-Match`/`If-Modified-Since` and confirmed the server honors it. **All 15 probed endpoints returned `304 Not Modified`** (12 on 2026-07-04; Buildkite, Tenable, and Elastic Cloud on 2026-07-10) — cheap sub-minute polling is viable across the board.

## Source quality tiers

| Tier | Definition | Examples |
|---|---|---|
| **A** | Machine-readable endpoint, versioned (sync token / timestamp / changelog in-band) | AWS, GCP, Salesforce, Atlassian, Auth0, Datadog, Oracle |
| **B** | Machine-readable endpoint, unversioned (detect by header or content hash) | Stripe, Fastly, CircleCI, Grafana Cloud, Braintree, PagerDuty, Zoom |
| **C** | Official docs/HTML page only (extract CIDRs, hash the extracted set) | Anthropic, GitLab.com, Sentry, New Relic, Postmark, DocuSign |
| **D** | Published but gated (auth or per-customer) — partner/API integrations later | HubSpot (API), MongoDB Atlas control plane (Admin API), Mailgun, Akamai |
| **✗** | No pinnable publication — cataloged as `pinnable: false` with the vendor's stated alternative | Twilio, Adyen, SendGrid, Slack, Shopify, Duo, Snowflake, Vercel, Netlify |

## Verified catalog at a glance

Direction: **E** = ranges you connect *to* (egress allowlists) · **I** = ranges they connect *from* (webhook/agent sources, ingress allowlists).

| Service | Source format | Versioning | Cond-GET 304 | Push/side channel | Documented cadence | Dir |
|---|---|---|---|---|---|---|
| AWS | JSON (`ip-ranges.json`) | `syncToken`, `createDate` | ✓ | **SNS `AmazonIpSpaceChanged`** | every change signaled | E/I |
| Google Cloud / Google | JSON (`cloud.json` / `goog.json`) | `syncToken`, `creationTime` | ✓ | — | ~daily regeneration | E |
| Azure Service Tags | JSON (weekly file) | per-tag `changeNumber` | n/a (rotating URL) | Discovery API | weekly + ≥1-week grace before use | E |
| Microsoft 365 | JSON web service | version string | n/a | **`/version` endpoint (purpose-built)** | monthly + out-of-band | E |
| Oracle OCI | JSON | `last_updated_timestamp` | ✓ | — | irregular | E |
| DigitalOcean | RFC 8805 geofeed CSV | none | ✗ (hash) | — | irregular | E |
| Linode/Akamai cloud | RFC 8805 geofeed | `Last modified` comment | ✓ | — | irregular | E |
| Cloudflare | text + API (`/client/v4/ips`) | API `etag` field | ✓ | staged before production | rare | E/I |
| Fastly | JSON (`public-ip-list`) | none | ✗ (hash) | **status-page announcements** | rare, announced in advance | E/I |
| GitHub | JSON (`/meta`) | none | ✓ | changelog blog | occasional | E/I |
| GitLab.com | docs page | none | n/a | **docs-repo commit feed** | stable; runners have no static IPs | E/I |
| CircleCI | JSON | none | ✗ (hash) | — | irregular (~30 IPs) | I |
| Stripe | JSON ×3 (api/webhooks/terminal) | none | ✗ (hash) | advance notice documented | irregular | E/I |
| Braintree | JSON (`ips.json`) | none | ✗ (hash) | "watch ips.json" (documented) | irregular | E |
| Datadog | JSON per site (US/EU/…) | `version`, `modified` | ✓ | — | irregular | E/I |
| Grafana Cloud | text per product | none | ✗ (hash) | — | irregular | I |
| Sentry | docs page | none | n/a | **docs-repo commit feed** (`getsentry/sentry-docs`) | rare | E/I |
| New Relic | docs page | none | n/a | **docs-repo commit feed** (`newrelic/docs-website`) + what's-new posts | occasional, pre-announced | E/I |
| Okta | JSON (S3) | none | ✓ | docs say watch `Last-Modified` | periodic (new cells) | E/I |
| Auth0 | JSON | `last_updated_at` + in-file changelog | ✓ | **email notice months ahead** (documented) | rare | E/I |
| Atlassian | JSON | `syncToken`, `creationDate` | ✓ | — | irregular; items tagged product/direction/region | E/I |
| Salesforce | JSON (AWS-schema clone) | `syncToken`, `createDate` | ✓ | — | irregular | E/I |
| Zoom | text CIDRs | none | ✓ | — | irregular | E |
| Intercom | JSON per region | `date` field | ✓ | — | **daily 09:00 Dublin (documented)** | E/I |
| PagerDuty | JSON array (US/EU) | none | ✓ | support notices | rare, fixed list | I |
| Postmark | docs page | none | n/a | — | rare, fixed list | E/I |
| DocuSign | Trust Center page | none | n/a | release notes | with releases | I |
| Zscaler | JSON per cloud | none | ✓ | — | irregular | E |
| Databricks | JSON | `timestampSeconds` | ✗ (hash) | — | **≤ every 30 days; 60-day activation grace (documented)** | E/I |
| Anthropic | docs page | none | n/a | "will not change without notice" (documented) | rare; dedicated address space | E/I |
| OpenAI (ChatGPT egress) | JSON ×2 | `creationTime` | ✗ (hash) | — | irregular; "fetch regularly" | I |
| Zendesk | JSON (`/ips`, per-subdomain, unauthenticated) | none | ✗ (hash) | daily fetch is vendor-documented best practice | irregular | E/I |
| PayPal | help-center page | none | n/a | — | irregular; "if you must allowlist" (their wording) | E/I |
| DocuSign | Trust Center page (`/trust/security/esignature`) | none | n/a | release notes | with releases | I |
| Vultr (Constant) | RFC 8805 geofeed | `Last Updated` comment | ✗ (hash) | — | irregular | E |
| Buildkite | JSON (`/v2/meta`, unauthenticated) | none | ✓ | — | irregular; **new IPs advertised ≥7 days before use (documented)** | I |
| Tenable | JSON (AWS-schema clone) | `syncToken`, `createDate` | ✓ | — | irregular | I |
| Rapid7 (InsightAppSec) | docs page | none | n/a | — | irregular; per-region engine IPs | I |
| Elastic Cloud | JSON (`ips.cld.elstc.co`) | none | ✓ | **status-page announcements** | **changes announced ≥8 weeks ahead (documented)** | E/I |
| Neon | docs page | none | n/a | **docs-repo commit feed** (`neondatabase/website`) | with region buildout | I |
| Klaviyo | help-center page | none | n/a | — | rare; two dedicated blocks | I |
| Twilio SIP Trunking | docs page | none | n/a | — | rare; regional /30s + media /18 | E/I |

## How we know about updates fast (detection architecture)

Four layers, cheapest-first. Every source gets layer 2 or 3 as a floor; layers 1 and 4 are added where the vendor offers them.

1. **Push subscriptions** — zero-latency where offered:
   - AWS: SQS queue subscribed to SNS `arn:aws:sns:us-east-1:806199016981:AmazonIpSpaceChanged` (payload carries the new `syncToken` + md5).
   - Fastly: status-page subscription ("IP address announcement" category).
   - Microsoft 365: `endpoints.office.com/version` — a tiny purpose-built version endpoint; poll every minute, fetch the full set only on version bump.
   - Vendor notice emails (Auth0's months-ahead policy, New Relic what's-new): routed into the review queue, not automation.
2. **Conditional-GET polling (60 s)** — for the 15+ endpoints with verified `ETag`/`Last-Modified` support. A `304` costs ~200 bytes; polling the whole tier every minute is < 1 GB/month of traffic. Practical detection latency: ≤ 60 s from publication.
3. **Content-hash polling (5 min)** — endpoints with no cache headers (Stripe, Fastly, CircleCI, Grafana, DigitalOcean, Databricks, OpenAI, O365 full payload). Fetch, canonicalize (parse → sort → dedupe), hash the *normalized range set*, compare. Hashing normalized output rather than raw bytes avoids false positives from key reordering or whitespace.
4. **Docs-page watching (15 min + commit feeds)** — for tier C: fetch page, extract CIDRs with a per-source parser, hash the extracted set (never the raw HTML — nav/footer churn would false-positive). Where the docs are in a public repo (New Relic, Sentry, GitLab), additionally watch the specific file's commit Atom feed — near-instant detection *plus* a human-readable diff and commit message explaining the change.

**Publication pipeline on detection:** fetch → parse → normalize → **validate** → version-bump → publish → notify.

Validation gates (the "never break a customer" layer):
- Reject impossible entries: `0.0.0.0/0`, RFC 1918/RFC 4193 space, malformed CIDRs, overlaps collapsing to empty.
- **Delta guardrail:** a change replacing more than N % of a service's ranges (or dropping below a per-service floor count) is quarantined for human review instead of auto-published — a vendor feed returning a truncated or error body must never propagate.
- Grace windows: on removal, keep the old range flagged `deprecated` for a configurable window before deletion (mirrors Azure's ≥1-week and Databricks' 60-day patterns).
- Every publish is versioned with a changelog entry recording the upstream evidence (source URL, upstream token/hash, retrieval timestamp) — the provenance chain customers and auditors can follow.

The pipeline's change history *is* the "State of SaaS Egress" dataset from the strategy doc: per-vendor churn rates, observed cadences, and stability ratings fall out of it for free.

## Services that do NOT publish pinnable ranges (verified positions)

The honest-catalog list — published in the feed as `pinnable: false` with the vendor's recommended alternative. This is sales collateral for Tier 2 as much as it is documentation.

| Service | Vendor's stated position | Their recommended alternative |
|---|---|---|
| Twilio (REST/webhooks) | IPs "highly dynamic, span a large range" | allow `*.twilio.com`; SIP trunking IS pinnable — see the `twilio-sip` service |
| Adyen | no IP list; ranges change with ISPs | allowlist/resolve `out.adyen.com` hourly (their wording) |
| SendGrid (webhooks) | dynamic cloud infra | signed webhooks, not IP allowlists |
| Slack | no egress IPs published | their allowlist feature restricts *your* IPs calling *them* |
| Shopify (webhooks) | not published | verify HMAC signatures |
| Duo Security | discourages IP rules outright | domain-based rules (KB 1337 if forced) |
| Snowflake | per-account deployment | `SYSTEM$ALLOWLIST()` per account |
| MongoDB Atlas (data plane) | per-project, dynamic | PrivateLink / private endpoints |
| Vercel (egress) | dynamic by default | paid per-customer Static IPs feature |
| Netlify (egress) | none; Lambda-backed | none (third-party proxies) |
| HubSpot | "change too frequently" for static lists | their live IP-ranges API (tier-D integration candidate) |
| OpenAI **API** (`api.openai.com`) | Cloudflare-fronted | domain rules; note contrast with Anthropic's dedicated ranges |
| Akamai | customer-specific maps, auth-gated | Site Shield per customer |
| Box | "IP addresses can change frequently and without notice" | domain allowlisting |
| Sumo Logic | no own ranges published | AWS `ip-ranges.json` (Amazon/EC2, your deployment region) — covered by our `aws` service |

## Notable findings

- **Salesforce publishes an AWS-style `ip-ranges.json`** at `ip-ranges.salesforce.com` (same `syncToken`/`createDate`/`prefixes` top level, but `ip_prefix` values are *arrays* of CIDRs rather than scalars) — undocumented in most search results, verified live. The AWS schema is becoming a de-facto standard (a point in favor of modeling our public feed on it), though even its imitators drift, which is why every source gets its own fixture-tested parser. **Tenable** (`docs.tenable.com/ip-ranges/data.json`) is a second, schema-exact clone — our AWS parser consumes it unmodified.
- **Direction matters and nobody models it.** Roughly a third of published ranges describe the vendor's *egress toward customers* (webhook/agent/synthetic sources) — useful for *ingress* rules, useless for egress allowlists. Every purpose in our schema carries a `direction` field; no upstream source does this consistently except Atlassian.
- **Anthropic is the only major AI API with pinnable inbound ranges** (dedicated space, "will not change without notice"). OpenAI publishes only its own egress; its API is CDN-fronted.
- **Grace windows are the emerging best practice** (Azure ≥1 week, Databricks 60 days) — our feed's `deprecated` flag generalizes this pattern to vendors that don't offer one.
- **Open-source docs repos are a change-detection goldmine**: for New Relic, Sentry, and GitLab, the authoritative IP page is a file in a public git repo — commit feeds give near-instant, semantically-diffed notice of changes to tier-C sources that would otherwise need scraping.

## Backlog

~27 candidates remain (IBM Cloud, Alibaba, Workday, ServiceNow, CrowdStrike, SentinelOne, Netskope, Plaid, Square, JFrog, Confluent, Aiven, …) — full list in `sources.yaml` under `backlog`. Researched 2026-07-10 and resolved: Zendesk, PayPal, Vultr, DocuSign integrated as publishers; Box and Sumo Logic documented as non-publishers; Dynatrace is tier D (per-tenant authenticated API only); Segment/Amplitude/Mixpanel have no public range documentation (vendor-outreach candidates); Scaleway/OVH/Hetzner expose no geofeeds at standard URLs.

Second pass (2026-07-10, same day): **Buildkite, Tenable, Rapid7, Elastic Cloud, Neon, Klaviyo, and Twilio SIP Trunking integrated as publishers** (all endpoints fetched live; 304 support verified for the three JSON feeds). Dead ends recorded in `sources.yaml → backlog.partially_verified`: Qualys *does* publish scanner ranges but only on client-rendered pages; Iterable's help center bot-blocks all non-browser fetchers (403); RingCentral's supernets live in an SPA-only support portal; Braze's IP page moved (docs are open source — locate the successor page). ServiceNow, Workday, NetSuite, CrowdStrike, and SentinelOne publish only behind customer login per prior research notes — still unverified.

## Verification artifacts

Raw fetched bodies + response headers for every endpoint are archived from the research session and become the initial parser test fixtures. Each `sources.yaml` entry carries `verified: 2026-07-04`; re-verification is automated once the pipeline exists (a source failing to fetch for >24 h flags `needs_attention`).
