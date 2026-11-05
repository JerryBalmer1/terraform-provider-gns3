package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// nodesActionConfig: a project and two template-less VPCS nodes. pc2 depends
// on pc1, so pc2's after_create trigger runs once both nodes exist. action is
// the action block; startOnCreate starts both nodes at create.
func nodesActionConfig(project string, startOnCreate bool, action, ref string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %[1]q
}

resource "gns3_node" "pc1" {
  project_id      = gns3_project.test.project_id
  node_type       = "vpcs"
  compute_id      = "local"
  name            = "PC1"
  start_on_create = %[2]t
}

resource "gns3_node" "pc2" {
  project_id      = gns3_project.test.project_id
  node_type       = "vpcs"
  compute_id      = "local"
  name            = "PC2"
  x               = 100
  start_on_create = %[2]t
  depends_on      = [gns3_node.pc1]

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [%[4]s]
    }
  }
}

%[3]s
`, project, startOnCreate, action, ref)
}

// checkNodeStatus compares a node's status on the server (the fake, or the
// live server through the API) with want.
func (e testEnv) checkNodeStatus(name, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		pid, nid := rs.Primary.Attributes["project_id"], rs.Primary.Attributes["node_id"]
		var got string
		if e.fake != nil {
			got = e.fake.nodeStatus(pid, nid)
		} else {
			n, err := e.client.GetNode(context.Background(), pid, nid)
			if err != nil {
				return err
			}
			got = n.Status
		}
		if got != want {
			return fmt.Errorf("%s: status on server is %q, want %q", name, got, want)
		}
		return nil
	}
}

func nodesActionTestCase(env testEnv, prefix string, startOnCreate bool, action, ref string, checks ...resource.TestCheckFunc) resource.TestCase {
	project := acctest.RandomWithPrefix(prefix)
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_node"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + nodesActionConfig(project, startOnCreate, action, ref),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
		},
	}
}

const startAllAction = `
action "gns3_start_nodes" "all" {
  config {
    project_id = gns3_project.test.project_id
  }
}
`

const startSubsetAction = `
action "gns3_start_nodes" "pc1" {
  config {
    project_id = gns3_project.test.project_id
    node_ids   = [gns3_node.pc1.node_id]
  }
}
`

const stopSubsetAction = `
action "gns3_stop_nodes" "pc1" {
  config {
    project_id = gns3_project.test.project_id
    node_ids   = [gns3_node.pc1.node_id]
  }
}
`

// Reload every node, then stop every node: both bulk endpoints in one trigger.
const reloadThenStopAction = `
action "gns3_reload_nodes" "all" {
  config {
    project_id = gns3_project.test.project_id
  }
}

