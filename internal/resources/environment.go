package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

var (
	_ resource.Resource                = (*Environment)(nil)
	_ resource.ResourceWithConfigure   = (*Environment)(nil)
	_ resource.ResourceWithImportState = (*Environment)(nil)
)

type Environment struct {
	client *client.Client
}

type environmentModel struct {
	ID                  types.String   `tfsdk:"id"`
	ProjectID           types.String   `tfsdk:"project_id"`
	Name                types.String   `tfsdk:"name"`
	SourceEnvironmentID types.String   `tfsdk:"source_environment_id"`
	IsEphemeral         types.Bool     `tfsdk:"is_ephemeral"`
	SkipInitialDeploys  types.Bool     `tfsdk:"skip_initial_deploys"`
	ConfigETag          types.String   `tfsdk:"config_etag"`
	Timeouts            timeouts.Value `tfsdk:"timeouts"`
}

func NewEnvironment() resource.Resource {
	return &Environment{}
}

func (r *Environment) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *Environment) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An independently managed Railway environment.",
		Attributes: map[string]schema.Attribute{
			"id": idAttribute("Railway environment ID."),
			"project_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"source_environment_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_ephemeral": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{},
			},
			"skip_initial_deploys": schema.BoolAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.Bool{},
			},
			"config_etag": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Opaque optimistic-concurrency token used for environment changes.",
			},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *Environment) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *Environment) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.CreateEnvironment(ctx, r.client.GraphQL(), railway.EnvironmentCreateInput{
		Name:                plan.Name.ValueString(),
		ProjectId:           plan.ProjectID.ValueString(),
		SourceEnvironmentId: stringPointer(plan.SourceEnvironmentID),
		Ephemeral:           boolPointer(plan.IsEphemeral),
		SkipInitialDeploys:  boolPointer(plan.SkipInitialDeploys),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Railway environment", client.DecodeAPIError(err).Error())
		return
	}
	setEnvironmentState(&plan, result.EnvironmentCreate.EnvironmentFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Environment) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.GetEnvironment(ctx, r.client.GraphQL(), state.ID.ValueString())
	if removeIfNotFound(ctx, err, resp, "Unable to read Railway environment") {
		return
	}
	if result.Environment.DeletedAt != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	setEnvironmentState(&state, result.Environment.EnvironmentFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Environment) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.RenameEnvironment(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.EnvironmentRenameInput{
		Name: plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to rename Railway environment", client.DecodeAPIError(err).Error())
		return
	}
	setEnvironmentState(&plan, result.EnvironmentRename.EnvironmentFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Environment) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	_, err := railway.DeleteEnvironment(ctx, r.client.GraphQL(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway environment", client.DecodeAPIError(err).Error())
	}
}

func (r *Environment) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importID(ctx, req.ID, resp)
}

func setEnvironmentState(state *environmentModel, remote railway.EnvironmentFields) {
	state.ID = types.StringValue(remote.Id)
	state.ProjectID = types.StringValue(remote.ProjectId)
	state.Name = types.StringValue(remote.Name)
	state.IsEphemeral = types.BoolValue(remote.IsEphemeral)
	state.ConfigETag = types.StringValue(remote.ConfigEtag)
}
