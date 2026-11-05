package provider

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	// checkSuspended re-reads the targeted nodes after the call and warns
	// about any whose status did not change (VPCS on 2.2.61, docs/api-drift.md).
	checkSuspended bool
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
			"(see `docs/api-drift.md`, \"Node actions and snapshots (2.2.61)\"). After the call the action " +
			"re-reads each targeted node and warns about any whose status did not change.",
		checkSuspended: true,
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

	var before map[string]gns3.Node
	if a.checkSuspended {
		var err error
		if before, err = a.readNodes(ctx, projectID, nodeIDs); err != nil {
			resp.Diagnostics.AddWarning("Cannot check the GNS3 suspend",
				"Reading the nodes before the suspend failed, so the action cannot tell whether it had an effect: "+err.Error())
		}
	}

	if len(nodeIDs) == 0 {
		if err := a.all(a.client, ctx, projectID); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error trying to %s GNS3 nodes", a.verb), err.Error())
			return
		}
		sendProgress(resp, fmt.Sprintf("%s all nodes in project %s", a.past, projectID))
	} else {
		slices.Sort(nodeIDs)
		for _, id := range nodeIDs {
			if err := a.perNode(a.client, ctx, projectID, id); err != nil {
				resp.Diagnostics.AddError(fmt.Sprintf("Error trying to %s GNS3 node %s", a.verb, id), err.Error())
				return
			}
			sendProgress(resp, fmt.Sprintf("%s node %s in project %s", a.past, id, projectID))
		}
	}

	if before != nil {
		a.warnUnsuspended(ctx, projectID, nodeIDs, before, &resp.Diagnostics)
	}
}

// readNodes reads the nodes an action targets: every node in the project
// (GET /v2/projects/{project_id}/nodes) when ids is empty, else each node
// (GET .../nodes/{node_id}). The node action responses can't be used, because
// 2.2.61 answers with the node as it was before the call (docs/api-drift.md).
func (a *nodesAction) readNodes(ctx context.Context, projectID string, ids []string) (map[string]gns3.Node, error) {
	out := map[string]gns3.Node{}
	if len(ids) == 0 {
		nodes, err := a.client.ListNodes(ctx, projectID)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			out[n.NodeID] = n
		}
		return out, nil
	}
	for _, id := range ids {
		n, err := a.client.GetNode(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		out[id] = *n
	}
	return out, nil
}

// warnUnsuspended re-reads the targeted nodes and warns about every node whose
// status the suspend left unchanged. Nodes that were already suspended are
// skipped: for them an unchanged status is the expected result.
func (a *nodesAction) warnUnsuspended(ctx context.Context, projectID string, ids []string, before map[string]gns3.Node, diags *diag.Diagnostics) {
	after, err := a.readNodes(ctx, projectID, ids)
	if err != nil {
		diags.AddWarning("Cannot check the GNS3 suspend",
			"Re-reading the nodes after the suspend failed, so the action cannot tell whether it had an effect: "+err.Error())
		return
	}
	for _, id := range slices.Sorted(maps.Keys(after)) {
		n := after[id]
		b, ok := before[id]
		if !ok || b.Status == "suspended" || n.Status != b.Status {
			continue
		}
		diags.AddWarning(fmt.Sprintf("GNS3 suspend had no effect on %s", n.Name),
			fmt.Sprintf("suspend had no effect on %s (%s); 2.2.61 accepts the call but does not suspend this type. "+
				"Node %s is still %q.", n.Name, n.NodeType, id, n.Status))
	}
}
