# Local development override

Build with `go build -o terraform-provider-railway-next`, then configure:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/micah5/railway-next" = "/absolute/path/to/terraform-provider-railway-next"
  }
  direct {}
}
```

The Terraform configuration should declare local provider name `railway` for
source `micah5/railway-next`, retaining `railway_*` resource names.
