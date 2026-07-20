# egress feed

The versioned public feed of third-party service IP ranges behind [terraform-provider-egress](https://github.com/slash0-io/terraform-provider-egress): **45 services** (Stripe, GitHub, Datadog, Okta, Cloudflare, AWS, Azure, Anthropic, PayPal, Zendesk, …) plus **16 documented non-publishers**, built exclusively from each vendor's **official publication** — never from third-party aggregators.

Feed URL: `https://feed.slash0.io/v1` (`index.json` + `services/<slug>.json`).

**→ [Browse the full service catalog](CATALOG.md)** — every slug, purpose, direction, and classification. Auto-generated from `sources.yaml` and CI-enforced to never drift; also served human-readable at the [feed root](https://feed.slash0.io/).

## Why trust it

- **[sources.yaml](sources.yaml)** is the complete registry: every vendor's official endpoint, data format, documented update cadence, and change-detection method — each verified live on the date recorded.
- **[research/SOURCES.md](research/SOURCES.md)** documents the methodology, per-source evidence, and the change-detection architecture (push subscriptions, conditional-GET polling, content hashing, docs-page watching).
- Every published service document carries a **provenance chain**: upstream URL, retrieval timestamp, and SHA-256 of the upstream body it was derived from.
- **Guardrails**: private/loopback/default-route entries are dropped; a purpose that parses to zero ranges fails the build; and a change that removes most of a service's previously published ranges is **quarantined** — the last-good version keeps serving while the build alerts for human review.
- **Incremental publishing**: unchanged services republish byte-for-byte (sync tokens preserved), deploys are skipped entirely when nothing changed, and every real change lands in `changelog.json` with per-purpose added/removed counts.
- **Signed**: `v1/index.json.sig` is an ECDSA P-256 detached signature over the exact bytes of `index.json` (format `keyid:base64`), produced by a non-exportable key in AWS KMS. Service documents are covered transitively via their SHA-256 in the index: verify the signature, then verify each document's hash. Public key (SPKI, base64) `MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEqevHf1FrgbrKV5zzV+C1yjoOvi11k2+y5sDFwSpNqhuS1yeJaBFSVUFYCc39pqlNST+vR/Jd4DMwfjjkVWovTQ==` (keyid `9823e091`). Verification commands: [slash0.io/docs/security](https://slash0.io/docs/security/).
- Parsers are tested against **archived upstream fixtures** (`testdata/fixtures/`), so the build runs fully offline in CI.

## What the schema models that upstreams don't

- **`direction`** per purpose: `egress` = ranges you connect *to* (SG egress rules); `ingress` = ranges the service connects *from* (webhook/agent sources — SG ingress rules).
- **`classification`** per service: `dedicated` | `mixed` | `cdn-shared` (pinning a CDN-fronted service allowlists the whole CDN — the feed says so instead of pretending otherwise).
- **Non-publishers**: services whose vendors state IP pinning is unsupported (Twilio, Adyen, Slack, …), with their recommended alternative, published in `index.json`.

## Usage

```sh
go run ./generator                              # build dist/v1 from live vendor endpoints
go run ./generator -fixtures testdata/fixtures  # build offline from archived fixtures
go run ./generator -services stripe,github      # subset
go test ./...
```

## Hosted tier

Want these ranges as AWS-managed prefix lists kept current in your account — no `terraform apply` required? The hosted tier is in development with design partners: [request early access](https://github.com/slash0-io/terraform-provider-egress/issues/new?template=early-access.yml).

## Contributing a source

Add an entry to `sources.yaml` (official vendor endpoint only, with provenance link), a parser for its format in `generator/parsers.go` if new, and a fixture in `testdata/fixtures/`. CI requires the offline build to pass.

## Roadmap

DocuSign (Trust Center page URL unresolved), push-based rebuilds (AWS SNS, M365 `/version`), sub-minute polling via a long-lived poller (GitHub Actions cron floors at ~5 min), and the ~50-service research backlog in `sources.yaml`.

## License

[MPL-2.0](LICENSE)
