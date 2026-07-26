# Service catalog

<!-- GENERATED from sources.yaml — do not edit by hand.
     Regenerate: go run ./generator -catalog CATALOG.md -->

Every service below publishes official IP ranges, verified against the vendor's own
publication. Use the **slug** and a **purpose** key with the Terraform provider:

```hcl
data "egress_ranges" "stripe_api" {
  service = "stripe"   # slug
  purpose = "api"      # purpose key
}
```

**Direction** is read from your workload's point of view: `egress` ranges are what you
connect *to* (security-group egress rules); `ingress` ranges are what the service
connects *from* — webhook and agent sources that belong in ingress rules.

**Classification**: `dedicated` = vendor-owned space, safe to pin · `mixed` = partly
shared or dynamic · `cdn-shared` = shared CDN ranges (pinning allowlists the whole CDN).

## Cloud / IaaS

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `aws` | Amazon Web Services | dedicated | `all` (egress), `cloudfront` (egress), `dynamodb` (egress), `route53-healthchecks` (ingress), `s3` (egress) | [official ↗](https://docs.aws.amazon.com/vpc/latest/userguide/aws-ip-ranges.html) |
| `azure` | Microsoft Azure (Service Tags) | dedicated | `all` (egress), `sql` (egress), `storage` (egress) | [official ↗](https://learn.microsoft.com/en-us/azure/virtual-network/service-tags-overview) |
| `digitalocean` | DigitalOcean | dedicated | `all` (egress) | [official ↗](https://www.digitalocean.com/geo/google.csv) |
| `google` | Google (all services) | dedicated | `all` (egress) | [official ↗](https://support.google.com/a/answer/10026322) |
| `google-cloud` | Google Cloud Platform | dedicated | `all` (egress) | [official ↗](https://support.google.com/a/answer/10026322) |
| `ibm-cloud` | IBM Cloud (Classic infrastructure) | dedicated | `frontend` (egress), `load-balancers` (egress) | [official ↗](https://cloud.ibm.com/docs/infrastructure-hub?topic=infrastructure-hub-ibm-cloud-ip-ranges) |
| `linode` | Akamai Connected Cloud (Linode) | dedicated | `all` (egress) | [official ↗](https://geoip.linode.com/) |
| `oracle-cloud` | Oracle Cloud Infrastructure | dedicated | `all` (egress), `object-storage` (egress) | [official ↗](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/addressranges.htm) |
| `vultr` | Vultr (Constant, AS20473) | dedicated | `all` (egress) | [official ↗](https://geofeed.constant.com/) |

## CDN / Edge

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `cloudflare` | Cloudflare | cdn-shared | `edge` (both) | [official ↗](https://www.cloudflare.com/ips/) |
| `fastly` | Fastly | cdn-shared | `edge` (both) | [official ↗](https://www.fastly.com/documentation/reference/api/utils/public-ip-list/) |

## Payments & fintech

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `braintree` | Braintree (PayPal) | dedicated | `api` (egress), `sandbox` (egress) | [official ↗](https://developer.paypal.com/braintree/docs/reference/general/braintree-ip-addresses) |
| `paypal` | PayPal | dedicated | `all` (both) | [official ↗](https://www.paypal.com/us/cshelp/article/what-are-the-internet-protocol-ip-addresses-for-paypal-server-endpoints-ts1056) |
| `plaid` | Plaid | dedicated | `webhooks` (ingress) | [official ↗](https://plaid.com/docs/api/webhooks/) |
| `stripe` | Stripe | dedicated | `api` (egress), `terminal` (egress), `webhooks` (ingress) | [official ↗](https://docs.stripe.com/ips) |

## Observability

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `datadog` | Datadog | dedicated | `agents-eu` (egress), `agents` (egress), `api-eu` (egress), `api` (egress), `apm` (egress), `logs` (egress), `synthetics` (ingress), `webhooks` (ingress) | [official ↗](https://docs.datadoghq.com/api/latest/ip-ranges/) |
| `grafana-cloud` | Grafana Cloud | dedicated | `alerts` (ingress), `logs` (ingress), `metrics` (ingress) | [official ↗](https://grafana.com/docs/grafana-cloud/security-and-account-management/allow-list/) |
| `new-relic` | New Relic | dedicated | `agents` (egress), `synthetics` (ingress) | [official ↗](https://docs.newrelic.com/docs/new-relic-solutions/get-started/networks/) |
| `sentry` | Sentry (hosted) | dedicated | `ingest` (egress), `uptime` (ingress), `webhooks` (ingress) | [official ↗](https://docs.sentry.io/security-legal-pii/security/ip-ranges/) |

## Developer platforms & CI

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `buildkite` | Buildkite | dedicated | `webhooks` (ingress) | [official ↗](https://buildkite.com/docs/apis/rest-api/meta) |
| `circleci` | CircleCI | dedicated | `core` (egress), `jobs` (ingress) | [official ↗](https://circleci.com/docs/guides/security/ip-ranges/) |
| `github` | GitHub | dedicated | `actions` (ingress), `api` (egress), `git` (egress), `hooks` (ingress), `packages` (egress), `pages` (egress), `web` (egress) | [official ↗](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses) |
| `gitlab` | GitLab.com | mixed | `web-api` (egress), `webhooks` (ingress) | [official ↗](https://docs.gitlab.com/user/gitlab_com/) |

## Data platforms

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `databricks` | Databricks | dedicated | `all` (both) | [official ↗](https://docs.databricks.com/aws/en/resources/ip-domain-region) |
| `elastic-cloud` | Elastic Cloud | dedicated | `api` (egress), `outbound` (ingress) | [official ↗](https://www.elastic.co/docs/deploy-manage/security/elastic-cloud-static-ips) |
| `neon` | Neon (Serverless Postgres) | dedicated | `outbound` (ingress) | [official ↗](https://neon.com/docs/introduction/regions) |

## Identity & auth

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `auth0` | Auth0 (Okta CIC) | dedicated | `all` (both) | [official ↗](https://auth0.com/docs/secure/security-guidance/data-security/allowlist) |
| `okta` | Okta | dedicated | `all` (both) | [official ↗](https://help.okta.com/en-us/content/topics/security/ip-address-allow-listing.htm) |

## Communications

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `intercom` | Intercom | dedicated | `all-eu` (both), `all` (both), `outbound-webhooks` (ingress) | [official ↗](https://developers.intercom.com/docs/webhooks) |
| `klaviyo` | Klaviyo | dedicated | `integrations` (ingress) | [official ↗](https://help.klaviyo.com/hc/en-us/articles/19143781289115) |
| `pagerduty` | PagerDuty | dedicated | `webhooks-eu` (ingress), `webhooks` (ingress) | [official ↗](https://support.pagerduty.com/main/docs/safelist-ips) |
| `postmark` | Postmark | dedicated | `smtp` (egress), `webhooks` (ingress) | [official ↗](https://postmarkapp.com/support/article/800-ips-for-firewalls) |
| `twilio-sip` | Twilio Elastic SIP Trunking | dedicated | `sip-media` (both), `sip-signaling` (both) | [official ↗](https://www.twilio.com/docs/sip-trunking/ip-addresses) |
| `zoom` | Zoom | dedicated | `all` (egress) | [official ↗](https://support.zoom.com/hc/en/article?id=zm_kb&sysparm_article=KB0060548) |

## Business SaaS

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `atlassian` | Atlassian Cloud | dedicated | `all` (both), `bitbucket` (both), `egress` (ingress) | [official ↗](https://support.atlassian.com/organization-administration/docs/ip-addresses-and-domains-for-atlassian-cloud-products/) |
| `docusign` | DocuSign | mixed | `connect-webhooks` (ingress), `email` (ingress) | [official ↗](https://www.docusign.com/trust/security/esignature) |
| `microsoft-365` | Microsoft 365 | mixed | `common` (egress), `exchange` (egress), `sharepoint` (egress), `teams` (egress) | [official ↗](https://learn.microsoft.com/en-us/microsoft-365/enterprise/microsoft-365-ip-web-service) |
| `salesforce` | Salesforce | dedicated | `all` (both) | [official ↗](https://help.salesforce.com/s/articleView?id=000384438 (IP Addresses and Domains to Allow)) |
| `zendesk` | Zendesk | mixed | `api` (egress), `webhooks` (ingress) | [official ↗](https://developer.zendesk.com/api-reference/ticketing/account-configuration/public_ips/) |

## Security

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `netskope` | Netskope (NewEdge) | dedicated | `dataplane` (both) | [official ↗](https://docs.netskope.com/en/newedge-ip-ranges-for-allowlisting) |
| `rapid7` | Rapid7 (InsightAppSec cloud engines) | dedicated | `appsec-engines` (ingress) | [official ↗](https://docs.rapid7.com/insightappsec/allowlist-cloud-engine-ips/) |
| `tenable` | Tenable (Vulnerability Management) | dedicated | `scanners` (ingress) | [official ↗](https://docs.tenable.com/vulnerability-management/Content/Settings/Sensors/CloudSensors.htm) |
| `zscaler` | Zscaler (zscaler.net cloud) | dedicated | `enforcement-nodes` (egress) | [official ↗](https://config.zscaler.com/) |

## AI APIs

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `anthropic` | Anthropic (Claude API) | dedicated | `api` (egress), `outbound` (ingress) | [official ↗](https://platform.claude.com/docs/en/api/ip-addresses) |
| `openai` | OpenAI (ChatGPT egress) | mixed | `agents` (ingress), `connectors` (ingress) | [official ↗](https://developers.openai.com/api/docs/guides/ip-addresses) |

## Services that do NOT publish pinnable ranges

These vendors state that IP allowlisting is unsupported or unreliable for their service.
Published in the feed (`index.json` → `nonPublishers`) so tooling can surface it.

| Service | Vendor position |
|---|---|
| Twilio (REST API + webhooks) | IPs 'highly dynamic, span a large range'; allow *.twilio.com instead. SIP trunking IS pinnable, see the twilio-sip service. |
| Adyen | No IP list; allowlist out.adyen.com or resolve it via DNS hourly (their words). |
| SendGrid (webhooks/parse) | Dynamic cloud infra; use signed webhooks, not IP allowlists. |
| Slack | No published egress IPs; their allowlisting feature restricts YOUR IPs calling THEM. |
| Shopify (webhooks) | Webhook source IPs not published; verify HMAC signatures instead. |
| Square (webhooks) | No webhook source-IP list published; validate notifications via the documented HMAC-SHA256 signature flow. |
| Duo Security | Explicitly discourages IP-based egress rules; may change to maintain availability. |
| Snowflake | Deployment-specific hostnames/IPs per account; no global list. |
| MongoDB Atlas (data plane) | Cluster IPs are per-project/dynamic; control-plane IPs only via authenticated Admin API. Vendor directs users to private endpoints (PrivateLink). |
| Vercel (function egress) | Dynamic by default; static IPs are a paid per-customer feature, not a public range. |
| Netlify (function egress) | No published IPs; any circulating list 'is a guess or outdated'. |
| HubSpot | No static publication ('change too frequently'); an API returning the current ranges is available to authenticated callers. |
| OpenAI API (api.openai.com) | Egress to api.openai.com resolves to shared Cloudflare ranges; pinning would allowlist the whole CDN. |
| Akamai (CDN) | Customer-specific maps behind login; no public global ranges. |
| Box | Use domain names; 'IP addresses can change frequently and without notice' (their wording). No webhook source ranges published. |
| Sumo Logic | No own ranges; officially delegates to AWS ip-ranges.json ('Amazon'/'EC2' services for your deployment region), covered by the aws service. |

Evidence links for every row: [`sources.yaml`](sources.yaml) · methodology: [`research/SOURCES.md`](research/SOURCES.md)
