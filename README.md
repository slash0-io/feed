# egress feed

The versioned public feed of third-party service IP ranges behind [terraform-provider-egress](https://github.com/egresshq/terraform-provider-egress): **25+ services** (Stripe, GitHub, Datadog, Okta, Cloudflare, AWS, …), built exclusively from each vendor's **official publication** — never from third-party aggregators.

Feed URL: `https://egresshq.github.io/feed/v1` (`index.json` + `services/<slug>.json`).

## Why trust it

- **[sources.yaml](sources.yaml)** is the complete registry: every vendor's official endpoint, data format, documented update cadence, and change-detection method — each verified live on the date recorded.
- **[research/SOURCES.md](research/SOURCES.md)** documents the methodology, per-source evidence, and the change-detection architecture (push subscriptions, conditional-GET polling, content hashing, docs-page watching).
- Every published service document carries a **provenance chain**: upstream URL, retrieval timestamp, and SHA-256 of the upstream body it was derived from.
- **Guardrails**: private/loopback/default-route entries are dropped; a purpose that parses to zero ranges fails the build rather than publishing an empty allowlist.
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

## Contributing a source

Add an entry to `sources.yaml` (official vendor endpoint only, with provenance link), a parser for its format in `generator/parsers.go` if new, and a fixture in `testdata/fixtures/`. CI requires the offline build to pass.

## Roadmap

Docs-page sources (Anthropic, GitLab, Sentry, New Relic, Postmark, DocuSign), Azure Service Tags, feed signing, push-based rebuilds (AWS SNS, M365 `/version`), and the ~50-service research backlog in `sources.yaml`.

## License

[MPL-2.0](LICENSE)
