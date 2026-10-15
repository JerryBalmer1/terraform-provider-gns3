package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/JerryBalmer1/terraform-provider-gns3/internal/client"
)

// linkConfig links pc1 to `peer` (pc2 or pc3); extra is spliced into the link.
func linkConfig(project, peer, extra string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.test.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"
}

resource "gns3_node" "pc2" {
  project_id  = gns3_project.test.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC2"
  x           = 200
}

resource "gns3_node" "pc3" {
  project_id  = gns3_project.test.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC3"
  x           = 400
}

resource "gns3_link" "test" {
  project_id = gns3_project.test.project_id

  node {
    node_id        = gns3_node.pc1.node_id
    adapter_number = gns3_node.pc1.ports[0].adapter_number
    port_number    = gns3_node.pc1.ports[0].port_number
  }

  node {
    node_id        = gns3_node.%[2]s.node_id
    adapter_number = gns3_node.%[2]s.ports[0].adapter_number
    port_number    = gns3_node.%[2]s.ports[0].port_number
  }
%[3]s
}
`, project, peer, extra)
}

func linkTestCase(env testEnv) resource.TestCase {
	project := acctest.RandomWithPrefix("tf-acc-link")
	const addr = "gns3_link.test"
	filters := `
  suspend = true
  filters = jsonencode({ frequency_drop = [50] })
`
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_link"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + linkConfig(project, "pc2", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "link_id"),
					resource.TestCheckResourceAttr(addr, "link_type", "ethernet"),
					resource.TestCheckResourceAttr(addr, "suspend", "false"),
					resource.TestCheckNoResourceAttr(addr, "filters"),
					resource.TestCheckResourceAttr(addr, "node.#", "2"),
					resource.TestCheckResourceAttrPair(addr, "node.0.node_id", "gns3_node.pc1", "node_id"),
					resource.TestCheckResourceAttrPair(addr, "node.1.node_id", "gns3_node.pc2", "node_id"),
				),
			},
			{
				Config: env.providerBlock + linkConfig(project, "pc2", filters),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "suspend", "true"),
					resource.TestCheckResourceAttr(addr, "filters", `{"frequency_drop":[50]}`),
				),
			},
			{
				// Removing filters from configuration clears them on the server.
				Config: env.providerBlock + linkConfig(project, "pc2", "  suspend = true\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckNoResourceAttr(addr, "filters"),
			},
			{
				ResourceName:                         addr,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    attrFromState(addr, "project_id", "link_id"),
				ImportStateVerifyIdentifierAttribute: "link_id",
			},
			{
				// Moving one end to another node forces a new link.
				Config: env.providerBlock + linkConfig(project, "pc3", "  suspend = true\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionDestroyBeforeCreate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttrPair(addr, "node.1.node_id", "gns3_node.pc3", "node_id"),
			},
		},
	}
}

func TestLinkResource(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, linkTestCase(env))
	if p, n, l := env.fake.counts(); p+n+l != 0 {
		t.Errorf("fake still holds %d projects, %d nodes, %d links after destroy", p, n, l)
	}
}

func TestAccLinkResource(t *testing.T) {
	resource.Test(t, linkTestCase(accEnv(t)))
}

func TestLinkResourceNeedsTwoNodes(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config: env.providerBlock + `
resource "gns3_link" "bad" {
  project_id = "p"
  node {
    node_id        = "n"
    adapter_number = 0
    port_number    = 0
  }
}
`,
			ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value.*list must contain at least 2 elements and at\s+most 2 elements`),
		}},
	})
}

func TestSameEndsIgnoresOrder(t *testing.T) {
	ends := []linkEndModel{
		{NodeID: types.StringValue("a"), AdapterNumber: types.Int64Value(0), PortNumber: types.Int64Value(1)},
		{NodeID: types.StringValue("b"), AdapterNumber: types.Int64Value(0), PortNumber: types.Int64Value(0)},
	}
	swapped := []client.LinkNode{{NodeID: "b"}, {NodeID: "a", PortNumber: 1}}
	if !sameEnds(ends, swapped) {
		t.Error("swapped ends reported different")
	}
	if sameEnds(ends, []client.LinkNode{{NodeID: "a", PortNumber: 1}, {NodeID: "c"}}) {
		t.Error("different ends reported same")
	}
}

func TestFiltersFromAPI(t *testing.T) {
	null := jsontypes.NewNormalizedNull()
	if got := filtersFromAPI(null, json.RawMessage(`{}`)); !got.IsNull() {
		t.Errorf("{} with null state = %v, want null", got)
	}
	if got := filtersFromAPI(null, json.RawMessage(`{"latency":[10]}`)); got.ValueString() != `{"latency":[10]}` {
		t.Errorf("server drift not reported: %v", got)
	}
	set := jsontypes.NewNormalizedValue(`{"latency":[10]}`)
	if got := filtersFromAPI(set, json.RawMessage(`{}`)); got.ValueString() != `{}` {
		t.Errorf("filters cleared on server not reported: %v", got)
	}
}
