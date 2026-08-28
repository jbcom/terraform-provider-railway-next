// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestResolveUnknownsLeavesKnownValuesAlone is the property that matters most.
//
// Nulling every Optional+Computed attribute would also make the apply result
// valid, and it would be wrong: it discards what the practitioner configured. A
// partial failure should cost them an error message, not their configuration.
func TestResolveUnknownsLeavesKnownValuesAlone(t *testing.T) {
	t.Parallel()

	model := serviceModel{
		ID:         types.StringValue("service-fixture"),
		Name:       types.StringValue("api"),
		Repository: types.StringValue("owner/repository"),
		Branch:     types.StringValue("master"),
		// Not yet resolved, because the refresh never ran.
		Builder:      types.StringUnknown(),
		BuildCommand: types.StringUnknown(),
	}

	ResolveUnknowns(&model)

	for name, got := range map[string]types.String{
		"id":         model.ID,
		"name":       model.Name,
		"repository": model.Repository,
		"branch":     model.Branch,
	} {
		if got.IsNull() || got.IsUnknown() {
			t.Errorf("%s = %#v, want the configured value preserved", name, got)
		}
	}

	for name, got := range map[string]types.String{
		"builder":       model.Builder,
		"build_command": model.BuildCommand,
	} {
		if !got.IsNull() || got.IsUnknown() {
			t.Errorf("%s = %#v, want known null", name, got)
		}
	}
}

// TestResolveUnknownsHandlesEveryAttributeKind covers the collection types,
// where the null needs the ELEMENT type rather than a bare container — the
// detail the hand-written version got wrong by typing a Set as a List.
func TestResolveUnknownsHandlesEveryAttributeKind(t *testing.T) {
	t.Parallel()

	model := serviceModel{
		HealthcheckTimeout: types.Int64Unknown(),
		MemoryGB:           types.Float64Unknown(),
		SleepApplication:   types.BoolUnknown(),
		PreDeployCommand:   types.ListUnknown(types.StringType),
		WatchPatterns:      types.SetUnknown(types.StringType),
		Regions:            types.MapUnknown(types.Int64Type),
	}

	ResolveUnknowns(&model)

	if model.HealthcheckTimeout.IsUnknown() || model.MemoryGB.IsUnknown() || model.SleepApplication.IsUnknown() {
		t.Error("a primitive attribute remained unknown")
	}
	if model.PreDeployCommand.IsUnknown() || model.WatchPatterns.IsUnknown() || model.Regions.IsUnknown() {
		t.Error("a collection attribute remained unknown")
	}

	// The element types must survive, or the value is null of the wrong type and
	// Terraform rejects it just as loudly as an unknown.
	if model.PreDeployCommand.ElementType(t.Context()) != types.StringType {
		t.Error("pre_deploy_command lost its element type")
	}
	if model.Regions.ElementType(t.Context()) != types.Int64Type {
		t.Error("regions lost its element type")
	}
}

// TestResolveUnknownsCoversEveryModelInThePackage is the reason this is
// reflection rather than a hand-written list.
//
// The first version enumerated `service.go`'s attributes by hand and missed four
// of them, including one typed wrongly. A list of a struct's fields maintained
// beside the struct drifts from it; walking the struct cannot.
func TestResolveUnknownsCoversEveryModelInThePackage(t *testing.T) {
	t.Parallel()

	models := map[string]any{
		"service":             &serviceModel{},
		"bucket":              &bucketModel{},
		"postgres":            &postgresModel{},
		"volume":              &volumeModel{},
		"service_domain":      &serviceDomainModel{},
		"environment":         &environmentModel{},
		"project":             &projectModel{},
		"variable_collection": &variableCollectionModel{},
	}

	for name, model := range models {
		// Nothing is unknown here, so this asserts only that the helper accepts
		// every model without panicking — a new resource whose model has a type
		// `nullOf` does not handle would surface as an unresolved unknown at
		// apply time, which is loud but late.
		ResolveUnknowns(model)
		if model == nil {
			t.Errorf("%s model became nil", name)
		}
	}
}
