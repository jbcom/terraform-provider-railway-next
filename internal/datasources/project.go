package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

type Project struct{ client *client.Client }

type projectModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	WorkspaceID          types.String `tfsdk:"workspace_id"`
	Description          types.String `tfsdk:"description"`
	IsPublic             types.Bool   `tfsdk:"is_public"`
	DefaultEnvironmentID types.String `tfsdk:"default_environment_id"`
}

func NewProject() datasource.DataSource { return &Project{} }

func (d *Project) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *Project) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":                     schema.StringAttribute{Optional: true, Computed: true},
		"name":                   schema.StringAttribute{Optional: true, Computed: true},
		"workspace_id":           schema.StringAttribute{Optional: true, Computed: true},
		"description":            schema.StringAttribute{Computed: true},
		"is_public":              schema.BoolAttribute{Computed: true},
		"default_environment_id": schema.StringAttribute{Computed: true},
	}}
}

func (d *Project) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func (d *Project) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var remote railway.ProjectFields
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := railway.GetProject(ctx, d.client.GraphQL(), config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read Railway project", client.DecodeAPIError(err).Error())
			return
		}
		remote = result.Project.ProjectFields
	} else {
		if config.Name.IsNull() || config.Name.IsUnknown() {
			resp.Diagnostics.AddError("Missing project lookup", "Configure id or name.")
			return
		}
		var workspaceID *string
		if !config.WorkspaceID.IsNull() && !config.WorkspaceID.IsUnknown() {
			value := config.WorkspaceID.ValueString()
			workspaceID = &value
		}
		result, err := railway.ListProjects(ctx, d.client.GraphQL(), workspaceID)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Railway projects", client.DecodeAPIError(err).Error())
			return
		}
		matches := make([]railway.ProjectFields, 0, 1)
		for _, edge := range result.Projects.Edges {
			if edge.Node.Name == config.Name.ValueString() {
				matches = append(matches, edge.Node.ProjectFields)
			}
		}
		if len(matches) != 1 {
			resp.Diagnostics.AddError("Ambiguous Railway project lookup", fmt.Sprintf("Found %d projects named %q.", len(matches), config.Name.ValueString()))
			return
		}
		remote = matches[0]
	}
	config.ID = types.StringValue(remote.Id)
	config.Name = types.StringValue(remote.Name)
	config.Description = types.StringValue(stringValue(remote.Description))
	config.IsPublic = types.BoolValue(remote.IsPublic)
	config.WorkspaceID = types.StringValue(stringValue(remote.WorkspaceId))
	defaultID := remote.PrimaryEnvironmentId
	if defaultID == nil {
		defaultID = remote.BaseEnvironmentId
	}
	config.DefaultEnvironmentID = types.StringValue(stringValue(defaultID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
