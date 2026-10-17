package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3"
)

func TestStartupConfigDispatch(t *testing.T) {
	cases := []struct {
		name, nodeType, dynamipsID, want, wantErr string
	}{
		{name: "vpcs", nodeType: "vpcs", want: "startup.vpc"},
		{name: "dynamips", nodeType: "dynamips", dynamipsID: "3", want: "configs/i3_startup-config.cfg"},
		{name: "dynamips no id", nodeType: "dynamips", wantErr: "reports no dynamips_id"},
		{name: "dynamips bad id", nodeType: "dynamips", dynamipsID: `"x"`, wantErr: "reports no dynamips_id"},
		{name: "qemu", nodeType: "qemu", wantErr: "config disks are not supported on Windows computes"},
		{name: "iou", nodeType: "iou", wantErr: "iou is not supported yet"},
		{name: "docker", nodeType: "docker", wantErr: "docker is not supported yet"},
		{name: "switch", nodeType: "ethernet_switch", wantErr: "a ethernet_switch node has no startup configuration"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := &gns3.Node{NodeID: "n1", NodeType: c.nodeType, Properties: map[string]json.RawMessage{}}
			if c.dynamipsID != "" {
				n.Properties["dynamips_id"] = json.RawMessage(c.dynamipsID)
			}
			got, err := startupConfigFile(n)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got %q, %v; want error containing %q", got, err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestCheckStartupConfigTypeCoversEveryNodeType(t *testing.T) {
	for _, nodeType := range nodeTypes {
		var diags diag.Diagnostics
		checkStartupConfigType(nodeType, &diags)
		supported := nodeType == "vpcs" || nodeType == "dynamips"
		if diags.HasError() == supported {
			t.Errorf("%s: supported = %v, diags = %v", nodeType, supported, diags)
		}
	}
}

func TestCheckStartupConfigTypeQemuMessage(t *testing.T) {
	var diags diag.Diagnostics
	checkStartupConfigType("qemu", &diags)
	if !diags.HasError() {
		t.Fatal("qemu startup_config accepted")
	}
	want := "startup_config is supported on vpcs and dynamips nodes only: " +
		"GNS3 2.2 config disks are not supported on Windows computes and are not read by VyOS; " +
		"use Ansible after boot, or cloud-init where the image supports it. " +
		"See https://github.com/JerryBalmer1/terraform-provider-gns3/blob/main/docs/api-drift.md#qemu-config-disk-2261."
	if got := diags.Errors()[0].Detail(); got != want {
		t.Fatalf("detail = %q\nwant     %q", got, want)
	}
}

func TestSplitPropertiesRefusesDynamipsConfigs(t *testing.T) {
	for _, k := range []string{"startup_config_content", "private_config_content"} {
		var diags diag.Diagnostics
		props := map[string]json.RawMessage{k: json.RawMessage(`"hostname R1"`), "ram": json.RawMessage(`256`)}
		rest, _ := splitProperties("dynamips", props, &diags)
		if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), k) {
			t.Fatalf("%s: diags = %v", k, diags)
		}
		if _, sent := rest[k]; sent || string(rest["ram"]) != "256" {
			t.Fatalf("%s: rest = %v", k, rest)
		}
	}
}

func TestNodeStartupConfigValidation(t *testing.T) {
	env := unitEnv(t)
	base := env.providerBlock + `
resource "gns3_project" "test" {
  name = "validation"
}
`
	node := func(body string) string {
		return base + "resource \"gns3_node\" \"n\" {\n  project_id = gns3_project.test.project_id\n  name = \"n\"\n" + body + "\n}\n"
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config:      node(`node_type = "qemu"` + "\n" + `startup_config = "x"`),
				ExpectError: regexp.MustCompile(`startup_config not supported for node type qemu`),
			},
			{
				Config:      node(`node_type = "iou"` + "\n" + `startup_config = "x"`),
				ExpectError: regexp.MustCompile(`startup_config not supported for node type iou`),
			},
			{
				// The template's type is looked up at plan time.
				Config:      node(`template_id = "b4503ea9-d6b6-3695-9fe4-1db3b39290b0"` + "\n" + `startup_config = "x"`),
				ExpectError: regexp.MustCompile(`startup_config not supported for node type ethernet_hub`),
			},
			{
				Config: node(`node_type = "vpcs"` + "\n" + `startup_config = "set pcname a"` + "\n" +
					`properties = jsonencode({ startup_script = "set pcname b" })`),
				ExpectError: regexp.MustCompile(`Conflicting startup configuration`),
			},
			{
				Config: node(`node_type = "dynamips"` + "\n" +
					`properties = jsonencode({ platform = "c7200", image = "c7200.image", startup_config_content = "hostname R1" })`),
				ExpectError: regexp.MustCompile(`Unsupported node property`),
			},
		},
	})
	if p, n, l := env.fake.counts(); p+n+l != 0 {
		t.Errorf("validation left %d projects, %d nodes, %d links", p, n, l)
	}
}

// checkStartupConfig reads the node's startup file, wherever its node type
// keeps it, through the API.
func (e testEnv) checkStartupConfig(name, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		ctx := context.Background()
		n, err := e.client.GetNode(ctx, rs.Primary.Attributes["project_id"], rs.Primary.Attributes["node_id"])
		if err != nil {
			return err
		}
		file, err := startupConfigFile(n)
		if err != nil {
			return err
		}
		got, err := e.client.ReadNodeFile(ctx, n.ProjectID, n.NodeID, file)
		if err != nil {
			return err
		}
		if string(got) != want {
			return fmt.Errorf("%s %s = %q, want %q", name, file, got, want)
		}
		return nil
	}
}

