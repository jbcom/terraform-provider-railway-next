# Changelog

All notable changes follow Keep a Changelog and Semantic Versioning.

## [0.1.3] - 2026-07-27

- Wait for Railway's eventually consistent volume instance after
  `railway_volume` creation instead of returning without Terraform state when
  the volume exists but its environment attachment is not visible yet.
- Add protocol-v6 coverage for delayed volume-instance visibility and include
  an independently managed volume in the guarded parallel acceptance lifecycle.

## [0.1.2] - 2026-07-27

- Resolve every omitted `railway_service` Optional+Computed attribute to a
  known value after Create, including typed nulls for absent source, command,
  resource-limit, and collection values.
- Avoid sending an empty service-instance limits mutation when `memory_gb` and
  `vcpus` are unknown in the plan.
- Create sourced services as empty services followed by Railway's documented
  source-connect mutation, while preserving configured source values during
  Railway's post-create consistency window.
- Retry only `STALE_ENVIRONMENT_BASE` change-set rejections with a fresh
  environment ETag, rebuilt preview, and bounded backoff.
- Serialize direct service, volume, domain, secret, bucket-name, and PostgreSQL
  environment mutations with change-set commits in the provider process.
- Add protocol-v6 coverage for a GitHub service whose source is initially
  absent, stale-ETag retry coverage, and a guarded parallel Railway acceptance
  lifecycle for two services, PostgreSQL, and a bucket.

## [0.1.1] - 2026-07-27

- Clarify that this is an independent community provider with no Railway
  Corporation affiliation or endorsement.

## [0.1.0] - 2026-07-27

- Initial native Go provider implementation.
- Validate the disposable bucket/PostgreSQL create, empty-plan, import, and
  destroy lifecycle against Railway.
- Preview and normalize opaque environment change sets before apply.
- Track the bucket, PostgreSQL service, and volume IDs realized by Railway
  change sets.
- Serialize environment change-set commits and normalize Railway region IDs.
