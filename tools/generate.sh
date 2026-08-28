#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Regenerate docs/ from the provider's own schemas.
#
# **WHY THIS IS A SCRIPT AND NOT A BARE `go:generate` LINE.**
#
# `tfplugindocs` builds the provider and then runs `terraform init` against it,
# which asks the public registry for `hashicorp/<provider-name>`. This provider
# lives at `micah5/railway-next`, so that lookup asked for a provider that does
# not exist and failed with:
#
#   Provider registry.terraform.io/hashicorp/railway v0.0.1 does not have a
#   package available for your current platform, darwin_arm64.
#
# which reads like a platform problem and is not one. There is no namespace
# flag; `--provider-name` is assumed to sit under `hashicorp/`.
#
# So the schema is exported HERE, from the locally built binary via
# `dev_overrides`, and handed to `tfplugindocs` with `--providers-schema` —
# skipping the registry entirely. It also means docs are generated from the
# provider in the working tree rather than from whatever is published.
#
# The re-key is the other half: `--provider-name` names BOTH the schema key and
# the resource-name prefix. The export is keyed by the full registry address
# (`registry.terraform.io/micah5/railway-next`) while resources are prefixed
# `railway_`, so the schema is re-keyed to `railway` and one value serves both.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "building the provider under test"
go -C "$root" build -o "$work/terraform-provider-railway-next" .

cat > "$work/.terraformrc" <<EOF
provider_installation {
  dev_overrides {
    "micah5/railway-next" = "$work"
  }
  direct {}
}
EOF

cat > "$work/main.tf" <<'EOF'
terraform {
  required_providers {
    railway = {
      source = "micah5/railway-next"
    }
  }
}
EOF

echo "exporting the provider schema"
(cd "$work" && TF_CLI_CONFIG_FILE="$work/.terraformrc" terraform providers schema -json > schema.json)

# See the header: one flag names both the schema key and the resource prefix.
node -e '
  const fs = require("fs");
  const path = process.argv[1];
  const schema = JSON.parse(fs.readFileSync(path, "utf8"));
  const exported = Object.values(schema.provider_schemas || {});
  if (exported.length !== 1) {
    throw new Error(`expected exactly one provider schema, got ${exported.length}`);
  }
  schema.provider_schemas = { railway: exported[0] };
  fs.writeFileSync(path, JSON.stringify(schema));
' "$work/schema.json"

echo "rendering docs"
go -C "$root" run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate \
  --provider-dir "$root" \
  --provider-name railway \
  --rendered-provider-name Railway \
  --providers-schema "$work/schema.json"

echo "docs regenerated"
