package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
	"github.com/micah5/terraform-provider-railway-next/internal/datasources"
	"github.com/micah5/terraform-provider-railway-next/internal/ephemeralresources"
	"github.com/micah5/terraform-provider-railway-next/internal/resources"
)

// Terraform resource names use railway_* while the distribution address remains
// registry.terraform.io/micah5/railway-next. Users should therefore declare the
// provider's local name as "railway" in required_providers.
const typeName = "railway"

var _ provider.Provider = (*RailwayProvider)(nil)

type RailwayProvider struct {
	version string
}

type model struct {
	Token           types.String `tfsdk:"token"`
	TokenType       types.String `tfsdk:"token_type"`
	GraphQLEndpoint types.String `tfsdk:"graphql_endpoint"`
	RequestTimeout  types.Int64  `tfsdk:"request_timeout_seconds"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RailwayProvider{version: version}
	}
}

func (p *RailwayProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = typeName
	resp.Version = p.version
}

func (p *RailwayProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Native Go provider for Railway's GraphQL API. This is an independent community provider and is not affiliated with or endorsed by Railway Corporation.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Railway account, workspace, or project token. Prefer RAILWAY_API_TOKEN or RAILWAY_TOKEN.",
			},
			"token_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Authentication type: `account`, `workspace`, or `project`. Account and workspace tokens use Bearer authentication.",
				Validators: []validator.String{
					stringvalidator.OneOf("account", "workspace", "project"),
				},
			},
			"graphql_endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Railway GraphQL endpoint. Can also be set with RAILWAY_GRAPHQL_ENDPOINT.",
			},
			"request_timeout_seconds": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Per-request timeout in seconds. Defaults to 30.",
			},
		},
	}
}

func (p *RailwayProvider) Configure(
	ctx context.Context,
	req provider.ConfigureRequest,
	resp *provider.ConfigureResponse,
) {
	var data model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Token.IsUnknown() || data.TokenType.IsUnknown() ||
		data.GraphQLEndpoint.IsUnknown() || data.RequestTimeout.IsUnknown() {
		resp.Diagnostics.AddError(
			"Unknown Railway provider configuration",
			"Provider token, token_type, graphql_endpoint, and request_timeout_seconds must be known during configuration.",
		)
		return
	}
	timeout := 30 * time.Second
	if !data.RequestTimeout.IsNull() {
		timeout = time.Duration(data.RequestTimeout.ValueInt64()) * time.Second
	}
	configured, err := client.New(client.Config{
		Token:     data.Token.ValueString(),
		TokenType: client.TokenType(data.TokenType.ValueString()),
		Endpoint:  data.GraphQLEndpoint.ValueString(),
		Timeout:   timeout,
		Version:   p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Railway client", err.Error())
		return
	}
	resp.ResourceData = configured
	resp.DataSourceData = configured
	// **EPHEMERAL RESOURCES NEED THEIR OWN ASSIGNMENT.** The framework does not
	// derive this from the other two, so omitting it left every ephemeral
	// resource with a nil client — which surfaced as a SIGSEGV in the provider
	// process rather than as a diagnostic.
	resp.EphemeralResourceData = configured
}

func (p *RailwayProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewProject,
		resources.NewEnvironment,
		resources.NewService,
		resources.NewVolume,
		resources.NewVariableCollection,
		resources.NewSecret,
		resources.NewServiceDomain,
		resources.NewBucket,
		resources.NewPostgres,
	}
}

// EphemeralResources registers values that must never reach state.
//
// **THE PROVIDER IMPLEMENTED NO EPHEMERAL RESOURCES BEFORE THIS**, which meant
// a bucket's real S3 credentials were unreachable except by hand — the only
// safe options the provider offered were reference expressions, which work
// solely for a consumer running inside Railway. Modelling them as a data source
// instead would have written a live secret into the state file.
var _ provider.ProviderWithEphemeralResources = (*RailwayProvider)(nil)

func (p *RailwayProvider) EphemeralResources(context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		ephemeralresources.NewBucketCredentials,
	}
}

func (p *RailwayProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewProject,
		datasources.NewEnvironment,
		datasources.NewService,
		datasources.NewBucket,
		datasources.NewBucketCredentials,
		datasources.NewDeploymentStatus,
	}
}
