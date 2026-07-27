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

type Environment struct{ client *client.Client }

type environmentModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	IsEphemeral types.Bool   `tfsdk:"is_ephemeral"`
	ConfigETag  types.String `tfsdk:"config_etag"`
}

func NewEnvironment() datasource.DataSource { return &Environment{} }

func (d *Environment) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *Environment) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":           schema.StringAttribute{Optional: true, Computed: true},
		"project_id":   schema.StringAttribute{Optional: true, Computed: true},
		"name":         schema.StringAttribute{Optional: true, Computed: true},
		"is_ephemeral": schema.BoolAttribute{Computed: true},
		"config_etag":  schema.StringAttribute{Computed: true},
	}}
}

func (d *Environment) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func (d *Environment) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config environmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var remote railway.EnvironmentFields
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := railway.GetEnvironment(ctx, d.client.GraphQL(), config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read Railway environment", client.DecodeAPIError(err).Error())
			return
		}
		remote = result.Environment.EnvironmentFields
	} else {
		if config.ProjectID.IsNull() || config.Name.IsNull() {
			resp.Diagnostics.AddError("Missing environment lookup", "Configure id, or configure project_id and name.")
			return
		}
		result, err := railway.ListEnvironments(ctx, d.client.GraphQL(), config.ProjectID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Railway environments", client.DecodeAPIError(err).Error())
			return
		}
		matches := make([]railway.EnvironmentFields, 0, 1)
		for _, edge := range result.Environments.Edges {
			if edge.Node.Name == config.Name.ValueString() {
				matches = append(matches, edge.Node.EnvironmentFields)
			}
		}
		if len(matches) != 1 {
			resp.Diagnostics.AddError("Ambiguous Railway environment lookup", fmt.Sprintf("Found %d environments named %q.", len(matches), config.Name.ValueString()))
			return
		}
		remote = matches[0]
	}
	config.ID = types.StringValue(remote.Id)
	config.ProjectID = types.StringValue(remote.ProjectId)
	config.Name = types.StringValue(remote.Name)
	config.IsEphemeral = types.BoolValue(remote.IsEphemeral)
	config.ConfigETag = types.StringValue(remote.ConfigEtag)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
