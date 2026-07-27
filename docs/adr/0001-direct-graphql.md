# ADR 0001: Call Railway GraphQL directly

- Status: Accepted
- Date: 2026-07-27

## Context

Terraform providers are long-running Go plugin processes. Invoking Railway's
TypeScript SDK would add Node.js, subprocess, packaging, logging, cancellation,
and secret-boundary failure modes. It would also prevent the provider from
using typed Terraform values and diagnostics end to end.

Railway documents its GraphQL v2 endpoint as the same API used by its dashboard
and exposes schema introspection.

## Decision

The provider uses Terraform Plugin Framework and a generated genqlient client
over a hardened Go HTTP transport. It does not import or execute Railway's
TypeScript SDK, the Railway CLI, scripts, provisioners, or browser automation.

The MIT-licensed `railwayapp/railway-ts-sdk` is studied as a behavioral
reference. Relevant behavior is reimplemented in Go with fixture-backed tests.
No TypeScript source is copied into the runtime.

## Consequences

- Provider releases are self-contained Go binaries.
- GraphQL operations are statically validated against a checked-in schema.
- Railway API changes require an intentional schema refresh and generated-code
  consistency check.
- Opaque environment configuration and change-set JSON requires additional
  documentation and fixtures; see ADR 0002.

