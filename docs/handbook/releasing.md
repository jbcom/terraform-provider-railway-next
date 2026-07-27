# Release process

1. Update `CHANGELOG.md`, generated code, and generated documentation.
2. Run tests, vet, Go/Terraform formatting, and a GoReleaser snapshot.
3. Run guarded acceptance tests in a disposable Railway workspace.
4. Tag `vX.Y.Z`.
5. The release workflow signs checksums and publishes Registry-compatible ZIP
   archives for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64.

The release environment requires `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`.
Before changing the namespace, search source, examples, docs,
manifest/release configuration, and `main.go` for the provider address.
