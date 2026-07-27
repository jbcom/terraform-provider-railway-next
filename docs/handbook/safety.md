# Lifecycle and data safety

Destroying `railway_project` deletes all nested infrastructure. Destroying
`railway_volume` or `railway_postgres` permanently deletes persistent data.
Use Terraform `prevent_destroy` where appropriate.

Bucket region is immutable and forces replacement. Railway removes a bucket
from environment configuration first and may delay permanent deletion, so a
same-name recreation can remain unavailable temporarily.

Terraform's `sensitive` flag does not keep data out of state.
`railway_variable_collection` values remain in state. For credentials use
`railway_secret.value_wo`, a Terraform 1.11+ write-only attribute. It is never
returned from Read; increment `value_wo_version` for rotation.

Safe reads use bounded retry with jitter and `Retry-After`. Mutations are not
blindly retried. Ambiguous failures use read-after-error reconciliation where
the requested result can be identified safely.
