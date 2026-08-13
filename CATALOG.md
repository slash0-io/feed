# Service catalog

<!-- GENERATED from sources.yaml. Do not edit by hand.
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
connects *from*, meaning webhook and agent sources that belong in ingress rules.

**Classification**: `dedicated` = vendor-owned space, safe to pin · `mixed` = partly
shared or dynamic · `cdn-shared` = shared CDN ranges (pinning allowlists the whole CDN).

## Cloud / IaaS

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `aws` | Amazon Web Services | dedicated | `all` (egress), `cloudfront` (egress), `dynamodb` (egress), `route53-healthchecks` (ingress), `s3` (egress) | [official ↗](https://docs.aws.amazon.com/vpc/latest/userguide/aws-ip-ranges.html) |
| `azure` | Microsoft Azure (Service Tags) | dedicated | `all` (egress), `sql` (egress), `storage` (egress) | [official ↗](https://learn.microsoft.com/en-us/azure/virtual-network/service-tags-overview) |
| `digitalocean` | DigitalOcean | dedicated | `all` (egress) | [official ↗](https://www.digitalocean.com/geo/google.csv) |
| `google` | Google (all services) | dedicated | `all` (egress) | [official ↗](https://knowledge.workspace.google.com/admin/security/obtain-google-ip-address-ranges) |
| `google-cloud` | Google Cloud Platform | dedicated | `all` (egress) | [official ↗](https://knowledge.workspace.google.com/admin/security/obtain-google-ip-address-ranges) |
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
| `checkout-com` | Checkout.com | mixed | `webhooks` (ingress) | [official ↗](https://www.checkout.com/docs/developer-resources/ip-addresses) |
| `paypal` | PayPal | dedicated | `all` (both) | [official ↗](https://www.paypal.com/us/cshelp/article/what-are-the-internet-protocol-ip-addresses-for-paypal-server-endpoints-ts1056) |
| `plaid` | Plaid | dedicated | `webhooks` (ingress) | [official ↗](https://plaid.com/docs/api/webhooks/) |
| `stripe` | Stripe | dedicated | `api` (egress), `terminal` (egress), `webhooks` (ingress) | [official ↗](https://docs.stripe.com/ips) |
| `wise` | Wise | dedicated | `webhooks-sandbox` (ingress), `webhooks` (ingress) | [official ↗](https://docs.wise.com/api-docs/features/webhooks-notifications) |

## Observability

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `appdynamics` | Splunk AppDynamics SaaS | mixed | `platform-sources` (ingress), `synthetic-agents` (ingress) | [official ↗](https://help.splunk.com/en/appdynamics-saas/product-announcements-and-alerts/saas-domains-and-ip-ranges) |
| `checkly` | Checkly | dedicated | `probes` (ingress) | [official ↗](https://www.checklyhq.com/docs/platform/allowlisting-traffic/) |
| `datadog` | Datadog | dedicated | `agents-eu` (egress), `agents` (egress), `api-eu` (egress), `api` (egress), `apm` (egress), `logs` (egress), `synthetics` (ingress), `webhooks` (ingress) | [official ↗](https://docs.datadoghq.com/api/latest/ip-ranges/) |
| `grafana-cloud` | Grafana Cloud | dedicated | `alerts` (ingress), `logs` (ingress), `metrics` (ingress) | [official ↗](https://grafana.com/docs/grafana-cloud/security-and-account-management/allow-list/) |
| `new-relic` | New Relic | dedicated | `agents` (egress), `synthetics` (ingress) | [official ↗](https://docs.newrelic.com/docs/new-relic-solutions/get-started/networks/) |
| `pingdom` | Pingdom | dedicated | `probes` (ingress) | [official ↗](https://my.pingdom.com/probes/feed) |
| `sentry` | Sentry (hosted) | dedicated | `ingest` (egress), `uptime` (ingress), `webhooks` (ingress) | [official ↗](https://docs.sentry.io/security-legal-pii/security/ip-ranges/) |
| `uptimerobot` | UptimeRobot | dedicated | `probes` (ingress) | [official ↗](https://uptimerobot.com/help/locations/) |

## Developer platforms & CI

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `buildkite` | Buildkite | dedicated | `webhooks` (ingress) | [official ↗](https://buildkite.com/docs/apis/rest-api/meta) |
| `circleci` | CircleCI | dedicated | `core` (egress), `jobs` (ingress) | [official ↗](https://circleci.com/docs/guides/security/ip-ranges/) |
| `github` | GitHub | dedicated | `actions` (ingress), `api` (egress), `git` (egress), `hooks` (ingress), `packages` (egress), `pages` (egress), `web` (egress) | [official ↗](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses) |
| `gitlab` | GitLab.com | mixed | `web-api` (egress), `webhooks` (ingress) | [official ↗](https://docs.gitlab.com/user/gitlab_com/) |
| `launchdarkly` | LaunchDarkly | dedicated | `service` (egress), `webhooks` (ingress) | [official ↗](https://launchdarkly.com/docs/home/infrastructure/ip-list) |
| `retool` | Retool | dedicated | `default-region` (ingress) | [official ↗](https://docs.retool.com/data-sources/reference/ip-allowlist-cloud-orgs) |
| `svix` | Svix | dedicated | `webhooks` (ingress) | [official ↗](https://docs.svix.com/receiving/source-ips) |

## Data platforms

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `airbyte` | Airbyte Cloud | dedicated | `all` (ingress) | [official ↗](https://docs.airbyte.com/platform/operating-airbyte/ip-allowlist) |
| `databricks` | Databricks | dedicated | `all` (both) | [official ↗](https://docs.databricks.com/aws/en/resources/ip-domain-region) |
| `dbt-cloud` | dbt Cloud | dedicated | `all` (ingress) | [official ↗](https://docs.getdbt.com/docs/cloud/about-cloud/access-regions-ip-addresses) |
| `elastic-cloud` | Elastic Cloud | dedicated | `api` (egress), `outbound` (ingress) | [official ↗](https://www.elastic.co/docs/deploy-manage/security/elastic-cloud-static-ips) |
| `fivetran` | Fivetran | dedicated | `sync` (ingress), `webhooks` (ingress) | [official ↗](https://fivetran.com/docs/using-fivetran/ips) |
| `hightouch` | Hightouch | dedicated | `all` (ingress) | [official ↗](https://hightouch.com/docs/security/networking) |
| `neon` | Neon (Serverless Postgres) | dedicated | `outbound` (ingress) | [official ↗](https://neon.com/docs/introduction/regions) |

## Identity & auth

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `auth0` | Auth0 (Okta CIC) | dedicated | `all` (both) | [official ↗](https://auth0.com/docs/secure/security-guidance/data-security/allowlist) |
| `duo` | Cisco Duo | dedicated | `mfa` (egress) | [official ↗](https://help.duo.com/s/article/1337) |
| `okta` | Okta | dedicated | `all` (both) | [official ↗](https://help.okta.com/en-us/content/topics/security/ip-address-allow-listing.htm) |
| `onelogin` | OneLogin | mixed | `agents-eu` (egress), `agents` (egress), `email` (ingress) | [official ↗](https://onelogin.service-now.com/kb_view_customer.do?sysparm_article=KB0010432) |
| `workos` | WorkOS | mixed | `webhooks` (ingress) | [official ↗](https://workos.com/docs/events/data-syncing/webhooks) |

## Communications

| Slug | Service | Classification | Purposes | Source |
|---|---|---|---|---|
| `braze` | Braze | dedicated | `connected-content` (ingress) | [official ↗](https://www.braze.com/docs/user_guide/messaging/design_and_edit/personalize/connected_content/making_an_api_call) |
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
| `hubspot` | HubSpot | dedicated | `api` (ingress), `crawlers` (ingress), `dns` (ingress), `email` (ingress) | [official ↗](https://developers.hubspot.com/docs/api-reference/2026-09-beta/account/ip-ranges/guide) |
| `make` | Make | mixed | `platform` (ingress) | [official ↗](https://help.make.com/allow-connections-to-and-from-make-ip-addresses) |
| `microsoft-365` | Microsoft 365 | mixed | `common` (egress), `exchange` (egress), `sharepoint` (egress), `teams` (egress) | [official ↗](https://learn.microsoft.com/en-us/microsoft-365/enterprise/microsoft-365-ip-web-service) |
| `netsuite-connector` | Oracle NetSuite Connector | dedicated | `connector` (ingress) | [official ↗](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_162859452018.html) |
| `salesforce` | Salesforce | dedicated | `all` (both) | [official ↗](https://help.salesforce.com/s/articleView?id=000384438 (IP Addresses and Domains to Allow)) |
| `workato` | Workato | dedicated | `on-prem-agent` (egress), `platform` (ingress) | [official ↗](https://docs.workato.com/security/ip-allowlists.html) |
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
| Twilio (REST API + webhooks) | IPs 'highly dynamic, and span a large range, so it's impractical to list each of them'; they recommend allowing outbound HTTPS to any *.twilio.com subdomain instead. SIP trunking IS pinnable, see the twilio-sip service. |
| Adyen | No IP list; allowlist out.adyen.com or resolve it via DNS hourly (their words). |
| SendGrid (webhooks/parse) | Dynamic cloud infra; use signed webhooks, not IP allowlists. |
| Slack | No published egress IPs; their allowlisting feature restricts YOUR IPs calling THEM. |
| Shopify (webhooks) | Documents HMAC-SHA256 signature verification as the way to authenticate a webhook. The page does not mention source IPs, and no official range list was found as of the verified date. |
| Square (webhooks) | No webhook source-IP list published; validate notifications via the documented HMAC-SHA256 signature flow. |
| Snowflake | Deployment-specific hostnames/IPs per account; no global list. |
| MongoDB Atlas (data plane) | Cluster IPs are per-project/dynamic; control-plane IPs only via authenticated Admin API. Vendor directs users to private endpoints (PrivateLink). |
| Vercel (function egress) | Dynamic by default; static IPs are a paid per-customer feature, not a public range. |
| JumpCloud | States for the agent, LDAP-as-a-Service and AD Integration alike: 'Due to the elastic nature of the JumpCloud infrastructure, we currently do not publish lists of IP addresses for allow lists'. Directs users to FQDNs instead. Their separate data-centre page does list six regional RADIUS anycast addresses, which is a single narrow endpoint rather than a service range set. |
| Stytch (webhooks) | Publishes no source-IP list of its own; states 'Stytch's webhooks are powered through Svix'. A receiving endpoint allowlists the svix service in this feed instead. Their own IP feature runs the other way: up to 10 customer IPs that may call the Stytch API, arranged over email with support. |
| Netlify (function egress) | States that by default the addresses builds and functions connect from 'will fluctuate when we scale up and down'. A static set is available only through the Private Connectivity add-on on Enterprise plans. |
| OpenAI API (api.openai.com) | api.openai.com resolves into Cloudflare's published ranges (verified 2026-07-30: 172.66.0.243 within 172.64.0.0/13, 162.159.140.245 within 162.158.0.0/15), so pinning it would allowlist the whole CDN rather than OpenAI. |
| Akamai (CDN) | Site Shield issues a per-customer set of IP subnet ranges (a map) retrieved through Akamai Control Center or the Site Shield API, rather than one public global list. |
| Box | Use domain names; 'IP addresses can change frequently and without notice' (their wording). No webhook source ranges published. |
| Sumo Logic | No own ranges; directs users to download the AWS IP ranges JSON and use the prefixes for the AWS region their deployment sits in, which the aws service already covers. States plainly that 'the list of IP ranges is shared infrastructure. It is not limited to Sumo Logic nodes and is subject to change over time.' |
| Oracle NetSuite (platform) | States plainly that Oracle 'does not support the use of NetSuite IP addresses to access or manage access to any NetSuite services', that outbound addresses are 'not documented in the NetSuite Help Center or in SuiteAnswers', and that *.netsuite.com is CDN-fronted. Directs users to 2FA, token-based auth and OAuth 2.0 instead, or to a DNS lookup on outboundips.netsuite.com. NetSuite Connector IS published, see the netsuite-connector service. |
| Confluent Cloud | Public egress addresses are read from the Cloud Console or the authenticated Cloud REST API (api.confluent.cloud/networking/v1/ip-addresses), are shared by every customer in the same cloud and region, and are 'not guaranteed to be static' (their wording). |
| Palo Alto Networks Prisma Access | Egress addresses are allocated per tenant and retrieved with your own API key from api.prod.datapath.prismaaccess.com, or read per location in the Prisma Access UI. |
| Splunk Cloud Platform | The documented control is an IP allow list restricting which addresses on your own network reach each Splunk feature, managed through the Admin Config Service API. Splunk publishes no ranges of its own for the stack. |
| CockroachDB Cloud | The documented controls are an allowlist of your own authorized networks and private connectivity through AWS PrivateLink, GCP Private Service Connect or Azure Private Link. |
| Redis Cloud | The CIDR allow list restricts which of your own addresses may reach your database, between 4 and 32 entries depending on plan. Redis publishes no ranges of its own. |
| Aiven | Services are addressed by hostname; static IP addresses are a paid per-project resource created and attached with the avn static-ip CLI, not a public range list. |
| Supabase | States that 'IPv4 addresses are guaranteed to be static for ingress traffic' through a per-project paid add-on, while 'the outbound IP address is not static and cannot be guaranteed'. |
| PlanetScale | The addresses to allowlist are shown in the console during the import workflow, differ by region, and the vendor directs users to read them there each time because they 'can change occasionally'. |
| Docker Hub / Docker Desktop | Publishes an allowlist of domain URLs rather than addresses; the page lists hostnames only. Reproducible on the data plane: registry-1.docker.io resolves into rotating AWS us-east-1 EC2 addresses, a different set on each query (verified 2026-07-30). |
| Mailchimp Transactional (Mandrill) webhooks | Directs users to authenticate that a webhook originated from Mailchimp's servers using the documented request-signature flow. The /ips/ API returns your own dedicated sending addresses, which is a different thing from webhook sources. |
| Honeycomb | Offers AWS PrivateLink to the Honeycomb API for Enterprise customers on AWS. No range list is published in the docs. |
| Snyk | The Broker Client opens the outbound WebSocket and Snyk rides it back, so in their words 'you do not need to allow a Snyk IP address. Instead, you can allow the Broker Client IP/port.' Requests to Snyk go through a CDN that rotates addresses and whole ranges, and they direct users to allow *.snyk.io. |
| Zapier | States that Zapier 'uses Amazon (AWS)'s us-east-1 region, where it dynamically provisions instances as needed', so there is no fixed set. They suggest matching the User-Agent: Zapier header instead, or the static IP feature available on paid plans. |
| Mailgun | Their IP Allowlist API 'lets you view and manage allowlisted IP addresses to which API key and SMTP credential usage is restricted', which controls your own callers rather than publishing Mailgun's addresses. Outbound sending addresses are per-account dedicated IPs grouped into pools, read through the authenticated /v3/ips API. |
| Dynatrace (Synthetic Monitoring) | Public Synthetic location addresses are read per environment, either from the Frequency and locations page in the web UI ('Copy IPs to clipboard or Download IPs') or from the Synthetic locations API, which 'returns all the locations available for your Environment along with their IP addresses'. No global list is published. |
| npm registry (registry.npmjs.org) | registry.npmjs.org resolves into Cloudflare's published ranges (verified 2026-07-30: 104.16.0.34 and 104.16.1.34, both inside 104.16.0.0/13), so pinning it would allowlist the whole CDN rather than npm. |
| PyPI (pypi.org, files.pythonhosted.org) | Both pypi.org and files.pythonhosted.org resolve into Fastly's published ranges (verified 2026-07-30: 151.101.0.223 and 151.101.128.223, inside 151.101.0.0/16), so pinning them would allowlist the whole CDN rather than PyPI. |
| Maven Central (repo1.maven.org) | repo1.maven.org resolves into Cloudflare's published ranges (verified 2026-07-30: 104.18.18.12 and 104.18.19.12, both inside 104.16.0.0/13), so pinning it would allowlist the whole CDN rather than Maven Central. |
| RubyGems (rubygems.org) | rubygems.org resolves into Fastly's published ranges (verified 2026-07-30: 151.101.1.227 and 151.101.129.227, inside 151.101.0.0/16), so pinning it would allowlist the whole CDN rather than RubyGems. |
| crates.io | crates.io resolves into Fastly's published ranges (verified 2026-07-30: 151.101.130.137 and 151.101.194.137, inside 151.101.0.0/16), so pinning it would allowlist the whole CDN rather than crates.io. |
| Alibaba Cloud | Publishes per-service ingress lists, such as this per-region table for Data Management Service, rather than a provider-wide range file of the kind AWS, Azure and Google publish. |

Evidence links for every row: [`sources.yaml`](sources.yaml) · methodology: [`research/SOURCES.md`](research/SOURCES.md)
