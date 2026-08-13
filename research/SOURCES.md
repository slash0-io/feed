# Upstream IP-range sources: research and verification

*Companion machine registry: [`sources.yaml`](../sources.yaml). Prose last
reviewed 2026-08-12. Original research date 2026-07-04.*

This document records, for every service in the catalog: how the ranges are
obtained (the official source and its provenance), how often they change
(documented notice where the vendor commits to one, observed behavior
otherwise), and how changes are detected.

**The two tables below are generated from `sources.yaml`**, and a test fails
the build when they drift. Regenerate with:

```sh
go run ./generator -sources-doc research/SOURCES.md
```

They were maintained by hand until 2026-07-26 and had drifted: several vendors'
documented notice periods were missing, one vendor appeared twice, and claims
carried no links. Every claim a table makes about a named vendor now renders as
a link to the vendor page that states it.

## Methodology

1. **Official sources only.** Every entry cites the vendor's own publication
   (docs page, JSON/CSV endpoint, or API). Community aggregators were used to
   discover leads, never as a data source. The provenance chain has to
   terminate at the vendor.
2. **Live verification.** Every machine-readable endpoint in `sources.yaml` was
   fetched, and its HTTP status, payload shape, and caching headers recorded.
   Response bodies are archived as parser fixtures, which is what lets the
   build run offline in CI.
3. **Conditional-GET probing.** For every endpoint returning an `ETag` or
   `Last-Modified`, the request was replayed with
   `If-None-Match` / `If-Modified-Since` to confirm the server honors it. All
   15 probed endpoints returned `304 Not Modified` (12 on 2026-07-04, then
   Buildkite, Tenable and Elastic Cloud on 2026-07-10).
4. **Claims are cited, not inherited.** A vendor's notice period or change
   signal is recorded only from the vendor's own page, read directly. Values
   derived from earlier internal summaries proved wrong often enough that the
   build now rejects a claim submitted without its evidence URL.

## Source quality tiers

| Tier | Definition | Examples |
|---|---|---|
| **A** | Machine-readable endpoint, versioned (sync token, timestamp, or in-band changelog) | AWS, GCP, Salesforce, Atlassian, Auth0, Datadog, Oracle |
| **B** | Machine-readable endpoint, unversioned (detect by header or content hash) | Stripe, Fastly, CircleCI, Grafana Cloud, Braintree, PagerDuty, Zoom |
| **C** | Official docs or HTML page only (extract CIDRs, hash the extracted set) | Anthropic, GitLab.com, Sentry, New Relic, Postmark, Fivetran |
| **C-js** | Docs page that renders only under a browser, fetched with `render: chrome` | Duo, UptimeRobot |
| **D** | Published but gated behind auth or per-customer scoping | MongoDB Atlas control plane, Mailgun, Akamai, Confluent, Dynatrace |
| **✗** | No pinnable publication, recorded under `non_publishers` with the vendor's stated alternative | Twilio, Adyen, SendGrid, Slack, Shopify, Snowflake, Vercel, Netlify, npm, PyPI |

## Verified catalog

Direction is read from your workload's point of view. `egress` ranges are what
you connect *to*; `ingress` ranges are what the service connects *from*,
meaning webhook and agent sources.

Where a service publishes through several endpoints, it is reported at its
weakest, because the hardest source to follow sets the integration cost.
"Advance notice" counts only where the vendor documents a period covering the
ranges this feed publishes. PagerDuty is the reason that distinction is
spelled out: their 30-day notice covers REST API IPs, not the webhook ranges
published here, so it scores none.

<!-- GEN:catalog -->

