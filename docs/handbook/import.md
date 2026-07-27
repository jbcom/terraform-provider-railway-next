# Import

| Resource | Import ID |
| --- | --- |
| `railway_project` | `project_id` |
| `railway_environment` | `environment_id` |
| `railway_service` | `project_id/environment_id/service_id` |
| `railway_volume` | `project_id/environment_id/volume_id` |
| `railway_variable_collection` | `project_id/environment_id/service_id` |
| `railway_secret` | `project_id/environment_id/service_id/name` |
| `railway_service_domain` | `kind/project_id/environment_id/service_id/domain_id` |
| `railway_bucket` | `project_id/environment_id/bucket_id` |
| `railway_postgres` | `project_id/environment_id/service_id/volume_id` |

Variable-collection import adopts every current service variable as its initial
owned set; review the first plan. PostgreSQL import requires the official
Postgres service and exact data-volume ID. A secret value cannot be recovered
from Railway, so imported secrets require matching write-only configuration.

For the complete example import project, bucket/PostgreSQL, services, volume,
variable collection, then domain. Matching configuration should yield an empty
plan after refresh.
