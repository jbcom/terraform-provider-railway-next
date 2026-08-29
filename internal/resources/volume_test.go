// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestVolumeCreateReportsCancellationAndRetainsCreatedIdentity(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		switch request.OperationName {
		case "CreateVolume":
			return map[string]any{"data": map[string]any{
				"volumeCreate": map[string]any{
					"id": "volume-fixture", "name": "api-data", "projectId": "project-fixture",
				},
			}}, nil
		case "GetProjectVolumes":
			cancel()
			return nil, context.Canceled
		default:
			return nil, errors.New("unexpected operation " + request.OperationName)
		}
	})
	volume := &Volume{client: testClient(t, "https://fixture.invalid/graphql", doer)}

	var schemaResponse resource.SchemaResponse
	volume.Schema(parent, resource.SchemaRequest{}, &schemaResponse)
	plan := tfsdk.Plan{Schema: schemaResponse.Schema}
	timeoutTypes := map[string]attr.Type{
		"create": types.StringType,
		"read":   types.StringType,
		"update": types.StringType,
		"delete": types.StringType,
	}
	planned := volumeModel{
		ID:               types.StringUnknown(),
		VolumeInstanceID: types.StringUnknown(),
		ProjectID:        types.StringValue("project-fixture"),
		EnvironmentID:    types.StringValue("environment-fixture"),
		ServiceID:        types.StringValue("service-fixture"),
		Name:             types.StringValue("api-data"),
		MountPath:        types.StringValue("/data"),
		Region:           types.StringNull(),
		SizeMB:           types.Int64Unknown(),
		CurrentSizeMB:    types.Float64Unknown(),
		State:            types.StringUnknown(),
		PendingDeletion:  types.BoolUnknown(),
		Timeouts: timeouts.Value{
			Object: types.ObjectNull(timeoutTypes),
		},
	}
	if diagnostics := plan.Set(parent, &planned); diagnostics.HasError() {
		t.Fatalf("set plan: %v", diagnostics)
	}

	response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	volume.Create(parent, resource.CreateRequest{Plan: plan}, &response)

	if !response.Diagnostics.HasError() {
		t.Fatal("Create returned no cancellation diagnostic")
	}
	diagnosticText := response.Diagnostics.Errors()[0].Summary() + " " +
		response.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(diagnosticText, "canceled") {
		t.Fatalf("diagnostic = %q, want cancellation provenance", diagnosticText)
	}
	if strings.Contains(diagnosticText, "configured create timeout") {
		t.Fatalf("diagnostic = %q, cancellation was misreported as timeout", diagnosticText)
	}

	var saved volumeModel
	if diagnostics := response.State.Get(context.Background(), &saved); diagnostics.HasError() {
		t.Fatalf("read saved state: %v", diagnostics)
	}
	if saved.ID.ValueString() != "volume-fixture" {
		t.Fatalf("saved volume id = %q, want created identity", saved.ID.ValueString())
	}
	if saved.ServiceID.ValueString() != "service-fixture" || saved.MountPath.ValueString() != "/data" {
		t.Fatalf("saved attachment = %q at %q, want requested attachment retained",
			saved.ServiceID.ValueString(), saved.MountPath.ValueString())
	}
}
