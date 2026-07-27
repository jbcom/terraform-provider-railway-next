package datasources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
	"github.com/micah5/terraform-provider-railway-next/internal/references"
)

type Bucket struct{ client *client.Client }

type bucketModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`
	Region        types.String `tfsdk:"region"`
	References    types.Map    `tfsdk:"references"`
}

func NewBucket() datasource.DataSource { return &Bucket{} }

func (d *Bucket) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (d *Bucket) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":             schema.StringAttribute{Optional: true, Computed: true},
		"project_id":     schema.StringAttribute{Required: true},
		"environment_id": schema.StringAttribute{Required: true},
		"name":           schema.StringAttribute{Optional: true, Computed: true},
		"region":         schema.StringAttribute{Computed: true},
		"references":     schema.MapAttribute{Computed: true, ElementType: types.StringType},
	}}
}

func (d *Bucket) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func (d *Bucket) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config bucketModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := railway.ListProjectBuckets(ctx, d.client.GraphQL(), config.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Railway buckets", client.DecodeAPIError(err).Error())
		return
	}
	matches := []railway.BucketFields{}
	for _, edge := range list.Project.Buckets.Edges {
		if (!config.ID.IsNull() && edge.Node.Id == config.ID.ValueString()) ||
			(config.ID.IsNull() && edge.Node.Name == config.Name.ValueString()) {
			matches = append(matches, edge.Node.BucketFields)
		}
	}
	if len(matches) != 1 {
		resp.Diagnostics.AddError("Ambiguous Railway bucket lookup", fmt.Sprintf("Found %d matching buckets.", len(matches)))
		return
	}
	config.ID = types.StringValue(matches[0].Id)
	config.Name = types.StringValue(matches[0].Name)
	environment, err := railway.GetEnvironmentConfiguration(ctx, d.client.GraphQL(), config.EnvironmentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway environment bucket registration", client.DecodeAPIError(err).Error())
		return
	}
	var opaque struct {
		Buckets map[string]*struct {
			Region string `json:"region"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal(environment.Environment.Config, &opaque); err != nil || opaque.Buckets[config.ID.ValueString()] == nil {
		resp.Diagnostics.AddError("Bucket is not registered in environment", "The bucket exists at project scope but has no active environment registration.")
		return
	}
	config.Region = types.StringValue(opaque.Buckets[config.ID.ValueString()].Region)
	config.References, _ = types.MapValueFrom(ctx, types.StringType, references.Bucket(config.Name.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
