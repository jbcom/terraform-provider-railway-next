# A `railway_tailscale` resource

## Why this is worth adding

**Railway deprecated its official Tailscale subnet-router template, and the
reason is structural rather than incidental.** The template advertised
Railway's entire private range — `fd12::/16` — as a single subnet route. Every
Railway environment uses that same range, so a tailnet could contain exactly
one of them. A second environment's router would collide with the first, and
the winner was arbitrary.

That is not a bug anybody can fix inside the template. It is what advertising a
shared range means.

## What works instead

**Do not advertise a route. Terminate the connection and forward from inside.**

A Tailscale node placed in a Railway environment can reach
`<service>.railway.internal` because it IS in that environment. So rather than
telling the tailnet "route `fd12::/16` through me", the node accepts a
connection on its own MagicDNS name and proxies it to the private hostname.
Nothing outside ever needs a route into `fd12::/16`, so two environments cannot
conflict — and `staging-postgres` and `production-postgres` can coexist in one tailnet.

The mechanism is `TS_SERVE_CONFIG`, which the official image already supports:

```json
{ "TCP": { "5432": { "TCPForward": "production-postgres.railway.internal:5432" } } }
```

```json
{ "TCP": { "443": { "HTTPS": true } },
  "Web": { "${TS_CERT_DOMAIN}:443": {
    "Handlers": { "/": { "Proxy": "http://prod-web.railway.internal:8080" } } } } }
```

**Verified in production use**: an operator runs `psql production-postgres` from a
laptop on the tailnet, against a database with no public route in any
environment.

## Why it needs a resource rather than a doc

Everything above can be done today with `railway_service` plus variables, and
that is how the author of this proposal is doing it. It is about thirty lines of
configuration per environment, and every one of them is a chance to get a
detail wrong:

- **`TS_SERVE_CONFIG` names a path INSIDE the image.** The stock
  `tailscale/tailscale` image has no `/config`, so a node configured this way
  joins the tailnet and proxies nothing — it appears healthy, resolves over
  MagicDNS, and refuses every connection.
- **The serve config cannot interpolate.** Only `${TS_CERT_DOMAIN}` is
  substituted, so the upstream hostname must be literal — which means a
  separate file per environment, baked into the image.
- **`TS_STATE_DIR` needs a volume**, or the node re-authenticates on every
  restart, burning a pre-auth key each time and leaving stale machines behind.
- **`--advertise-routes` must NOT be set**, which is the whole point and the
  opposite of what every subnet-router tutorial says.
- **Self-hosted headscale needs `--login-server`**, without which the client
  registers against `controlplane.tailscale.com` and fails with an auth error
  that reads like a bad key rather than the wrong server.

A resource encodes all five. A tutorial gets three of them right.

## Proposed shape

```hcl
resource "railway_tailscale" "postgres" {
  project_id     = railway_project.this.id
  environment_id = railway_environment.uat.id

  name     = "staging-postgres"          # the MagicDNS name an operator types
  auth_key = var.tailscale_auth_key  # sensitive; from a secret store

  # WHAT THIS NODE FRONTS. Exactly one upstream — a node per service rather
  # than one per environment, so the address says what it reaches.
  upstream {
    host   = "staging-postgres.railway.internal"
    port   = 5432
    scheme = "tcp"                   # "tcp" | "http"
  }

  # Self-hosted control server. Omit for Tailscale's SaaS.
  login_server = "https://headscale.example.com"

  # Optional: accept the tailnet's advertised routes. Off by default, because a
  # Railway node reaches its own environment natively and has no reason to pull
  # somebody else's LAN into its routing table.
  accept_routes = false
}
```

The provider would own the image (stock `tailscale/tailscale` at a pinned
version), generate the serve configuration from `upstream`, attach a volume for
`TS_STATE_DIR`, and set `TS_EXTRA_ARGS` correctly — including refusing to set
`--advertise-routes`, which is the mistake this resource exists to prevent.

`auth_key` is `Sensitive` and never returned.

## Open questions

- **Should `upstream` be a list?** One node per service is the recommendation
  because the address then says what it reaches. But a single node fronting
  several ports is legitimate for a small environment, and the serve config
  supports it.
- **Pinning the image version.** `:stable` is a moving tag, and a VPN gateway
  that changes under a running environment is the failure this provider's own
  `known-limitations.md` philosophy would reject. A pinned default with an
  override seems right.
- **Ephemeral nodes.** Attractive for a node that is genuinely disposable, but
  it removes the machine when it goes offline — so a restarting container
  churns the node list. Probably an opt-in flag, defaulting off.
