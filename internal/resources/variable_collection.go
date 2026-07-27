package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/changeset"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

var (
	_ resource.Resource                = (*VariableCollection)(nil)
	_ resource.ResourceWithConfigure   = (*VariableCollection)(nil)
	_ resource.ResourceWithImportState = (*VariableCollection)(nil)
)

type VariableCollection struct {
	client *client.Client
}

type variableCollectionModel struct {
	ID            types.String   `tfsdk:"id"`
	ProjectID     types.String   `tfsdk:"project_id"`
	EnvironmentID types.String   `tfsdk:"environment_id"`
	ServiceID     types.String   `tfsdk:"service_id"`
	Variables     types.Map      `tfsdk:"variables"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func NewVariableCollection() resource.Resource {
	return &VariableCollection{}
}

func (r *VariableCollection) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_variable_collection"
}

func (r *VariableCollection) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An authoritative collection of only the listed Railway variables for one service/environment. Updates, including removals, are committed atomically and cause at most one deployment. Other remote variables are preserved. Values are stored in Terraform state; use railway_secret for secrets.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"variables": schema.MapAttribute{
				Required:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Variables owned by this collection. Railway reference expressions are passed through verbatim.",
			},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *VariableCollection) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *VariableCollection) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan variableCollectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	desired := mapFromTerraform(ctx, plan.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	remote, err := r.remoteVariables(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway variables before create", client.DecodeAPIError(err).Error())
		return
	}
	before := selectKeys(remote, desired)
	if !r.apply(ctx, &plan, before, desired, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(collectionID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VariableCollection) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state variableCollectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	remote, err := r.remoteVariables(ctx, &state)
	if removeIfNotFound(ctx, err, resp, "Unable to read Railway variable collection") {
		return
	}

	if state.Variables.IsNull() {
		// Import adopts all variables currently attached to this service. This
		// is explicit in the import documentation because subsequent plans own
		// the adopted keys.
		state.Variables = mapToTerraform(ctx, remote, &resp.Diagnostics)
	} else {
		owned := mapFromTerraform(ctx, state.Variables, &resp.Diagnostics)
		refreshed := make(map[string]string, len(owned))
		found := 0
		for key := range owned {
			if value, ok := remote[key]; ok {
				refreshed[key] = value
				found++
			}
		}
		if len(owned) > 0 && found == 0 {
			resp.State.RemoveResource(ctx)
			return
		}
		state.Variables = mapToTerraform(ctx, refreshed, &resp.Diagnostics)
	}
	state.ID = types.StringValue(collectionID(&state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VariableCollection) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan variableCollectionModel
	var prior variableCollectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	desired := mapFromTerraform(ctx, plan.Variables, &resp.Diagnostics)
	ownedBefore := mapFromTerraform(ctx, prior.Variables, &resp.Diagnostics)
	remote, err := r.remoteVariables(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway variables before update", client.DecodeAPIError(err).Error())
		return
	}
	before := make(map[string]string, len(ownedBefore))
	for key, oldStateValue := range ownedBefore {
		if value, ok := remote[key]; ok {
			before[key] = value
		} else {
			before[key] = oldStateValue
		}
	}
	for key := range desired {
		if value, ok := remote[key]; ok {
			before[key] = value
		}
	}
	if !r.apply(ctx, &plan, before, desired, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(collectionID(&plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VariableCollection) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state variableCollectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	owned := mapFromTerraform(ctx, state.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || len(owned) == 0 {
		return
	}
	remote, err := r.remoteVariables(ctx, &state)
	if client.IsNotFound(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway variables before delete", client.DecodeAPIError(err).Error())
		return
	}
	before := selectKeys(remote, owned)
	r.apply(ctx, &state, before, map[string]string{}, &resp.Diagnostics)
}

func (r *VariableCollection) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diagnostics := splitImportID(req.ID, 3)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *VariableCollection) apply(
	ctx context.Context,
	state *variableCollectionModel,
	before map[string]string,
	after map[string]string,
	diagnostics *diag.Diagnostics,
) bool {
	service, err := railway.GetService(
		ctx,
		r.client.GraphQL(),
		state.ServiceID.ValueString(),
		state.EnvironmentID.ValueString(),
	)
	if err != nil {
		diagnostics.AddError("Unable to resolve Railway service for variables", client.DecodeAPIError(err).Error())
		return false
	}
	set := changeset.VariableCollection(service.Service.Name, before, after)
	if len(set.Changes) == 0 {
		return true
	}
	payload, err := set.JSON()
	if err != nil {
		diagnostics.AddError("Unable to build Railway variable change set", err.Error())
		return false
	}
	message := fmt.Sprintf("Terraform: update %d variables for %s", len(set.Changes), service.Service.Name)
	_, err = applyEnvironmentChangeSet(
		ctx,
		r.client.GraphQL(),
		state.EnvironmentID.ValueString(),
		payload,
		message,
	)
	if err == nil {
		return true
	}

	remote, readErr := r.remoteVariables(ctx, state)
	if readErr == nil && collectionMatches(remote, before, after) {
		return true
	}
	diagnostics.AddError(
		"Unable to apply Railway variable collection",
		"The mutation failed and read-after-error reconciliation did not confirm the requested collection. "+client.DecodeAPIError(err).Error(),
	)
	return false
}

func (r *VariableCollection) remoteVariables(ctx context.Context, state *variableCollectionModel) (map[string]string, error) {
	serviceID := state.ServiceID.ValueString()
	result, err := railway.ListVariables(
		ctx,
		r.client.GraphQL(),
		state.ProjectID.ValueString(),
		state.EnvironmentID.ValueString(),
		&serviceID,
	)
	if err != nil {
		return nil, err
	}
	return result.Variables, nil
}

func mapFromTerraform(ctx context.Context, value types.Map, diagnostics *diag.Diagnostics) map[string]string {
	result := map[string]string{}
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

func mapToTerraform(ctx context.Context, value map[string]string, diagnostics *diag.Diagnostics) types.Map {
	result, converted := types.MapValueFrom(ctx, types.StringType, value)
	diagnostics.Append(converted...)
	return result
}

func selectKeys(remote, owned map[string]string) map[string]string {
	result := make(map[string]string, len(owned))
	for key := range owned {
		if value, ok := remote[key]; ok {
			result[key] = value
		}
	}
	return result
}

func collectionMatches(remote, before, after map[string]string) bool {
	for key, value := range after {
		if remote[key] != value {
			return false
		}
	}
	for key := range before {
		if _, remains := after[key]; !remains {
			if _, exists := remote[key]; exists {
				return false
			}
		}
	}
	return true
}

func collectionID(state *variableCollectionModel) string {
	return state.ProjectID.ValueString() + "/" +
		state.EnvironmentID.ValueString() + "/" +
		state.ServiceID.ValueString()
}
