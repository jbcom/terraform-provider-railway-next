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
	_ resource.Resource                = (*Project)(nil)
	_ resource.ResourceWithConfigure   = (*Project)(nil)
	_ resource.ResourceWithImportState = (*Project)(nil)
)

type Project struct {
	client *client.Client
}

type projectModel struct {
	ID                     types.String   `tfsdk:"id"`
	Name                   types.String   `tfsdk:"name"`
	Description            types.String   `tfsdk:"description"`
	IsPublic               types.Bool     `tfsdk:"is_public"`
	WorkspaceID            types.String   `tfsdk:"workspace_id"`
	DefaultEnvironmentName types.String   `tfsdk:"default_environment_name"`
	DefaultEnvironmentID   types.String   `tfsdk:"default_environment_id"`
	PRDeploys              types.Bool     `tfsdk:"pr_deploys"`
	BotPREnvironments      types.Bool     `tfsdk:"bot_pr_environments"`
	FocusedPREnvironments  types.Bool     `tfsdk:"focused_pr_environments"`
	Timeouts               timeouts.Value `tfsdk:"timeouts"`
}

func NewProject() resource.Resource {
	return &Project{}
}

func (r *Project) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *Project) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Railway project. Destroying this resource permanently deletes the project and everything in it; review the destroy plan carefully.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Railway project ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project display name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project description.",
			},
			"is_public": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the project is public.",
			},
			"workspace_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Workspace in which to create the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_environment_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the default environment created with the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_environment_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of Railway's primary/default environment.",
			},
			"pr_deploys": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enable pull-request deployments.",
			},
			"bot_pr_environments": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enable bot-created PR environments where supported.",
			},
			"focused_pr_environments": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enable focused PR environments where supported.",
			},
			"timeouts": timeouts.AttributesAll(ctx),
		},
	}
}

func (r *Project) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *Project) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	defaultName := stringPointer(plan.DefaultEnvironmentName)
	if defaultName == nil {
		value := "production"
		defaultName = &value
	}
	result, err := railway.CreateProject(ctx, r.client.GraphQL(), railway.ProjectCreateInput{
		Name:                   stringPointer(plan.Name),
		Description:            stringPointer(plan.Description),
		IsPublic:               boolPointer(plan.IsPublic),
		WorkspaceId:            stringPointer(plan.WorkspaceID),
		DefaultEnvironmentName: defaultName,
		PrDeploys:              boolPointer(plan.PRDeploys),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Railway project", client.DecodeAPIError(err).Error())
		return
	}
	plan.ID = types.StringValue(result.ProjectCreate.Id)
	plan.DefaultEnvironmentName = types.StringValue(*defaultName)
	setProjectState(&plan, result.ProjectCreate.ProjectFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Project) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.GetProject(ctx, r.client.GraphQL(), state.ID.ValueString())
	if removeIfNotFound(ctx, err, resp, "Unable to read Railway project") {
		return
	}
	setProjectState(&state, result.Project.ProjectFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Project) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.UpdateProject(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.ProjectUpdateInput{
		Name:                  stringPointer(plan.Name),
		Description:           stringPointer(plan.Description),
		IsPublic:              boolPointer(plan.IsPublic),
		PrDeploys:             boolPointer(plan.PRDeploys),
		BotPrEnvironments:     boolPointer(plan.BotPREnvironments),
		FocusedPrEnvironments: boolPointer(plan.FocusedPREnvironments),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Railway project", client.DecodeAPIError(err).Error())
		return
	}
	setProjectState(&plan, result.ProjectUpdate.ProjectFields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Project) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	_, err := railway.DeleteProject(ctx, r.client.GraphQL(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway project", client.DecodeAPIError(err).Error())
	}
}

func (r *Project) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importID(ctx, req.ID, resp)
}

func setProjectState(state *projectModel, remote railway.ProjectFields) {
	state.ID = types.StringValue(remote.Id)
	state.Name = types.StringValue(remote.Name)
	state.Description = valueString(remote.Description)
	state.IsPublic = types.BoolValue(remote.IsPublic)
	state.WorkspaceID = valueString(remote.WorkspaceId)
	state.PRDeploys = types.BoolValue(remote.PrDeploys)
	state.BotPREnvironments = types.BoolValue(remote.BotPrEnvironments)
	state.FocusedPREnvironments = types.BoolValue(remote.FocusedPrEnvironments)

	defaultID := remote.PrimaryEnvironmentId
	if defaultID == nil {
		defaultID = remote.BaseEnvironmentId
	}
	if defaultID == nil && len(remote.Environments.Edges) > 0 {
		value := remote.Environments.Edges[0].Node.Id
		defaultID = &value
	}
	state.DefaultEnvironmentID = valueString(defaultID)
	if defaultID != nil {
		for _, edge := range remote.Environments.Edges {
			if edge.Node.Id == *defaultID {
				state.DefaultEnvironmentName = types.StringValue(edge.Node.Name)
				break
			}
		}
	}
}
