# Changelog

All notable changes follow Keep a Changelog and Semantic Versioning.

## [Unreleased]

## [0.1.0] - 2026-07-27

- Initial native Go provider implementation.
- Validate the disposable bucket/PostgreSQL create, empty-plan, import, and
  destroy lifecycle against Railway.
- Preview and normalize opaque environment change sets before apply.
- Track the bucket, PostgreSQL service, and volume IDs realized by Railway
  change sets.
- Serialize environment change-set commits and normalize Railway region IDs.
