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
	ObjectCount   types.Int64  `tfsdk:"object_count"`
	SizeBytes     types.Int64  `tfsdk:"size_bytes"`
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

		// **WHAT IS ACTUALLY IN THE BUCKET.** No secrets, so this belongs on
		// the data source rather than in the ephemeral credentials resource.
		//
		// `object_count` answers the question that governs whether destroying
		// or replacing a bucket is safe, and it was previously unanswerable
		// through the provider at all — the decision had to be made by
		// reasoning about how recently the bucket was created.
		"object_count": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of objects currently stored in this bucket in this environment.",
		},
		"size_bytes": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Total size in bytes of the objects stored in this bucket in this environment.",
		},
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

	details, err := railway.GetBucketInstanceDetails(
		ctx,
		d.client.GraphQL(),
		config.ID.ValueString(),
		config.EnvironmentID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Railway bucket contents", client.DecodeAPIError(err).Error())
		return
	}
	// Railway types these as `BigInt`, which arrives as a STRING — a count that
	// can exceed what JSON numbers represent exactly. Parsed rather than cast.
	config.ObjectCount = bigIntValue(details.BucketInstanceDetails.ObjectCount, "object_count", &resp.Diagnostics)
	config.SizeBytes = bigIntValue(details.BucketInstanceDetails.SizeBytes, "size_bytes", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
