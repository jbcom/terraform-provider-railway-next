# Complete example

This declares one disposable Railway project with an API service, UI service,
PostgreSQL 18, storage bucket, API volume, atomic API variables, and a public
Railway domain.

Set `RAILWAY_API_TOKEN` (or `RAILWAY_TOKEN`) and provide
`github_repository`. Review the destroy plan: the project, bucket, API volume,
and PostgreSQL data are deleted.

Reference map values are runtime Railway expressions such as
`${{Postgres.DATABASE_URL}}`. Terraform displays the HCL source spelling as
`$${{Postgres.DATABASE_URL}}` when an expression is written literally; using
the computed `references` maps avoids additional escaping.

After apply, `terraform plan` must report no changes. See
`docs/handbook/import.md` for fresh-state import formats and ordering.