action "gns3_stop_nodes" "all" {
  config {
    project_id = gns3_project.test.project_id
    node_ids   = []
  }
}
`

func startAllTestCase(env testEnv) resource.TestCase {
	return nodesActionTestCase(env, "tf-acc-start-all", false, startAllAction, "action.gns3_start_nodes.all",
		env.checkNodeStatus("gns3_node.pc1", "started"), env.checkNodeStatus("gns3_node.pc2", "started"))
}

func startSubsetTestCase(env testEnv) resource.TestCase {
	return nodesActionTestCase(env, "tf-acc-start-subset", false, startSubsetAction, "action.gns3_start_nodes.pc1",
		env.checkNodeStatus("gns3_node.pc1", "started"), env.checkNodeStatus("gns3_node.pc2", "stopped"))
}

func stopSubsetTestCase(env testEnv) resource.TestCase {
	return nodesActionTestCase(env, "tf-acc-stop-subset", true, stopSubsetAction, "action.gns3_stop_nodes.pc1",
		env.checkNodeStatus("gns3_node.pc1", "stopped"), env.checkNodeStatus("gns3_node.pc2", "started"))
}

func reloadThenStopTestCase(env testEnv) resource.TestCase {
	return nodesActionTestCase(env, "tf-acc-reload-stop", true, reloadThenStopAction,
		"action.gns3_reload_nodes.all, action.gns3_stop_nodes.all",
		env.checkNodeStatus("gns3_node.pc1", "stopped"), env.checkNodeStatus("gns3_node.pc2", "stopped"))
}

func TestStartNodesActionAll(t *testing.T)    { resource.UnitTest(t, startAllTestCase(unitEnv(t))) }
func TestStartNodesActionSubset(t *testing.T) { resource.UnitTest(t, startSubsetTestCase(unitEnv(t))) }
func TestStopNodesActionSubset(t *testing.T)  { resource.UnitTest(t, stopSubsetTestCase(unitEnv(t))) }
func TestReloadStopNodesActionAll(t *testing.T) {
	resource.UnitTest(t, reloadThenStopTestCase(unitEnv(t)))
}

const suspendSubsetAction = `
action "gns3_suspend_nodes" "pc2" {
  config {
    project_id = gns3_project.test.project_id
    node_ids   = [gns3_node.pc2.node_id]
  }
}
`

// suspendSubsetTestCase suspends pc2 only. VPCS has no suspend: 2.2.61
// answers 200 and the node stays started (docs/api-drift.md), so the action
// must succeed, both nodes end started, and the action warns about pc2.
func suspendSubsetTestCase(env testEnv) resource.TestCase {
	return nodesActionTestCase(env, "tf-acc-suspend", true, suspendSubsetAction, "action.gns3_suspend_nodes.pc2",
		env.checkNodeStatus("gns3_node.pc1", "started"), env.checkNodeStatus("gns3_node.pc2", "started"),
		env.checkSuspendWarning("gns3_node.pc2", "PC2"))
}

// checkSuspendWarning invokes gns3_suspend_nodes for one node directly, through
// the test's client, and requires exactly the "no effect" warning naming the
// node and its type. plugin-testing has no check for warning diagnostics, so
// the warning of the Terraform-run invocation can't be asserted; this repeats
// the same Invoke (on VPCS a second suspend changes nothing either).
func (e testEnv) checkSuspendWarning(name, nodeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		pid, nid := rs.Primary.Attributes["project_id"], rs.Primary.Attributes["node_id"]
		ctx := context.Background()
		a := NewSuspendNodesAction().(*nodesAction)
		a.client = e.client
		var sr action.SchemaResponse
		a.Schema(ctx, action.SchemaRequest{}, &sr)
		raw := tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), map[string]tftypes.Value{
			"project_id": tftypes.NewValue(tftypes.String, pid),
			"node_ids":   tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, nid)}),
		})
		var resp action.InvokeResponse
		a.Invoke(ctx, action.InvokeRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: raw}}, &resp)
		if resp.Diagnostics.HasError() {
			return fmt.Errorf("suspend returned errors: %v", resp.Diagnostics)
		}
		warnings := resp.Diagnostics.Warnings()
		want := fmt.Sprintf("suspend had no effect on %s (vpcs); 2.2.61 accepts the call but does not suspend this type", nodeName)
		if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), want) {
			return fmt.Errorf("want one warning containing %q, got %v", want, warnings)
		}
		return nil
	}
}

func TestSuspendNodesActionSubset(t *testing.T) {
	resource.UnitTest(t, suspendSubsetTestCase(unitEnv(t)))
}
func TestAccSuspendNodesActionSubset(t *testing.T) {
	resource.Test(t, suspendSubsetTestCase(accEnv(t)))
}

func TestAccStartNodesActionAll(t *testing.T)    { resource.Test(t, startAllTestCase(accEnv(t))) }
func TestAccStartNodesActionSubset(t *testing.T) { resource.Test(t, startSubsetTestCase(accEnv(t))) }
func TestAccStopNodesActionSubset(t *testing.T)  { resource.Test(t, stopSubsetTestCase(accEnv(t))) }
func TestAccReloadStopNodesActionAll(t *testing.T) {
	resource.Test(t, reloadThenStopTestCase(accEnv(t)))
}
