package provider

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithConfigure   = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*projectResource)(nil)
)

// NewProjectResource is the gns3_project resource.
func NewProjectResource() resource.Resource { return &projectResource{} }

type projectResource struct{ client gns3.Client }

type projectResourceModel struct {
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Path        types.String `tfsdk:"path"`
	AutoStart   types.Bool   `tfsdk:"auto_start"`
	AutoOpen    types.Bool   `tfsdk:"auto_open"`
	AutoClose   types.Bool   `tfsdk:"auto_close"`
	SceneWidth  types.Int64  `tfsdk:"scene_width"`
	SceneHeight types.Int64  `tfsdk:"scene_height"`
	GridSize    types.Int64  `tfsdk:"grid_size"`
	ShowGrid    types.Bool   `tfsdk:"show_grid"`
	Status      types.String `tfsdk:"status"`
	Filename    types.String `tfsdk:"filename"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

// Defaults are the GNS3 2.2.52 server defaults shown in the POST /v2/projects
// sample session, https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projects.html
func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A GNS3 project. Created opened; destroy closes it and then deletes it from disk.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project UUID assigned by the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name. Must be unique on the server.",
			},
			"path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Project directory on the server. The server picks one under its projects directory when unset. Changing it replaces the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auto_start": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Start all nodes when the project is opened. Default `false`.",
			},
			"auto_open": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Open the project when the GNS3 server starts. Default `false`.",
			},
			"auto_close": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				MarkdownDescription: "Close the project when the last client disconnects from its notification feed. Default `true`.",
			},
			"scene_width": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(2000),
				MarkdownDescription: "Width of the drawing area. Default `2000`.",
			},
			"scene_height": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(1000),
				MarkdownDescription: "Height of the drawing area. Default `1000`.",
			},
			"grid_size": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(75),
				MarkdownDescription: "Grid size for nodes on the drawing area. Default `75`.",
			},
			"show_grid": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Show the grid on the drawing area. Default `false`.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`opened` or `closed`, as last read from the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"filename": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project file name (`<name>.gns3`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// ModifyPlan marks filename unknown on a rename: the server may rename the
// .gns3 file with the project, and UseStateForUnknown would promise the old one.
func (r *projectResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var planName, stateName types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &planName)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("name"), &stateName)...)
	if !planName.Equal(stateName) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("filename"), types.StringUnknown())...)
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (m *projectResourceModel) fromAPI(p *gns3.Project) {
	m.ProjectID = types.StringValue(p.ProjectID)
	m.Name = types.StringValue(p.Name)
	m.Path = types.StringValue(p.Path)
	m.AutoStart = types.BoolValue(p.AutoStart)
	m.AutoOpen = types.BoolValue(p.AutoOpen)
	m.AutoClose = types.BoolValue(p.AutoClose)
	m.SceneWidth = types.Int64Value(p.SceneWidth)
	m.SceneHeight = types.Int64Value(p.SceneHeight)
	m.GridSize = types.Int64Value(p.GridSize)
	m.ShowGrid = types.BoolValue(p.ShowGrid)
	m.Status = types.StringValue(p.Status)
	m.Filename = types.StringValue(p.Filename)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreateProject(ctx, gns3.ProjectCreateRequest{
		Name:        plan.Name.ValueString(),
		Path:        knownString(plan.Path),
		AutoStart:   knownBool(plan.AutoStart),
		AutoOpen:    knownBool(plan.AutoOpen),
		AutoClose:   knownBool(plan.AutoClose),
		SceneWidth:  knownInt64(plan.SceneWidth),
		SceneHeight: knownInt64(plan.SceneHeight),
		GridSize:    knownInt64(plan.GridSize),
		ShowGrid:    knownBool(plan.ShowGrid),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating GNS3 project", err.Error())
		return
	}
	plan.fromAPI(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetProject(ctx, state.ProjectID.ValueString())
	if gns3.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading GNS3 project", err.Error())
		return
	}
	state.fromAPI(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.UpdateProject(ctx, state.ProjectID.ValueString(), gns3.ProjectUpdateRequest{
		Name:        knownString(plan.Name),
		AutoStart:   knownBool(plan.AutoStart),
		AutoOpen:    knownBool(plan.AutoOpen),
		AutoClose:   knownBool(plan.AutoClose),
		SceneWidth:  knownInt64(plan.SceneWidth),
		SceneHeight: knownInt64(plan.SceneHeight),
		GridSize:    knownInt64(plan.GridSize),
		ShowGrid:    knownBool(plan.ShowGrid),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating GNS3 project", err.Error())
		return
	}
	plan.fromAPI(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ProjectID.ValueString()
	if err := r.client.CloseProject(ctx, id); err != nil {
		if gns3.IsNotFound(err) {
			return // already gone
		}
		resp.Diagnostics.AddError("Error closing GNS3 project before delete", err.Error())
		return
	}
	if err := deleteProjectRetrying(ctx, r.client, id); err != nil && !gns3.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting GNS3 project", err.Error())
	}
}

// projectDeleteRetryWindow bounds deleteProjectRetrying.
var projectDeleteRetryWindow = 15 * time.Second

// deleteProjectRetrying deletes a project, retrying only the 409 "Cannot
// delete project directory" the server returns while a just-deleted node's
// emulator process on Windows still holds a file in the project directory
// open (DELETE .../nodes/{id} returns 204 before vpcs.exe has exited; seen on
// 2.2.61 as "[WinError 32] ... vpcs.log", docs/api-drift.md).
func deleteProjectRetrying(ctx context.Context, c gns3.Client, id string) error {
	deadline := time.Now().Add(projectDeleteRetryWindow)
	wait := 250 * time.Millisecond
	for {
		err := c.DeleteProject(ctx, id)
		var apiErr *gns3.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict ||
			!strings.Contains(apiErr.Message, "Cannot delete project directory") || time.Now().Add(wait).After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
		wait = min(wait*2, 2*time.Second)
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("project_id"), req, resp)
}
