# Authentication

Token lookup order is `token`, `RAILWAY_API_TOKEN`, then `RAILWAY_TOKEN`.
Endpoint lookup order is `graphql_endpoint`, `RAILWAY_GRAPHQL_ENDPOINT`, then
`https://backboard.railway.com/graphql/v2`.

`account` and `workspace` tokens use `Authorization: Bearer`; `project` tokens
use `Project-Access-Token`. Since project tokens are opaque, select
`token_type = "project"` explicitly. Only HTTPS endpoints are accepted except
for loopback HTTP fixture servers.

Tokens, authorization headers, and raw GraphQL variables are never logged.
Diagnostics preserve Railway trace/request IDs where available.
