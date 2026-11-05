package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

// snapshotAction is gns3_snapshot: create a project snapshot, or restore one
// by name.
type snapshotAction struct {
	client gns3.Client
}

var (
	_ action.Action              = (*snapshotAction)(nil)
	_ action.ActionWithConfigure = (*snapshotAction)(nil)
)

type snapshotActionModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Restore   types.Bool   `tfsdk:"restore"`
}

func NewSnapshotAction() action.Action { return &snapshotAction{} }

func (a *snapshotAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot"
}

func (a *snapshotAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a snapshot of a GNS3 project, or restores one by name. " +
			"With `restore = false` (the default) it calls `POST /v2/projects/{project_id}/snapshots` with `name`. " +
			"With `restore = true` it lists the project's snapshots, picks the one called `name` and calls " +
			"`POST /v2/projects/{project_id}/snapshots/{snapshot_id}/restore`; it fails when no snapshot or more " +
			"than one snapshot has that name. The action changes no state and has no outputs.\n\n" +
			"Actions need Terraform 1.14 or later. Invoke it from a resource's `lifecycle { action_trigger { ... } }` " +
			"block or with `terraform apply -invoke=action.gns3_snapshot.<name>`.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the project to snapshot or restore.",
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Snapshot name. GNS3 does not require names to be unique, but a restore " +
					"needs exactly one snapshot with this name.",
			},
			"restore": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "`false` (default) creates a snapshot called `name`. `true` restores the " +
					"existing snapshot called `name`, replacing the project's current topology. On GNS3 2.2.61 a " +
					"restore leaves every node stopped (node IDs are kept); start them again with `gns3_start_nodes`.",
			},
		},
	}
}

func (a *snapshotAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (a *snapshotAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var m snapshotActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID, name := m.ProjectID.ValueString(), m.Name.ValueString()

	if !m.Restore.ValueBool() {
		s, err := a.client.CreateSnapshot(ctx, projectID, name)
		if err != nil {
			resp.Diagnostics.AddError("Error creating GNS3 snapshot", err.Error())
			return
		}
		sendProgress(resp, fmt.Sprintf("created snapshot %q (%s) in project %s", name, s.SnapshotID, projectID))
		return
	}

	snaps, err := a.client.ListSnapshots(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error listing GNS3 snapshots", err.Error())
		return
	}
	var ids []string
	for _, s := range snaps {
		if s.Name == name {
			ids = append(ids, s.SnapshotID)
		}
	}
	switch len(ids) {
	case 0:
		resp.Diagnostics.AddError("GNS3 snapshot not found",
			fmt.Sprintf("Project %s has no snapshot named %q, so there is nothing to restore.", projectID, name))
		return
	case 1:
	default:
		resp.Diagnostics.AddError("GNS3 snapshot name is ambiguous",
			fmt.Sprintf("Project %s has %d snapshots named %q (%s). Delete all but one in GNS3, then restore again.",
				projectID, len(ids), name, strings.Join(ids, ", ")))
		return
	}
	if err := a.client.RestoreSnapshot(ctx, projectID, ids[0]); err != nil {
		resp.Diagnostics.AddError("Error restoring GNS3 snapshot", err.Error())
		return
	}
	sendProgress(resp, fmt.Sprintf("restored snapshot %q (%s) in project %s", name, ids[0], projectID))
}