func vpcsStartupConfig(project, startup string) string {
	line := ""
	if startup != "" {
		line = fmt.Sprintf("startup_config  = %q", startup)
	}
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc" {
  project_id      = gns3_project.test.project_id
  template_id     = data.gns3_template.vpcs.template_id
  name            = "PC1"
  start_on_create = true
  %s
}
`, project, line)
}

func nodeStartupConfigTestCase(env testEnv) resource.TestCase {
	project := acctest.RandomWithPrefix("tf-acc-startup")
	first := "set pcname PC1\nip 10.0.0.1/24 10.0.0.254\n"
	second := "set pcname PC1\nip 10.0.0.2/24 10.0.0.254\n"
	var projectID, nodeID string
	empty := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	update := []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_node.pc", plancheck.ResourceActionUpdate)}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_node"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{
				// Written before start_on_create starts the node.
				Config:           env.providerBlock + vpcsStartupConfig(project, first),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: empty},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.pc", "startup_config", first),
					resource.TestCheckResourceAttr("gns3_node.pc", "status", "started"),
					env.checkStartupConfig("gns3_node.pc", first),
				),
			},
			{
				// A change is written in place; the running node is not restarted.
				Config:           env.providerBlock + vpcsStartupConfig(project, second),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: update, PostApplyPostRefresh: empty},
				Check: resource.ComposeAggregateTestCheckFunc(
					env.checkStartupConfig("gns3_node.pc", second),
					resource.TestCheckResourceAttr("gns3_node.pc", "status", "started"),
					func(s *terraform.State) error {
						a := s.RootModule().Resources["gns3_node.pc"].Primary.Attributes
						projectID, nodeID = a["project_id"], a["node_id"]
						return nil
					},
				),
			},
			{
				// An edit on the server is drift, and apply writes the file back.
				PreConfig: func() {
					if err := env.client.WriteNodeFile(context.Background(), projectID, nodeID, "startup.vpc", []byte("set pcname edited\n")); err != nil {
						panic(err)
					}
				},
				Config:           env.providerBlock + vpcsStartupConfig(project, second),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: update, PostApplyPostRefresh: empty},
				Check:            env.checkStartupConfig("gns3_node.pc", second),
			},
			{
				// Removing the attribute stops managing the file and leaves it.
				Config:           env.providerBlock + vpcsStartupConfig(project, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: empty},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("gns3_node.pc", "startup_config"),
					env.checkStartupConfig("gns3_node.pc", second),
				),
			},
		},
	}
}

func TestNodeStartupConfig(t *testing.T) {
	resource.UnitTest(t, nodeStartupConfigTestCase(unitEnv(t)))
}

func TestAccNodeStartupConfig(t *testing.T) {
	resource.Test(t, nodeStartupConfigTestCase(accEnv(t)))
}

func dynamipsStartupConfig(project, source, name, startup string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

resource "gns3_node" "r" {
  project_id     = gns3_project.test.project_id
  %s
  name           = %q
  startup_config = %q
}
`, project, source, name, startup)
}

// dynamipsStartupConfigTestCase creates a dynamips router (never started) from
// source, then renames it and changes its config in one update. On 2.2.61 the
// rename fails if the config was ever sent as a property.
func dynamipsStartupConfigTestCase(env testEnv, source string) resource.TestCase {
	project := acctest.RandomWithPrefix("tf-acc-startup-dyn")
	first := "hostname R1\n"
	second := "hostname R1\ninterface FastEthernet0/0\n ip address 10.0.0.1 255.255.255.0\n"
	empty := []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_node"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{
				Config:           env.providerBlock + dynamipsStartupConfig(project, source, "R1", first),
				ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: empty},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.r", "node_type", "dynamips"),
					env.checkStartupConfig("gns3_node.r", first),
				),
			},
			{
				Config: env.providerBlock + dynamipsStartupConfig(project, source, "R1-renamed", second),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_node.r", plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: empty,
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_node.r", "name", "R1-renamed"),
					env.checkStartupConfig("gns3_node.r", second),
				),
			},
		},
	}
}

func TestNodeStartupConfigDynamips(t *testing.T) {
	// The fake does not check that the image exists.
	resource.UnitTest(t, dynamipsStartupConfigTestCase(unitEnv(t),
		"node_type  = \"dynamips\"\n  compute_id = \"local\"\n  properties = jsonencode({ platform = \"c7200\", image = \"c7200-fake.image\" })"))
}

// TestAccNodeStartupConfigDynamips needs a Dynamips template (and so its IOS
// image) on the live server: GNS3 refuses a dynamips node whose image is
// missing (HTTP 409).
func TestAccNodeStartupConfigDynamips(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skip("acceptance test: set TF_ACC=1")
	}
	env := accEnv(t)
	templates, err := env.client.ListTemplates(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, tpl := range templates {
		if tpl.TemplateType == "dynamips" {
			t.Logf("using Dynamips template %q (%s)", tpl.Name, tpl.TemplateID)
			resource.Test(t, dynamipsStartupConfigTestCase(env, fmt.Sprintf("template_id    = %q", tpl.TemplateID)))
			return
		}
	}
	t.Skip("no Dynamips template on the server: add an IOS router template (Edit > Preferences > Dynamips > IOS routers) to run this test. " +
		"IOU is not tested: startup_config does not support iou yet.")
}
