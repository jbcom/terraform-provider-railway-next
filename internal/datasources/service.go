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

type Service struct{ client *client.Client }

type serviceModel struct {
	ID                     types.String `tfsdk:"id"`
	ProjectID              types.String `tfsdk:"project_id"`
	EnvironmentID          types.String `tfsdk:"environment_id"`
	Name                   types.String `tfsdk:"name"`
	LatestDeploymentID     types.String `tfsdk:"latest_deployment_id"`
	LatestDeploymentStatus types.String `tfsdk:"latest_deployment_status"`
}

func NewService() datasource.DataSource { return &Service{} }

func (d *Service) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (d *Service) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":                       schema.StringAttribute{Optional: true, Computed: true},
		"project_id":               schema.StringAttribute{Optional: true, Computed: true},
		"environment_id":           schema.StringAttribute{Required: true},
		"name":                     schema.StringAttribute{Optional: true, Computed: true},
		"latest_deployment_id":     schema.StringAttribute{Computed: true},
		"latest_deployment_status": schema.StringAttribute{Computed: true},
	}}
}

func (d *Service) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func (d *Service) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serviceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	serviceID := config.ID.ValueString()
	if serviceID == "" {
		if config.ProjectID.IsNull() || config.Name.IsNull() {
			resp.Diagnostics.AddError("Missing service lookup", "Configure id, or configure project_id and name.")
			return
		}
		list, err := railway.ListProjectServices(ctx, d.client.GraphQL(), config.ProjectID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Railway services", client.DecodeAPIError(err).Error())
			return
		}
		matches := []string{}
		for _, edge := range list.Project.Services.Edges {
			if edge.Node.Name == config.Name.ValueString() {
				matches = append(matches, edge.Node.Id)
			}
		}
		if len(matches) != 1 {
			resp.Diagnostics.AddError("Ambiguous Railway service lookup", fmt.Sprintf("Found %d services named %q.", len(matches), config.Name.ValueString()))
			return
		}
		serviceID = matches[0]
	}
	result, err := railway.GetService(ctx, d.client.GraphQL(), serviceID, config.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway service", client.DecodeAPIError(err).Error())
		return
	}
	config.ID = types.StringValue(result.Service.Id)
	config.ProjectID = types.StringValue(result.Service.ProjectId)
	config.Name = types.StringValue(result.Service.Name)
	for _, edge := range result.Environment.ServiceInstances.Edges {
		if edge.Node.ServiceId == serviceID && edge.Node.LatestDeployment != nil {
			config.LatestDeploymentID = types.StringValue(edge.Node.LatestDeployment.Id)
			config.LatestDeploymentStatus = types.StringValue(string(edge.Node.LatestDeployment.Status))
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
