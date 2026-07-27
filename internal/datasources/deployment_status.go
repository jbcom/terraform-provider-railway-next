package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

type DeploymentStatus struct{ client *client.Client }

type deploymentStatusModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	ServiceID     types.String `tfsdk:"service_id"`
	Status        types.String `tfsdk:"status"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func NewDeploymentStatus() datasource.DataSource { return &DeploymentStatus{} }

func (d *DeploymentStatus) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_status"
}

func (d *DeploymentStatus) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":             schema.StringAttribute{Required: true},
		"project_id":     schema.StringAttribute{Computed: true},
		"environment_id": schema.StringAttribute{Computed: true},
		"service_id":     schema.StringAttribute{Computed: true},
		"status":         schema.StringAttribute{Computed: true},
		"created_at":     schema.StringAttribute{Computed: true},
		"updated_at":     schema.StringAttribute{Computed: true},
	}}
}

func (d *DeploymentStatus) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func (d *DeploymentStatus) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config deploymentStatusModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := railway.GetDeploymentStatus(ctx, d.client.GraphQL(), config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway deployment", client.DecodeAPIError(err).Error())
		return
	}
	config.ProjectID = types.StringValue(result.Deployment.ProjectId)
	config.EnvironmentID = types.StringValue(result.Deployment.EnvironmentId)
	config.ServiceID = types.StringValue(stringValue(result.Deployment.ServiceId))
	config.Status = types.StringValue(string(result.Deployment.Status))
	config.CreatedAt = types.StringValue(result.Deployment.CreatedAt)
	config.UpdatedAt = types.StringValue(result.Deployment.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
