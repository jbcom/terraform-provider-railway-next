# Testing

`go test ./...` is credential-free and covers framework schemas, references,
generated GraphQL requests, response normalization, deterministic change sets,
redaction, HTTP behavior, Terraform plan/state/import, bucket registration,
one-commit variable updates, ambiguous mutation reconciliation, and remote
deletion.

Live tests require `TF_ACC=1`, an explicit Railway token, and
`RAILWAY_ACC_PROJECT_PREFIX` beginning with `tfacc-`:

```text
TF_ACC=1 RAILWAY_API_TOKEN=... \
RAILWAY_ACC_PROJECT_PREFIX=tfacc-local- \
go test ./internal/provider -run '^TestAcc' -count=1 -v
```

`TestAccBucketAndPostgresLifecycle` creates a disposable project, `ams`
bucket, PostgreSQL 18 and its volume; tests an empty second plan and composite
imports; then destroys everything. It is non-parallel. GitHub acceptance is
manual, protected by an environment, and requires a billable-resource
confirmation input.
