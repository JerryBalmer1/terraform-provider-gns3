package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

// nodesAction is gns3_start_nodes, gns3_stop_nodes, gns3_suspend_nodes and
// gns3_reload_nodes: one type, parameterised by the verb and the two client
// calls (bulk for the whole project, per node for a node_ids list).
type nodesAction struct {
	client gns3.Client

	verb    string // "start", "stop", "suspend", "reload"
	past    string // "started", ... for progress messages
	all     func(c gns3.Client, ctx context.Context, projectID string) error
	perNode func(c gns3.Client, ctx context.Context, projectID, nodeID string) error
	note    string // extra paragraph for the description, "" for none
}

var (
	_ action.Action              = (*nodesAction)(nil)
	_ action.ActionWithConfigure = (*nodesAction)(nil)
)

type nodesActionModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	NodeIDs   types.Set    `tfsdk:"node_ids"`
}

func NewStartNodesAction() action.Action {
	return &nodesAction{
		verb: "start", past: "started",
		all: func(c gns3.Client, ctx context.Context, pid string) error { return c.StartAllNodes(ctx, pid) },
		perNode: func(c gns3.Client, ctx context.Context, pid, nid string) error {
			_, err := c.StartNode(ctx, pid, nid)
			return err
		},
	}
}

func NewStopNodesAction() action.Action {
	return &nodesAction{
		verb: "stop", past: "stopped",
		all: func(c gns3.Client, ctx context.Context, pid string) error { return c.StopAllNodes(ctx, pid) },
		perNode: func(c gns3.Client, ctx context.Context, pid, nid string) error {
			_, err := c.StopNode(ctx, pid, nid)
			return err
		},
	}
}

func NewSuspendNodesAction() action.Action {
	return &nodesAction{
		verb: "suspend", past: "suspended",
		all:     func(c gns3.Client, ctx context.Context, pid string) error { return c.SuspendAllNodes(ctx, pid) },
		perNode: func(c gns3.Client, ctx context.Context, pid, nid string) error { return c.SuspendNode(ctx, pid, nid) },
		note: "VPCS has no suspend: GNS3 2.2.61 accepts the call and the node keeps running " +
			"(see `docs/api-drift.md`, \"Node actions and snapshots (2.2.61)\").",
	}
}

func NewReloadNodesAction() action.Action {
	return &nodesAction{
		verb: "reload", past: "reloaded",
		all:     func(c gns3.Client, ctx context.Context, pid string) error { return c.ReloadAllNodes(ctx, pid) },
		perNode: func(c gns3.Client, ctx context.Context, pid, nid string) error { return c.ReloadNode(ctx, pid, nid) },
		note:    "On GNS3 2.2.61, reloading a stopped VPCS node starts it.",
	}
}

func (a *nodesAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + a.verb + "_nodes"
}

func (a *nodesAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	desc := fmt.Sprintf("Sends %s to GNS3 nodes in one project. Without `node_ids` it makes one "+
		"`POST /v2/projects/{project_id}/nodes/%s` call for every node in the project; with `node_ids` it makes "+
		"one `POST /v2/projects/{project_id}/nodes/{node_id}/%s` call per node, in sorted order, and stops at "+
		"the first error. The action changes no state and has no outputs.\n\n"+
		"Actions need Terraform 1.14 or later. Invoke it from a resource's `lifecycle { action_trigger { ... } }` "+
		"block or with `terraform apply -invoke=action.gns3_%s_nodes.<name>`.",
		a.verb, a.verb, a.verb, a.verb)
	if a.note != "" {
		desc += "\n\n" + a.note
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: desc,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the project whose nodes to " + a.verb + ". The project must be open.",
			},
			"node_ids": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				MarkdownDescription: "IDs of the nodes to " + a.verb + ". Null or empty means every node in the " +
					"project, through the project-wide endpoint.",
			},
		},
	}
}

func (a *nodesAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (a *nodesAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var m nodesActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := m.ProjectID.ValueString()
	var nodeIDs []string
	if !m.NodeIDs.IsNull() && !m.NodeIDs.IsUnknown() {
		resp.Diagnostics.Append(m.NodeIDs.ElementsAs(ctx, &nodeIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if len(nodeIDs) == 0 {
		if err := a.all(a.client, ctx, projectID); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error trying to %s GNS3 nodes", a.verb), err.Error())
			return
		}
		sendProgress(resp, fmt.Sprintf("%s all nodes in project %s", a.past, projectID))
		return
	}

	slices.Sort(nodeIDs)
	for _, id := range nodeIDs {
		if err := a.perNode(a.client, ctx, projectID, id); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error trying to %s GNS3 node %s", a.verb, id), err.Error())
			return
		}
		sendProgress(resp, fmt.Sprintf("%s node %s in project %s", a.past, id, projectID))
	}
}
