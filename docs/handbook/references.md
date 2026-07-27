# Railway reference variables

Use computed references:

```hcl
variables = {
  DATABASE_URL             = railway_postgres.main.references["DATABASE_URL"]
  BUCKET_SECRET_ACCESS_KEY = railway_bucket.cache.references["SECRET_ACCESS_KEY"]
}
```

The resulting `${{Postgres.DATABASE_URL}}`-style values are sent unrendered;
Railway resolves them at runtime and Terraform never receives the credential.
When writing a literal HCL string, use
`"$${{Postgres.DATABASE_URL}}"` so Terraform produces the exact Railway
expression.
