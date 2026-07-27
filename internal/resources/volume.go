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
	_ resource.Resource                = (*Volume)(nil)
	_ resource.ResourceWithConfigure   = (*Volume)(nil)
	_ resource.ResourceWithImportState = (*Volume)(nil)
)

type Volume struct {
	client *client.Client
}

type volumeModel struct {
	ID               types.String   `tfsdk:"id"`
	VolumeInstanceID types.String   `tfsdk:"volume_instance_id"`
	ProjectID        types.String   `tfsdk:"project_id"`
	EnvironmentID    types.String   `tfsdk:"environment_id"`
	ServiceID        types.String   `tfsdk:"service_id"`
	Name             types.String   `tfsdk:"name"`
	MountPath        types.String   `tfsdk:"mount_path"`
	Region           types.String   `tfsdk:"region"`
	SizeMB           types.Int64    `tfsdk:"size_mb"`
	CurrentSizeMB    types.Float64  `tfsdk:"current_size_mb"`
	State            types.String   `tfsdk:"state"`
	PendingDeletion  types.Bool     `tfsdk:"pending_deletion"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func NewVolume() resource.Resource {
	return &Volume{}
}

func (r *Volume) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_volume"
}

func (r *Volume) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Railway persistent volume. Destroying this resource destroys persistent data. Mutable mount or service changes update the volume instance and do not silently recreate the volume.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true},
			"volume_instance_id": schema.StringAttribute{Computed: true},
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
			"service_id": schema.StringAttribute{Required: true},
			"name":       schema.StringAttribute{Required: true},
			"mount_path": schema.StringAttribute{Required: true},
			"region": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size_mb":          schema.Int64Attribute{Computed: true},
			"current_size_mb":  schema.Float64Attribute{Computed: true},
			"state":            schema.StringAttribute{Computed: true},
			"pending_deletion": schema.BoolAttribute{Computed: true},
			"timeouts":         timeouts.AttributesAll(ctx),
		},
	}
}

func (r *Volume) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *Volume) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutCreate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	result, err := railway.CreateVolume(ctx, r.client.GraphQL(), railway.VolumeCreateInput{
		ProjectId:     plan.ProjectID.ValueString(),
		EnvironmentId: stringPointer(plan.EnvironmentID),
		ServiceId:     stringPointer(plan.ServiceID),
		MountPath:     plan.MountPath.ValueString(),
		Region:        stringPointer(plan.Region),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Railway volume", client.DecodeAPIError(err).Error())
		return
	}
	plan.ID = types.StringValue(result.VolumeCreate.Id)
	if plan.Name.ValueString() != result.VolumeCreate.Name {
		updated, updateErr := railway.UpdateVolume(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.VolumeUpdateInput{
			Name: stringPointer(plan.Name),
		})
		if updateErr != nil {
			resp.Diagnostics.AddError("Railway volume created but naming failed", client.DecodeAPIError(updateErr).Error())
			return
		}
		plan.Name = types.StringValue(updated.VolumeUpdate.Name)
	}
	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Volume) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutRead, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if !r.refresh(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Volume) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan volumeModel
	var prior volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, plan.Timeouts, timeoutUpdate, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	if plan.Name.ValueString() != prior.Name.ValueString() {
		if _, err := railway.UpdateVolume(ctx, r.client.GraphQL(), plan.ID.ValueString(), railway.VolumeUpdateInput{
			Name: stringPointer(plan.Name),
		}); err != nil {
			resp.Diagnostics.AddError("Unable to rename Railway volume", client.DecodeAPIError(err).Error())
			return
		}
	}
	if plan.ServiceID.ValueString() != prior.ServiceID.ValueString() ||
		plan.MountPath.ValueString() != prior.MountPath.ValueString() {
		serviceID := plan.ServiceID.ValueString()
		_, err := railway.UpdateVolumeInstance(
			ctx,
			r.client.GraphQL(),
			plan.ID.ValueString(),
			plan.EnvironmentID.ValueString(),
			railway.VolumeInstanceUpdateInput{
				ServiceId: &serviceID,
				MountPath: stringPointer(plan.MountPath),
			},
		)
		if err != nil {
			resp.Diagnostics.AddError("Unable to update Railway volume attachment", client.DecodeAPIError(err).Error())
			return
		}
	}
	if !r.refresh(ctx, &plan, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Volume) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel, ok := operationContext(ctx, state.Timeouts, timeoutDelete, &resp.Diagnostics)
	if !ok {
		return
	}
	defer cancel()
	_, err := railway.DeleteVolume(ctx, r.client.GraphQL(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Railway volume", client.DecodeAPIError(err).Error())
	}
}

func (r *Volume) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diagnostics := splitImportID(req.ID, 3)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func (r *Volume) refresh(ctx context.Context, state *volumeModel, diagnostics *diag.Diagnostics) bool {
	result, err := railway.GetProjectVolumes(
		ctx,
		r.client.GraphQL(),
		state.ProjectID.ValueString(),
		state.EnvironmentID.ValueString(),
	)
	if client.IsNotFound(err) {
		return false
	}
	if err != nil {
		diagnostics.AddError("Unable to read Railway volume", client.DecodeAPIError(err).Error())
		return false
	}
	foundVolume := false
	for _, edge := range result.Project.Volumes.Edges {
		if edge.Node.Id == state.ID.ValueString() {
			foundVolume = true
			state.Name = types.StringValue(edge.Node.Name)
			state.ProjectID = types.StringValue(edge.Node.ProjectId)
			break
		}
	}
	if !foundVolume {
		return false
	}
	for _, edge := range result.Environment.VolumeInstances.Edges {
		if edge.Node.VolumeId != state.ID.ValueString() {
			continue
		}
		instance := edge.Node
		if instance.DeletedAt != nil {
			return false
		}
		state.VolumeInstanceID = types.StringValue(instance.Id)
		state.EnvironmentID = types.StringValue(instance.EnvironmentId)
		state.ServiceID = valueString(instance.ServiceId)
		state.MountPath = types.StringValue(instance.MountPath)
		state.Region = valueString(instance.Region)
		state.SizeMB = types.Int64Value(int64(instance.SizeMB))
		state.CurrentSizeMB = types.Float64Value(instance.CurrentSizeMB)
		state.PendingDeletion = types.BoolValue(instance.IsPendingDeletion)
		if instance.State == nil {
			state.State = types.StringNull()
		} else {
			state.State = types.StringValue(string(*instance.State))
		}
		return true
	}
	return false
}
