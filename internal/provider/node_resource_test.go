package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// nodeConfig: pc1 from a built-in template, pc2 template-less with properties.
func nodeConfig(project, template string, pc1X int, pc2Name, script string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

data "gns3_template" "t" {
  name = %q
}

resource "gns3_node" "pc1" {
  project_id      = gns3_project.test.project_id
  template_id     = data.gns3_template.t.template_id
  name            = "PC1"
  x               = %d
  y               = 0
  start_on_create = true
}

resource "gns3_node" "pc2" {
  project_id = gns3_project.test.project_id
  node_type  = "vpcs"
  compute_id = "local"
  name       = %q
  x          = 100
  properties = jsonencode({ startup_script = %q })
}
`, project, template, pc1X, pc2Name, script)
}

func nodeTestCase(env testEnv) resource.TestCase {
	project := acctest.RandomWithPrefix("tf-acc-node")
	empty := resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_node"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{
				Config:           env.providerBlock + nodeConfig(project, "VPCS", -100, "PC2", "set pcname tfacc"),
				ConfigPlanChecks: empty,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.pc1", "node_type", "vpcs"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "compute_id", "local"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "status", "started"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "x", "-100"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.#", "1"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.0.adapter_number", "0"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.0.port_number", "0"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.0.name", "Ethernet0"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.0.link_type", "ethernet"),
					resource.TestCheckResourceAttrSet("gns3_node.pc1", "node_id"),
					resource.TestCheckResourceAttrSet("gns3_node.pc1", "console"),
					resource.TestCheckResourceAttrSet("gns3_node.pc1", "symbol"),
					resource.TestCheckResourceAttrPair("gns3_node.pc1", "template_id", "data.gns3_template.t", "template_id"),
					resource.TestCheckResourceAttr("gns3_node.pc2", "status", "stopped"),
					resource.TestCheckNoResourceAttr("gns3_node.pc2", "template_id"),
					resource.TestCheckResourceAttr("gns3_node.pc2", "ports.0.name", "Ethernet0"),
				),
			},
			{
				// Move pc1, rename pc2 and change its startup script: all in place.
				Config: env.providerBlock + nodeConfig(project, "VPCS", 50, "PC2-renamed", "set pcname tfacc2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("gns3_node.pc1", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("gns3_node.pc2", plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.pc1", "x", "50"),
					resource.TestCheckResourceAttr("gns3_node.pc2", "name", "PC2-renamed"),
					resource.TestCheckResourceAttr("gns3_node.pc2", "properties", `{"startup_script":"set pcname tfacc2"}`),
					env.checkNodeFile("gns3_node.pc2", "startup.vpc", "set pcname tfacc2"),
				),
			},
			{
				ResourceName:                         "gns3_node.pc1",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    attrFromState("gns3_node.pc1", "project_id", "node_id"),
				ImportStateVerifyIdentifierAttribute: "node_id",
				// start_on_create only acts at create; an import cannot know it.
				ImportStateVerifyIgnore: []string{"start_on_create"},
			},
			{
				ResourceName:                         "gns3_node.pc2",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    attrFromState("gns3_node.pc2", "project_id", "node_id"),
				ImportStateVerifyIdentifierAttribute: "node_id",
				// Which properties keys are managed comes from configuration.
				ImportStateVerifyIgnore: []string{"properties"},
			},
			{
				// A different template forces a new node.
				Config: env.providerBlock + nodeConfig(project, "Ethernet hub", 50, "PC2-renamed", "set pcname tfacc2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("gns3_node.pc1", plancheck.ResourceActionDestroyBeforeCreate),
						plancheck.ExpectResourceAction("gns3_node.pc2", plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.pc1", "node_type", "ethernet_hub"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "ports.#", "8"),
				),
			},
		},
	}
}

func TestNodeResource(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, nodeTestCase(env))
	if p, n, l := env.fake.counts(); p+n+l != 0 {
		t.Errorf("fake still holds %d projects, %d nodes, %d links after destroy", p, n, l)
	}
}

func TestAccNodeResource(t *testing.T) {
	resource.Test(t, nodeTestCase(accEnv(t)))
}

// TestNodeResourceClosedProject: a closed project hides its nodes; refresh
// must fail loudly instead of dropping the node from state.
func TestNodeResourceClosedProject(t *testing.T) {
	env := unitEnv(t)
	project := acctest.RandomWithPrefix("tf-unit-closed")
	var projectID string
	cfg := env.providerBlock + nodeConfig(project, "VPCS", 0, "PC2", "set pcname tfacc")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: func(s *terraform.State) error {
					projectID = s.RootModule().Resources["gns3_project.test"].Primary.Attributes["project_id"]
					return nil
				},
			},
			{
				PreConfig:   func() { env.fake.closeProject(projectID) },
				Config:      cfg,
				ExpectError: regexp.MustCompile(`GNS3 project .* is closed`),
			},
			{
				PreConfig: func() {
					if _, err := env.client.OpenProject(t.Context(), projectID); err != nil {
						t.Fatal(err)
					}
				},
				Config: cfg,
			},
		},
	})
}

func TestNodeConfigValidation(t *testing.T) {
	env := unitEnv(t)
	base := env.providerBlock + `
resource "gns3_project" "test" {
  name = "validation"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: base + `
resource "gns3_node" "n" {
  project_id = gns3_project.test.project_id
  name       = "neither"
}
`,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured: \[template_id,node_type\]`),
			},
			{
				Config: base + `
resource "gns3_node" "n" {
  project_id  = gns3_project.test.project_id
  template_id = "19021f99-e36f-394d-b4a1-8aaa902ab9cc"
  node_type   = "vpcs"
  name        = "both"
}
`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config: base + `
resource "gns3_node" "n" {
  project_id  = gns3_project.test.project_id
  template_id = "19021f99-e36f-394d-b4a1-8aaa902ab9cc"
  name        = "props"
  properties  = jsonencode({ ram = 256 })
}
`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func TestReadPropertiesKeepsOnlyConfiguredKeys(t *testing.T) {
	var diags diag.Diagnostics
	server := map[string]json.RawMessage{
		"startup_script":   json.RawMessage(`"set pcname tfacc"`),
		"base_script_file": json.RawMessage(`"vpcs_base_config.txt"`),
	}
	got := readProperties(jsontypes.NewNormalizedValue(`{"startup_script":"old"}`), server, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got.ValueString() != `{"startup_script":"set pcname tfacc"}` {
		t.Fatalf("got %s", got.ValueString())
	}
	if null := readProperties(jsontypes.NewNormalizedNull(), server, &diags); !null.IsNull() {
		t.Fatalf("null properties became %s", null.ValueString())
	}
}

// checkNodeFile reads a file from the node's directory through the API.
func (e testEnv) checkNodeFile(name, file, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		got, err := e.client.ReadNodeFile(context.Background(), rs.Primary.Attributes["project_id"], rs.Primary.Attributes["node_id"], file)
		if err != nil {
			return err
		}
		if string(got) != want {
			return fmt.Errorf("%s %s = %q, want %q", name, file, got, want)
		}
		return nil
	}
}

func TestSplitPropertiesMovesFileBackedKeys(t *testing.T) {
	var diags diag.Diagnostics
	props := map[string]json.RawMessage{"startup_script": json.RawMessage(`"set pcname x"`), "console": json.RawMessage(`5000`)}
	rest, files := splitProperties("vpcs", props, &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(rest) != 1 || string(rest["console"]) != "5000" || len(files) != 1 || files["startup.vpc"] != "set pcname x" {
		t.Fatalf("rest %v, files %v", rest, files)
	}
	if rest, files := splitProperties("qemu", props, &diags); len(rest) != 2 || len(files) != 0 {
		t.Fatalf("qemu: rest %v, files %v", rest, files)
	}
	splitProperties("vpcs", map[string]json.RawMessage{"startup_script": json.RawMessage(`42`)}, &diags)
	if !diags.HasError() {
		t.Fatal("non-string startup_script accepted")
	}
}
