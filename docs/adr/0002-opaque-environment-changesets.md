# ADR 0002: Railway opaque environment change sets

- Status: Accepted with constraints
- Date: 2026-07-27

## Verified source behavior

The July 22, 2026 Railway TypeScript SDK (`v3.6.0`, commit
`dd3a11ea140059149079a91045efc454b21cdbc4`) treats environment configuration
and change-set payloads as opaque GraphQL scalars. It:

- reads `environment.config(decryptVariables: false)` and `configEtag`;
- computes an intent-level change set from the current and desired graphs;
- calls `environmentPreviewChangeSet` before apply and uses Railway's
  normalized returned change set;
- lets the applied change set realize services, volumes, and buckets;
- represents bucket registration under `config.buckets[bucketID]`;
- applies one `environmentApplyChangeSet` mutation with `baseConfigEtag`;
- treats a stale base as a plan-invalidating error;
- represents removed objects with `isDeleted: true`;
- preserves secret values that were not decrypted;
- treats bucket region as immutable.

## Provider rule

Opaque payloads are produced only by `internal/changeset`. Each supported shape
has a JSON fixture and deterministic normalization test. Resources must not
construct these JSON objects ad hoc.

Before apply, the provider sends each payload through the typed
`PreviewEnvironmentChangeSet` operation and applies the normalized change set
returned by Railway. Bucket and PostgreSQL resources then discover and persist
the IDs Railway realized, rather than pre-creating detached low-level objects.

For the first release, change sets are used only where they are necessary for
correct environment registration or atomic deployment behavior:

- bucket registration/removal;
- composite PostgreSQL registration;
- atomic variable collection updates;
- service-instance configuration batches.

Every apply includes the last-read `configEtag` when available. A stale-base
response is never retried automatically. A timeout or transport interruption
after a mutation is reconciled by a fresh read.

## Known boundary

Railway does not publish a stable SDL contract for the internal JSON layout.
The checked-in fixtures therefore pin behavior inferred from the official SDK
and current introspection. Schema-refresh pull requests must compare these
fixtures with the current SDK before generated code is updated.
