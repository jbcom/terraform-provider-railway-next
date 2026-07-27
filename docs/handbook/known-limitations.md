# Known Railway API limitations and assumptions

- Live acceptance tests are opt-in. The disposable bucket/PostgreSQL lifecycle
  suite last passed on July 27, 2026; broader live coverage remains on the
  roadmap.
- Environment configuration/change sets are opaque JSON pinned to current
  introspection and Railway TypeScript SDK v3.6.0 fixtures.
- Current GraphQL schema has no public `bucketDelete`; deletion is an
  environment change followed by Railway-controlled delayed cleanup.
- Secret drift proves name presence, not plaintext equality.
- Service creation is distinct from deployment success; consult
  `railway_deployment_status`.
- PostgreSQL import assumes
  `ghcr.io/railwayapp-templates/postgres-ssl:<version>` and one
  `/var/lib/postgresql/data` volume.
- Project-token capabilities remain bounded by Railway permissions.
