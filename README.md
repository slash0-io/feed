# slash0 feed

The versioned public feed of third-party service IP ranges behind [terraform-provider-egress](https://github.com/slash0-io/terraform-provider-egress): **45 services** (Stripe, GitHub, Datadog, Okta, Cloudflare, AWS, Azure, Anthropic, PayPal, Zendesk, …) plus **16 documented non-publishers**, built exclusively from each vendor's **official publication**, never from third-party aggregators.

Feed URL: `https://feed.slash0.io/v1` (`index.json` + `services/<slug>.json`).

**→ [Browse the full service catalog](CATALOG.md)**: every slug, purpose, direction, and classification. Generated from `sources.yaml` and CI-enforced to never drift; also served human-readable at the [feed root](https://feed.slash0.io/).

## Why trust it

- **[sources.yaml](sources.yaml)** is the complete registry: every vendor's official endpoint, data format, documented update cadence, and change-detection method, each verified live on the date recorded.
- **Vendor claims are cited.** A documented notice period or a vendor-operated change signal cannot be recorded without the vendor page that states it; the build fails otherwise. Those citations render as links in [research/SOURCES.md](research/SOURCES.md), whose tables are generated from the registry so they cannot drift from it.
- **[research/SOURCES.md](research/SOURCES.md)** also documents the methodology and the change-detection architecture (push subscriptions, conditional-GET polling, content hashing, docs-page watching), and states plainly which parts of it run today.
- Every published service document carries a **provenance chain**: upstream URL, retrieval timestamp, and SHA-256 of the upstream body it was derived from.
- **Guardrails**: private/loopback/default-route entries are dropped; a purpose that parses to zero ranges fails the build; and a change that removes more than half of a purpose's previously published ranges is **quarantined**, so the last-good version keeps serving while the build alerts for human review.
- **Aggregation is lossless.** Published coverage is preserved exactly and never widened, so an aggregated entry can never grant more than the vendor published.
- **Incremental publishing**: unchanged services republish byte-for-byte (sync tokens preserved), deploys are skipped entirely when nothing changed, and every real change lands in `changelog.json` with per-purpose added/removed counts.
- **Signed**: `v1/index.json.sig` is an ECDSA P-256 detached signature over the exact bytes of `index.json` (format `keyid:base64`), produced by a non-exportable key in AWS KMS. Service documents are covered transitively via their SHA-256 in the index: verify the signature, then verify each document's hash. Public key (SPKI, base64) `MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEqevHf1FrgbrKV5zzV+C1yjoOvi11k2+y5sDFwSpNqhuS1yeJaBFSVUFYCc39pqlNST+vR/Jd4DMwfjjkVWovTQ==` (keyid `9823e091`). Verification commands: [slash0.io/docs/security](https://slash0.io/docs/security/).
- Parsers are tested against **archived upstream fixtures** (`testdata/fixtures/`), so the build runs fully offline in CI.

## What the schema models that upstreams don't

- **`direction`** per purpose: `egress` = ranges you connect *to* (SG egress rules); `ingress` = ranges the service connects *from*, meaning webhook and agent sources, which belong in SG ingress rules.
- **`classification`** per service: `dedicated` | `mixed` | `cdn-shared`. Pinning a CDN-fronted service allowlists the whole CDN, and the feed says so rather than pretending otherwise.
- **Non-publishers**: services whose vendors state IP pinning is unsupported (Twilio, Adyen, Slack, …), with their recommended alternative, published in `index.json`.

## Usage

```sh
go run ./generator                              # build dist/v1 from live vendor endpoints
go run ./generator -fixtures testdata/fixtures  # build offline from archived fixtures
go run ./generator -services stripe,github      # subset
go test ./...
```

Regenerate the committed documents after changing `sources.yaml`:

```sh
go run ./generator -catalog CATALOG.md
go run ./generator -sources-doc research/SOURCES.md
```

## Hosted tier

Want these ranges as AWS-managed prefix lists kept current in your account, with no `terraform apply` required? The hosted tier is in development with design partners: [request early access](https://github.com/slash0-io/terraform-provider-egress/issues/new?template=early-access.yml).

## Contributing a source

Add an entry to `sources.yaml` (official vendor endpoint only, with a provenance link), a parser for its format in `generator/parsers.go` if new, and a fixture in `testdata/fixtures/`. CI requires the offline build to pass. A `notice` or `detection.push` value needs its evidence URL alongside it.

## Roadmap

Push-based rebuilds (AWS SNS, Microsoft 365 `/version`), sub-minute polling via a long-lived poller (a GitHub Actions cron floors at roughly five minutes), and parser upgrades where a vendor has since published a better source than the one being read.

A scheduled source audit, separate from the publish cron. The publish only
notices a source that fails to parse; it says nothing about a citation that has
quietly gone dead. Three failure modes have shown up in practice and a weekly
job would catch all three:

- **Evidence and provenance link rot.** Two cited vendor pages went to 401 and
  403 within a fortnight, which leaves dead links on a page whose whole point is
  that every claim is checkable.
- **Sources that parse but should not.** A vendor reorganising a docs page can
  leave a heading select matching a smaller section rather than failing outright.
  Comparing published address counts against the previous run would flag it.
- **Prose drift.** Hand-written text keeps falling behind the generated tables:
  the tier examples in `research/SOURCES.md` outlived two vendors being
  reclassified, and `/docs/coverage/` described the unpinnable set as "small"
  after it reached two in five.

Two source types that vendor research has shown are needed:

- **Set subtraction across endpoints.** Google publishes `goog.json` and `cloud.json` and documents everything else as the difference between them, so a Workspace purpose is `goog` minus `cloud`. A purpose currently reads one endpoint, so this cannot be expressed.
- **DNS as a source.** Oracle documents `outboundips.netsuite.com` rather than a list as the way to track NetSuite outbound addresses. It answers with 41 A records, which exceeds a 512-byte UDP response, so an implementation needs EDNS0 or TCP and must treat a short answer as a fetch failure rather than as a removal.

Vendors still unresolved are tracked in the `backlog` block of `sources.yaml`, grouped by why: `blocked` where the vendor publishes but the document is unreachable, `no_source_found` where nothing public exists, `non_publisher_candidates` where a per-customer source exists and only an evidence URL is missing, and `declined`.

## License

[MPL-2.0](LICENSE)
