package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

var (
	_ resource.Resource                = (*Secret)(nil)
	_ resource.ResourceWithConfigure   = (*Secret)(nil)
	_ resource.ResourceWithImportState = (*Secret)(nil)
)

type Secret struct {
	client *client.Client
}

type secretModel struct {
	ID             types.String   `tfsdk:"id"`
	ProjectID      types.String   `tfsdk:"project_id"`
	EnvironmentID  types.String   `tfsdk:"environment_id"`
	ServiceID      types.String   `tfsdk:"service_id"`
	Name           types.String   `tfsdk:"name"`
	ValueWO        types.String   `tfsdk:"value_wo"`
	ValueWOVersion types.Int64    `tfsdk:"value_wo_version"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

func NewSecret() resource.Resource {
	return &Secret{}
}

func (r *Secret) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *Secret) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Railway secret variable using Terraform 1.11+ write-only state semantics. The provider never returns the value from Read. Increment value_wo_version to rotate the secret.",
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
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value_wo": schema.StringAttribute{
				Required:            true,
				WriteOnly:           true,
				Sensitive:           true,
				MarkdownDescription: "Secret value. It is sent to Railway but omitted from Terraform plan and state.",
			},
			"value_wo_version": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Rotation trigger. Increment whenever value_wo changes.",
			},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *Secret) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *Secret) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config secretModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, config.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if !r.upsert(ctx, &config, &resp.Diagnostics) {
		return
	}
	config.ID = types.StringValue(secretID(&config))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (r *Secret) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	variables, err := r.remoteVariables(ctx, &state)
	if removeIfNotFound(ctx, err, resp, "Unable to read Railway secret") {
		return
	}
	if _, exists := variables[state.Name.ValueString()]; !exists {
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(secretID(&state))
	state.ValueWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Secret) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var config secretModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, config.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if !r.upsert(ctx, &config, &resp.Diagnostics) {
		return
	}
	config.ID = types.StringValue(secretID(&config))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (r *Secret) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	serviceID := state.ServiceID.ValueString()
	_, err := railway.DeleteVariable(ctx, r.client.GraphQL(), railway.VariableDeleteInput{
		ProjectId:     state.ProjectID.ValueString(),
		EnvironmentId: state.EnvironmentID.ValueString(),
		ServiceId:     &serviceID,
		Name:          state.Name.ValueString(),
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway secret", client.DecodeAPIError(err).Error())
	}
}

func (r *Secret) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diagnostics := splitImportID(req.ID, 4)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[3])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *Secret) upsert(ctx context.Context, state *secretModel, diagnostics *diag.Diagnostics) bool {
	if state.ValueWO.IsNull() || state.ValueWO.IsUnknown() {
		diagnostics.AddError("Missing write-only secret", "value_wo must be supplied whenever value_wo_version changes.")
		return false
	}
	serviceID := state.ServiceID.ValueString()
	skipDeploys := false
	_, err := railway.UpsertVariable(ctx, r.client.GraphQL(), railway.VariableUpsertInput{
		ProjectId:     state.ProjectID.ValueString(),
		EnvironmentId: state.EnvironmentID.ValueString(),
		ServiceId:     &serviceID,
		Name:          state.Name.ValueString(),
		Value:         state.ValueWO.ValueString(),
		SkipDeploys:   &skipDeploys,
	})
	if err == nil {
		return true
	}
	if client.IsAmbiguousMutationError(err) {
		variables, readErr := r.remoteVariables(ctx, state)
		if readErr == nil {
			if _, exists := variables[state.Name.ValueString()]; exists {
				diagnostics.AddWarning(
					"Railway secret mutation result was ambiguous",
					"Railway contains a variable with the requested name, but the write-only value cannot be read back. Increment value_wo_version and apply again if the request did not take effect.",
				)
				return true
			}
		}
	}
	diagnostics.AddError("Unable to write Railway secret", client.DecodeAPIError(err).Error())
	return false
}

func (r *Secret) remoteVariables(ctx context.Context, state *secretModel) (map[string]string, error) {
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

func secretID(state *secretModel) string {
	return state.ProjectID.ValueString() + "/" +
		state.EnvironmentID.ValueString() + "/" +
		state.ServiceID.ValueString() + "/" +
		state.Name.ValueString()
}