| Service | Document | Change detection | Advance notice | Vendor change signal | Direction |
|---|---|---|---|---|---|
| [Airbyte Cloud](https://docs.airbyte.com/platform/operating-airbyte/ip-allowlist) | docs page | page extraction | none documented | none | ingress |
| [Akamai Connected Cloud (Linode)](https://geoip.linode.com/) | CSV | conditional GET | none documented | none | egress |
| [Amazon Web Services](https://docs.aws.amazon.com/vpc/latest/userguide/aws-ip-ranges.html) | JSON | conditional GET | none documented | [SNS arn:aws:sns:us-east-1:806199016981:AmazonIpSpaceChanged (us-east-1 subscription required)](https://docs.aws.amazon.com/vpc/latest/userguide/subscribe-notifications.html) | egress + ingress |
| [Anthropic (Claude API)](https://platform.claude.com/docs/en/api/ip-addresses) | docs page | page extraction | none documented | none | egress + ingress |
| [Atlassian Cloud](https://support.atlassian.com/organization-administration/docs/ip-addresses-and-domains-for-atlassian-cloud-products/) | JSON | conditional GET | none documented | [SNS arn:aws:sns:us-east-1:745490931007:atlassian-public-ip-changes](https://support.atlassian.com/organization-administration/docs/ip-addresses-and-domains-for-atlassian-cloud-products/) | egress + ingress |
| [Auth0 (Okta CIC)](https://auth0.com/docs/secure/security-guidance/data-security/allowlist) | JSON | conditional GET | [several months, by email](https://auth0.com/docs/secure/security-guidance/data-security/allowlist) | none | egress + ingress |
| [Braintree (PayPal)](https://developer.paypal.com/braintree/docs/reference/general/braintree-ip-addresses) | JSON | full download | none documented | none | egress |
| [Braze](https://www.braze.com/docs/user_guide/messaging/design_and_edit/personalize/connected_content/making_an_api_call) | docs page | page extraction | none documented | none | ingress |
| [Buildkite](https://buildkite.com/docs/apis/rest-api/meta) | JSON | conditional GET | [7 days, vendor says it will try](https://buildkite.com/docs/apis/rest-api/meta) | none | ingress |
| [Checkly](https://www.checklyhq.com/docs/platform/allowlisting-traffic/) | JSON | full download | none documented | none | ingress |
| [Checkout.com](https://www.checkout.com/docs/developer-resources/ip-addresses) | docs page | page extraction | none documented | none | ingress |
| [CircleCI](https://circleci.com/docs/guides/security/ip-ranges/) | JSON | full download | [30 days](https://circleci.com/docs/guides/security/ip-ranges/) | [email to customers with a job opted into the IP ranges feature](https://circleci.com/docs/guides/security/ip-ranges/) | egress + ingress |
| [Cisco Duo](https://help.duo.com/s/article/1337) | docs page | page extraction | none documented | none | egress |
| [Cloudflare](https://www.cloudflare.com/ips/) | JSON | conditional GET | none documented | none | egress + ingress |
| [Databricks](https://docs.databricks.com/aws/en/resources/ip-domain-region) | JSON | full download | [60 days before updated IPs activate](https://docs.databricks.com/aws/en/resources/ip-domain-region) | none | egress + ingress |
| [Datadog](https://docs.datadoghq.com/api/latest/ip-ranges/) | JSON | conditional GET | none documented | none | egress + ingress |
| [DigitalOcean](https://www.digitalocean.com/geo/google.csv) | CSV | full download | none documented | none | egress |
| [DocuSign](https://www.docusign.com/trust/security/esignature) | JSON | full download | none documented | none | ingress |
| [Elastic Cloud](https://www.elastic.co/docs/deploy-manage/security/elastic-cloud-static-ips) | JSON | conditional GET | [8 weeks before static IPs change](https://www.elastic.co/docs/deploy-manage/security/elastic-cloud-static-ips) | [status.elastic.co subscription (static IP changes announced >=8 weeks ahead)](https://www.elastic.co/docs/deploy-manage/security/elastic-cloud-static-ips) | egress + ingress |
| [Fastly](https://www.fastly.com/documentation/reference/api/utils/public-ip-list/) | JSON | full download | none documented | [fastlystatus.com subscription ('IP address announcement' category)](https://www.fastly.com/documentation/reference/api/utils/public-ip-list/) | egress + ingress |
| [Fivetran](https://fivetran.com/docs/using-fivetran/ips) | docs page | page extraction | none documented | none | ingress |
| [GitHub](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses) | JSON | conditional GET | none documented | none | egress + ingress |
| [GitLab.com](https://docs.gitlab.com/user/gitlab_com/) | docs page | page extraction | none documented | [gitlab.com/gitlab-org/gitlab commits atom feed for doc/user/gitlab_com/_index.md](https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/user/gitlab_com/_index.md) (docs repo) | egress + ingress |
| [Google (all services)](https://knowledge.workspace.google.com/admin/security/obtain-google-ip-address-ranges) | JSON | conditional GET | none documented | none | egress |
| [Google Cloud Platform](https://knowledge.workspace.google.com/admin/security/obtain-google-ip-address-ranges) | JSON | conditional GET | none documented | none | egress |
| [Grafana Cloud](https://grafana.com/docs/grafana-cloud/security-and-account-management/allow-list/) | text | full download | none documented | none | ingress |
| [Hightouch](https://hightouch.com/docs/security/networking) | docs page | page extraction | none documented | none | ingress |
| [HubSpot](https://developers.hubspot.com/docs/api-reference/2026-09-beta/account/ip-ranges/guide) | JSON | full download | none documented | none | ingress |
| [IBM Cloud (Classic infrastructure)](https://cloud.ibm.com/docs/infrastructure-hub?topic=infrastructure-hub-ibm-cloud-ip-ranges) | docs page | page extraction | none documented | none | egress |
| [Intercom](https://developers.intercom.com/docs/webhooks) | JSON | conditional GET | none documented | none | egress + ingress |
| [Klaviyo](https://help.klaviyo.com/hc/en-us/articles/19143781289115) | JSON | full download | none documented | none | ingress |
| [LaunchDarkly](https://launchdarkly.com/docs/home/infrastructure/ip-list) | JSON | full download | [2 weeks, posted on the status page](https://launchdarkly.com/docs/home/infrastructure/ip-list) | [status.launchdarkly.com posts IP list changes ahead of the change](https://launchdarkly.com/docs/home/infrastructure/ip-list) | egress + ingress |
| [Microsoft 365](https://learn.microsoft.com/en-us/microsoft-365/enterprise/microsoft-365-ip-web-service) | JSON | full download | [30 days before new endpoints are used](https://learn.microsoft.com/en-us/microsoft-365/enterprise/microsoft-365-ip-web-service?view=o365-worldwide) | [poll https://endpoints.office.com/version (tiny response; version string bumps on change)](https://learn.microsoft.com/en-us/microsoft-365/enterprise/microsoft-365-ip-web-service?view=o365-worldwide) | egress |
| [Microsoft Azure (Service Tags)](https://learn.microsoft.com/en-us/azure/virtual-network/service-tags-overview) | JSON | page extraction | [1 week before new IPs are used](https://learn.microsoft.com/en-us/azure/virtual-network/service-tags-overview) | none | egress |
| [Neon (Serverless Postgres)](https://neon.com/docs/introduction/regions) | docs page | page extraction | none documented | [github.com/neondatabase/website commits feed for content/docs/introduction/regions.md](https://github.com/neondatabase/website/blob/main/content/docs/introduction/regions.md) (docs repo) | ingress |
| [Netskope (NewEdge)](https://docs.netskope.com/en/newedge-ip-ranges-for-allowlisting) | docs page | page extraction | none documented | none | egress + ingress |
| [New Relic](https://docs.newrelic.com/docs/new-relic-solutions/get-started/networks/) | docs page | page extraction | none documented | [github.com/newrelic/docs-website commits feed for networks.mdx](https://github.com/newrelic/docs-website/blob/develop/src/content/docs/new-relic-solutions/get-started/networks.mdx) (docs repo) | egress + ingress |
| [Okta](https://help.okta.com/en-us/content/topics/security/ip-address-allow-listing.htm) | JSON | conditional GET | none documented | none | egress + ingress |
| [OpenAI (ChatGPT egress)](https://developers.openai.com/api/docs/guides/ip-addresses) | JSON | full download | none documented | none | ingress |
| [Oracle Cloud Infrastructure](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/addressranges.htm) | JSON | conditional GET | none documented | none | egress |
| [Oracle NetSuite Connector](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_162859452018.html) | docs page | page extraction | none documented | none | ingress |
| [PagerDuty](https://support.pagerduty.com/main/docs/safelist-ips) | JSON | conditional GET | none documented | none | ingress |
| [PayPal](https://www.paypal.com/us/cshelp/article/what-are-the-internet-protocol-ip-addresses-for-paypal-server-endpoints-ts1056) | docs page | page extraction | none documented | none | egress + ingress |
| [Plaid](https://plaid.com/docs/api/webhooks/) | docs page | page extraction | none documented | none | ingress |
| [Postmark](https://postmarkapp.com/support/article/800-ips-for-firewalls) | docs page | page extraction | none documented | none | egress + ingress |
| [Rapid7 (InsightAppSec cloud engines)](https://docs.rapid7.com/insightappsec/allowlist-cloud-engine-ips/) | docs page | page extraction | none documented | none | ingress |
| [Salesforce](https://help.salesforce.com/s/articleView?id=000384438 (IP Addresses and Domains to Allow)) | JSON | conditional GET | none documented | none | egress + ingress |
| [Sentry (hosted)](https://docs.sentry.io/security-legal-pii/security/ip-ranges/) | docs page | page extraction | none documented | [github.com/getsentry/sentry-docs commits feed for the ip-ranges page source](https://github.com/getsentry/sentry-docs/blob/master/docs/security-legal-pii/security/ip-ranges.mdx) (docs repo) | egress + ingress |
| [Splunk AppDynamics SaaS](https://help.splunk.com/en/appdynamics-saas/product-announcements-and-alerts/saas-domains-and-ip-ranges) | docs page | page extraction | none documented | none | ingress |
| [Stripe](https://docs.stripe.com/ips) | JSON | full download | [7 days, by mailing list](https://docs.stripe.com/ips) | [stripe.com/ips mailing list (seven days notice before changes)](https://docs.stripe.com/ips) | egress + ingress |
| [Tenable (Vulnerability Management)](https://docs.tenable.com/vulnerability-management/Content/Settings/Sensors/CloudSensors.htm) | JSON | conditional GET | none documented | none | ingress |
| [Twilio Elastic SIP Trunking](https://www.twilio.com/docs/sip-trunking/ip-addresses) | docs page | page extraction | none documented | none | egress + ingress |
| [UptimeRobot](https://uptimerobot.com/help/locations/) | docs page | page extraction | none documented | none | ingress |
| [Vultr (Constant, AS20473)](https://geofeed.constant.com/) | CSV | full download | none documented | none | egress |
| [Wise](https://docs.wise.com/api-docs/features/webhooks-notifications) | docs page | page extraction | none documented | none | ingress |
| [WorkOS](https://workos.com/docs/events/data-syncing/webhooks) | docs page | page extraction | none documented | none | ingress |
| [Zendesk](https://developer.zendesk.com/api-reference/ticketing/account-configuration/public_ips/) | JSON | full download | none documented | none | egress + ingress |
| [Zoom](https://support.zoom.com/hc/en/article?id=zm_kb&sysparm_article=KB0060548) | text | conditional GET | none documented | none | egress |
| [Zscaler (zscaler.net cloud)](https://config.zscaler.com/) | JSON | conditional GET | none documented | none | egress |
| [dbt Cloud](https://docs.getdbt.com/docs/cloud/about-cloud/access-regions-ip-addresses) | docs page | page extraction | none documented | none | ingress |

<!-- /GEN:catalog -->

## Change detection

Four layers, cheapest first. Every source gets layer 2 or 3 as a floor; layers
1 and 4 are added where the vendor offers them.

1. **Push subscriptions**, where the vendor operates one. AWS publishes the SNS
   topic `AmazonIpSpaceChanged`, whose payload carries the new `syncToken`.
   Fastly announces on its status page. Microsoft 365 exposes
   `endpoints.office.com/version`, a small purpose-built endpoint that can be
   polled cheaply so the full payload is fetched only on a version bump.
   Vendor notice emails route into a review queue rather than into automation.
2. **Conditional-GET polling** for endpoints with verified `ETag` or
   `Last-Modified` support. A `304` costs roughly 200 bytes, so this tier is
   cheap enough to poll far more often than it currently is.
3. **Content-hash polling** for endpoints with no cache headers. Fetch,
   canonicalize by parsing, sorting and deduplicating, then hash the
   *normalized range set*. Hashing normalized output rather than raw bytes
   avoids false positives from key reordering and whitespace.
4. **Docs-page watching** for tier C. Fetch the page, extract CIDRs with a
   per-source parser, and hash the extracted set, never the raw HTML, since nav
   and footer churn would false-positive. Where the docs live in a public repo
   (New Relic, Sentry, GitLab), the specific file's commit Atom feed is watched
   as well, which gives a readable diff and a commit message explaining the
   change.

**What runs today**: a single scheduled GitHub Actions job on a `*/10` cron
polls every source on the same schedule. GitHub's shared scheduler frequently
defers it, so real intervals are longer than ten minutes and occasionally
exceed an hour. The per-tier cadences above describe the design the layers were
built for, not the current schedule. Moving the conditional-GET tier onto a
long-lived poller is the change that would close that gap, since a GitHub
Actions cron cannot go below roughly five minutes.

**Publication pipeline on detection**: fetch, parse, normalize, validate,
version-bump, publish, notify.

Validation gates:

- Reject impossible entries: `0.0.0.0/0`, RFC 1918 and RFC 4193 space,
  malformed CIDRs, and overlaps collapsing to empty.
- **Removal guardrail.** A change removing more than half of a purpose's ranges
  is quarantined for review instead of published, and the guardrail applies
  only where the purpose previously had at least 8 ranges, so small fixed lists
  do not trip it. The last-good version keeps serving while it is held. A
  vendor endpoint returning a truncated or error body must never propagate.
- **Aggregation is lossless.** Published coverage is preserved exactly and
  never widened, so an aggregated entry can never grant more than the vendor
  published.
- Every publish is versioned, with a changelog entry recording the source URL,
  the upstream token or hash, and the retrieval timestamp.

Grace windows for removals are implemented in the hosted tier rather than in
the feed: a range that disappears upstream is marked and held for 72 hours
before it is withdrawn from a customer's prefix list, which mirrors the
patterns Azure and Databricks document. The feed itself reports removals as
soon as the vendor makes them.

## Services that do not publish pinnable ranges

Verified vendor positions, published in the feed as `pinnable: false` alongside
the vendor's recommended alternative. A recorded negative is a claim about a
named vendor, so each row links the page stating it.

<!-- GEN:nonpublishers -->

| Service | Vendor's stated position |
|---|---|
| [Adyen](https://help.adyen.com/knowledge/ecommerce-integrations/webhooks/what-ip-addresses-does-adyen-use-to-send-webhook-events) | No IP list; allowlist out.adyen.com or resolve it via DNS hourly (their words). |
| [Aiven](https://aiven.io/docs/platform/concepts/static-ips) | Services are addressed by hostname; static IP addresses are a paid per-project resource created and attached with the avn static-ip CLI, not a public range list. |
| [Akamai (CDN)](https://techdocs.akamai.com/site-shield/docs/welcome-site-shield) | Site Shield issues a per-customer set of IP subnet ranges (a map) retrieved through Akamai Control Center or the Site Shield API, rather than one public global list. |
| [Alibaba Cloud](https://www.alibabacloud.com/help/en/dms/configure-an-ip-address-whitelist) | Publishes per-service ingress lists, such as this per-region table for Data Management Service, rather than a provider-wide range file of the kind AWS, Azure and Google publish. |
| [Box](https://support.box.com/hc/en-us/articles/360043696434-Configuring-A-Firewall-For-Box-Applications-and-Services) | Use domain names; 'IP addresses can change frequently and without notice' (their wording). No webhook source ranges published. |
| [CockroachDB Cloud](https://www.cockroachlabs.com/docs/cockroachcloud/network-authorization) | The documented controls are an allowlist of your own authorized networks and private connectivity through AWS PrivateLink, GCP Private Service Connect or Azure Private Link. |
| [Confluent Cloud](https://docs.confluent.io/cloud/current/networking/static-egress-ip-addresses.html) | Public egress addresses are read from the Cloud Console or the authenticated Cloud REST API (api.confluent.cloud/networking/v1/ip-addresses), are shared by every customer in the same cloud and region, and are 'not guaranteed to be static' (their wording). |
| [Docker Hub / Docker Desktop](https://docs.docker.com/desktop/setup/allow-list/) | Publishes an allowlist of domain URLs rather than addresses; the page lists hostnames only. Reproducible on the data plane: registry-1.docker.io resolves into rotating AWS us-east-1 EC2 addresses, a different set on each query (verified 2026-07-30). |
| [Dynatrace (Synthetic Monitoring)](https://docs.dynatrace.com/docs/observe/digital-experience/synthetic-monitoring/general-information/public-synthetic-locations) | Public Synthetic location addresses are read per environment, either from the Frequency and locations page in the web UI ('Copy IPs to clipboard or Download IPs') or from the Synthetic locations API, which 'returns all the locations available for your Environment along with their IP addresses'. No global list is published. |
| [Honeycomb](https://docs.honeycomb.io/security-compliance/security-data-protection/) | Offers AWS PrivateLink to the Honeycomb API for Enterprise customers on AWS. No range list is published in the docs. |
| [Mailchimp Transactional (Mandrill) webhooks](https://mailchimp.com/developer/transactional/docs/webhooks/) | Directs users to authenticate that a webhook originated from Mailchimp's servers using the documented request-signature flow. The /ips/ API returns your own dedicated sending addresses, which is a different thing from webhook sources. |
| [Mailgun](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/ip-allowlist) | Their IP Allowlist API 'lets you view and manage allowlisted IP addresses to which API key and SMTP credential usage is restricted', which controls your own callers rather than publishing Mailgun's addresses. Outbound sending addresses are per-account dedicated IPs grouped into pools, read through the authenticated /v3/ips API. |
| [Maven Central (repo1.maven.org)](https://www.cloudflare.com/ips/) | repo1.maven.org resolves into Cloudflare's published ranges (verified 2026-07-30: 104.18.18.12 and 104.18.19.12, both inside 104.16.0.0/13), so pinning it would allowlist the whole CDN rather than Maven Central. |
| [MongoDB Atlas (data plane)](https://www.mongodb.com/docs/atlas/reference/faq/networking/) | Cluster IPs are per-project/dynamic; control-plane IPs only via authenticated Admin API. Vendor directs users to private endpoints (PrivateLink). |
| [Netlify (function egress)](https://docs.netlify.com/manage/security/private-connectivity/) | States that by default the addresses builds and functions connect from 'will fluctuate when we scale up and down'. A static set is available only through the Private Connectivity add-on on Enterprise plans. |
| [OpenAI API (api.openai.com)](https://www.cloudflare.com/ips/) | api.openai.com resolves into Cloudflare's published ranges (verified 2026-07-30: 172.66.0.243 within 172.64.0.0/13, 162.159.140.245 within 162.158.0.0/15), so pinning it would allowlist the whole CDN rather than OpenAI. |
| [Oracle NetSuite (platform)](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_N251955.html) | States plainly that Oracle 'does not support the use of NetSuite IP addresses to access or manage access to any NetSuite services', that outbound addresses are 'not documented in the NetSuite Help Center or in SuiteAnswers', and that *.netsuite.com is CDN-fronted. Directs users to 2FA, token-based auth and OAuth 2.0 instead, or to a DNS lookup on outboundips.netsuite.com. NetSuite Connector IS published, see the netsuite-connector service. |
| [Palo Alto Networks Prisma Access](https://docs.paloaltonetworks.com/prisma-access/administration/prisma-access-setup/retrieve-ip-addresses-for-prisma-access) | Egress addresses are allocated per tenant and retrieved with your own API key from api.prod.datapath.prismaaccess.com, or read per location in the Prisma Access UI. |
| [PlanetScale](https://planetscale.com/docs/imports/import-tool-migration-addresses) | The addresses to allowlist are shown in the console during the import workflow, differ by region, and the vendor directs users to read them there each time because they 'can change occasionally'. |
| [PyPI (pypi.org, files.pythonhosted.org)](https://api.fastly.com/public-ip-list) | Both pypi.org and files.pythonhosted.org resolve into Fastly's published ranges (verified 2026-07-30: 151.101.0.223 and 151.101.128.223, inside 151.101.0.0/16), so pinning them would allowlist the whole CDN rather than PyPI. |
| [Redis Cloud](https://redis.io/docs/latest/operate/rc/security/cidr-whitelist/) | The CIDR allow list restricts which of your own addresses may reach your database, between 4 and 32 entries depending on plan. Redis publishes no ranges of its own. |
| [RubyGems (rubygems.org)](https://api.fastly.com/public-ip-list) | rubygems.org resolves into Fastly's published ranges (verified 2026-07-30: 151.101.1.227 and 151.101.129.227, inside 151.101.0.0/16), so pinning it would allowlist the whole CDN rather than RubyGems. |
| [SendGrid (webhooks/parse)](https://support.sendgrid.com/hc/en-us/articles/44375457225371) | Dynamic cloud infra; use signed webhooks, not IP allowlists. |
| [Shopify (webhooks)](https://shopify.dev/docs/apps/build/webhooks/verify-deliveries) | Documents HMAC-SHA256 signature verification as the way to authenticate a webhook. The page does not mention source IPs, and no official range list was found as of the verified date. |
| [Slack](https://docs.slack.dev/concepts/security/) | No published egress IPs; their allowlisting feature restricts YOUR IPs calling THEM. |
| [Snowflake](https://docs.snowflake.com/en/sql-reference/functions/system_allowlist) | Deployment-specific hostnames/IPs per account; no global list. |
| [Splunk Cloud Platform](https://help.splunk.com/en?resourceId=SplunkCloud_Config_ConfigureIPAllowList) | The documented control is an IP allow list restricting which addresses on your own network reach each Splunk feature, managed through the Admin Config Service API. Splunk publishes no ranges of its own for the stack. |
| [Square (webhooks)](https://developer.squareup.com/docs/webhooks/step3validate) | No webhook source-IP list published; validate notifications via the documented HMAC-SHA256 signature flow. |
| [Sumo Logic](https://help.sumologic.com/docs/api/getting-started/) | No own ranges; directs users to download the AWS IP ranges JSON and use the prefixes for the AWS region their deployment sits in, which the aws service already covers. States plainly that 'the list of IP ranges is shared infrastructure. It is not limited to Sumo Logic nodes and is subject to change over time.' |
| [Supabase](https://supabase.com/docs/guides/platform/ipv4-address) | States that 'IPv4 addresses are guaranteed to be static for ingress traffic' through a per-project paid add-on, while 'the outbound IP address is not static and cannot be guaranteed'. |
| [Twilio (REST API + webhooks)](https://help.twilio.com/articles/115015934048) | IPs 'highly dynamic, and span a large range, so it's impractical to list each of them'; they recommend allowing outbound HTTPS to any *.twilio.com subdomain instead. SIP trunking IS pinnable, see the twilio-sip service. |
| [Vercel (function egress)](https://vercel.com/docs/networking/static-ips) | Dynamic by default; static IPs are a paid per-customer feature, not a public range. |
| [crates.io](https://api.fastly.com/public-ip-list) | crates.io resolves into Fastly's published ranges (verified 2026-07-30: 151.101.130.137 and 151.101.194.137, inside 151.101.0.0/16), so pinning it would allowlist the whole CDN rather than crates.io. |
| [npm registry (registry.npmjs.org)](https://www.cloudflare.com/ips/) | registry.npmjs.org resolves into Cloudflare's published ranges (verified 2026-07-30: 104.16.0.34 and 104.16.1.34, both inside 104.16.0.0/13), so pinning it would allowlist the whole CDN rather than npm. |

<!-- /GEN:nonpublishers -->

## Notable findings

- **Salesforce publishes an AWS-style `ip-ranges.json`** at
  `ip-ranges.salesforce.com`, with the same `syncToken`, `createDate` and
  `prefixes` top level, except that `ip_prefix` values are *arrays* of CIDRs
  rather than scalars. **Tenable** (`docs.tenable.com/ip-ranges/data.json`) is
  a second, schema-exact clone that the AWS parser consumes unmodified. The AWS
  schema is becoming a de-facto standard, though even its imitators drift,
  which is why every source gets its own fixture-tested parser.
- **Direction matters and almost nobody models it.** Roughly a third of
  published ranges describe the vendor's egress toward customers, which belongs
  in ingress rules and is useless in an egress allowlist. Every purpose in this
  schema carries a `direction`. Atlassian is the only upstream source that does
  this consistently.
- **Anthropic is the only major AI API with pinnable inbound ranges**, using
  dedicated space and stating the ranges will not change without notice. OpenAI
  publishes only its own egress; its API is CDN-fronted.
- **Grace windows are an emerging vendor practice**, with Azure documenting at
  least a week and Databricks 60 days.
- **Open-source docs repos are a change-detection asset.** For New Relic,
  Sentry and GitLab the authoritative page is a file in a public git repo, so a
  commit feed gives near-instant notice with a readable diff, for tier-C
  sources that would otherwise only be scrapable.

## Backlog

`sources.yaml → backlog` groups what is left by the reason it is unresolved,
because those reasons call for different work:

- `blocked`: the vendor publishes a real list and the document is unreachable,
  usually behind a customer login or a docs portal that renders nothing without
  JavaScript. These are never recorded as non-publishers, which would be a false
  claim about the vendor.
- `no_source_found`: researched, nothing public located, and no vendor page
  stating a position that could be cited. That missing citation is precisely why
  they are not non-publishers either.
- `non_publisher_candidates`: a real source exists but is per-customer or
  authenticated. Each becomes a non-publisher as soon as someone fetches an
  evidence URL for it.
- `declined`: product decisions, recorded so they are not researched again.

Generator features that this research surfaced are tracked in the README
roadmap rather than here, so that they get scheduled as engineering work.

Resolved in earlier passes: Zendesk, PayPal, Vultr and DocuSign integrated as
publishers; Box and Sumo Logic documented as non-publishers; Segment, Amplitude
and Mixpanel found to have no public range documentation; Scaleway, OVH and
Hetzner found to expose no geofeeds at standard URLs.

A second pass added Buildkite, Tenable, Rapid7, Elastic Cloud, Neon, Klaviyo
and Twilio SIP Trunking as publishers. A third added Plaid, Netskope (NewEdge)
and IBM Cloud Classic, and documented Square as a non-publisher.

Several dead ends were recorded as "client-rendered page, unusable" and are
worth revisiting, because that diagnosis has since proved unreliable. Headless
Chrome with a virtual time budget renders pages that previously failed, and at
least one vendor (IBM Cloud) serves a full server-rendered page to a
non-browser User-Agent while returning a bare app shell to a browser. Qualys,
Iterable, RingCentral and Braze all fall in this category.

IBM's IP-ranges page silently moved from `docs/cloud-infrastructure` to
`docs/infrastructure-hub`, with the old URL returning 410. Docs-page sources
need URL-health monitoring, not just content hashing.

## Verification artifacts

Fetched bodies and response headers are archived from each research session and
become the initial parser test fixtures, which is what allows
`go run ./generator -fixtures testdata/fixtures` to build the whole feed with
no network access. Each `sources.yaml` entry carries a `verified` date, and a
source that fails to fetch is held at its last-good version rather than
published empty.
