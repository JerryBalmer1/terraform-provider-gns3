package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// builtinVPCSTemplateID is the built-in VPCS template on every 2.2 server.
const builtinVPCSTemplateID = "19021f99-e36f-394d-b4a1-8aaa902ab9cc"

// templateVpcsConfig: a vpcs template and one node created from it.
func templateVpcsConfig(name, properties string) string {
	return fmt.Sprintf(`
resource "gns3_template" "pc" {
  name          = %[1]q
  template_type = "vpcs"
  properties    = jsonencode(%[2]s)
}

resource "gns3_project" "test" {
  name = %[1]q
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.test.project_id
  template_id = gns3_template.pc.template_id
  name        = "PC1"
  x           = 0
  y           = 0
}
`, name, properties)
}

func templateVpcsTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-template")
	renamed := name + "-renamed"
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			env.checkGone("gns3_template"), env.checkGone("gns3_project"), env.checkNoValidateTemplates(),
		),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + templateVpcsConfig(name, `{ console_type = "none" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("gns3_template.pc", "template_id"),
					resource.TestCheckResourceAttr("gns3_template.pc", "compute_id", "local"),
					resource.TestCheckResourceAttr("gns3_template.pc", "builtin", "false"),
					resource.TestCheckResourceAttr("gns3_template.pc", "category", "guest"),
					resource.TestCheckResourceAttr("gns3_template.pc", "symbol", ":/symbols/vpcs_guest.svg"),
					resource.TestCheckResourceAttr("gns3_template.pc", "default_name_format", "PC{0}"),
					resource.TestCheckResourceAttr("gns3_template.pc", "properties", `{"console_type":"none"}`),
					resource.TestCheckResourceAttrPair("gns3_node.pc1", "template_id", "gns3_template.pc", "template_id"),
					resource.TestCheckResourceAttr("gns3_node.pc1", "node_type", "vpcs"),
					env.checkTemplateField("gns3_template.pc", "console_type", "none"),
				),
			},
			{
				// Rename and change a property: in place, and the node stays.
				Config: env.providerBlock + templateVpcsConfig(renamed, `{ console_type = "telnet" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("gns3_template.pc", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("gns3_node.pc1", plancheck.ResourceActionNoop),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_template.pc", "name", renamed),
					env.checkTemplateField("gns3_template.pc", "name", renamed),
					env.checkTemplateField("gns3_template.pc", "console_type", "telnet"),
				),
			},
			{
				ResourceName:      "gns3_template.pc",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: attrFromState("gns3_template.pc", "template_id"),
				// Which properties keys are managed comes from configuration.
				ImportStateVerifyIdentifierAttribute: "template_id",
				ImportStateVerifyIgnore:              []string{"properties"},
			},
			{
				// A key the vpcs schema lacks: a PUT would store it and break
				// the next node created from the template; the trial create refuses it.
				Config:      env.providerBlock + templateVpcsConfig(renamed, `{ console_type = "telnet", ram = 256 }`),
				ExpectError: regexp.MustCompile(`(?s)GNS3 rejected the template properties.*'ram' was unexpected`),
			},
			{
				// The rejected update left the template as it was.
				Config:   env.providerBlock + templateVpcsConfig(renamed, `{ console_type = "telnet" }`),
				PlanOnly: true,
			},
			{
				ResourceName:  "gns3_template.builtin",
				ImportState:   true,
				ImportStateId: builtinVPCSTemplateID,
				Config:        env.providerBlock + templateVpcsConfig(renamed, `{ console_type = "telnet" }`) + "\nresource \"gns3_template\" \"builtin\" {\n  name          = \"VPCS\"\n  template_type = \"vpcs\"\n}\n",
				ExpectError:   regexp.MustCompile(`Built-in GNS3 template`),
			},
		},
	}
}

func TestTemplateResourceVpcs(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, templateVpcsTestCase(env))
	if n := len(env.fake.templates); n != len(fakeTemplates) {
		t.Errorf("fake holds %d templates after destroy, want the %d built-ins", n, len(fakeTemplates))
	}
}

func TestAccTemplateResourceVpcs(t *testing.T) { resource.Test(t, templateVpcsTestCase(accEnv(t))) }

func templateQemuConfig(name, tplType, properties string) string {
	props := ""
	if properties != "" {
		props = "  properties    = jsonencode(" + properties + ")\n"
	}
	return fmt.Sprintf("resource \"gns3_template\" \"vm\" {\n  name          = %q\n  template_type = %q\n%s}\n", name, tplType, props)
}

// templateQemuTestCase creates and updates a qemu template only; no node is
// created from it, so nothing is ever started and no image is needed.
func templateQemuTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-template-qemu")
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_template"), env.checkNoValidateTemplates()),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + templateQemuConfig(name, "qemu", `{ ram = 512, adapters = 2, platform = "x86_64" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("gns3_template.vm", "template_type", "qemu"),
					resource.TestCheckResourceAttr("gns3_template.vm", "default_name_format", "{name}-{0}"),
					resource.TestCheckResourceAttr("gns3_template.vm", "symbol", ":/symbols/qemu_guest.svg"),
					resource.TestCheckResourceAttr("gns3_template.vm", "properties", `{"adapters":2,"platform":"x86_64","ram":512}`),
					env.checkTemplateField("gns3_template.vm", "ram", "512"),
					env.checkTemplateField("gns3_template.vm", "adapter_type", "e1000"),
				),
			},
			{
				Config: env.providerBlock + templateQemuConfig(name, "qemu", `{ ram = 384, adapters = 2, platform = "x86_64" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_template.vm", plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: env.checkTemplateField("gns3_template.vm", "ram", "384"),
			},
			{
				// A wrongly typed value: PUT on 2.2.61 would store it.
				Config:      env.providerBlock + templateQemuConfig(name, "qemu", `{ ram = "lots", adapters = 2, platform = "x86_64" }`),
				ExpectError: regexp.MustCompile(`GNS3 rejected the template properties`),
			},
			{
				ResourceName:                         "gns3_template.vm",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    attrFromState("gns3_template.vm", "template_id"),
				ImportStateVerifyIdentifierAttribute: "template_id",
				ImportStateVerifyIgnore:              []string{"properties"},
			},
			{
				// GNS3 ignores a new template_type in an update, so it replaces.
				Config: env.providerBlock + templateQemuConfig(name, "vpcs", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_template.vm", plancheck.ResourceActionDestroyBeforeCreate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("gns3_template.vm", "template_type", "vpcs"),
			},
		},
	}
}

func TestTemplateResourceQemu(t *testing.T) { resource.UnitTest(t, templateQemuTestCase(unitEnv(t))) }

func TestAccTemplateResourceQemu(t *testing.T) { resource.Test(t, templateQemuTestCase(accEnv(t))) }

func TestTemplateConfigValidation(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config:      env.providerBlock + templateQemuConfig("r", "dynamips", `{ ram = 256 }`),
				ExpectError: regexp.MustCompile(`(?s)A dynamips template needs properties.platform.*A dynamips template needs properties.image`),
			},
			{
				Config:      env.providerBlock + templateQemuConfig("d", "docker", `{}`),
				ExpectError: regexp.MustCompile(`A docker template needs properties.image`),
			},
			{
				Config:      env.providerBlock + templateQemuConfig("c", "vpcs", `{ symbol = ":/symbols/router.svg" }`),
				ExpectError: regexp.MustCompile(`properties.symbol is a common template field`),
			},
			{
				Config:      env.providerBlock + templateQemuConfig("t", "nosuch", ""),
				ExpectError: regexp.MustCompile(`template_type value must be one of`),
			},
		},
	})
}

func TestTemplateResourceGoneOutsideTerraform(t *testing.T) {
	env := unitEnv(t)
	cfg := env.providerBlock + templateQemuConfig("tf-unit-gone", "vpcs", "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				// Deleted on the server: refresh drops it and the plan recreates it.
				PreConfig: func() {
					all, err := env.client.ListTemplates(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					for _, tpl := range all {
						if tpl.Name == "tf-unit-gone" {
							if err := env.client.DeleteTemplate(context.Background(), tpl.TemplateID); err != nil {
								t.Fatal(err)
							}
						}
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("gns3_template.vm", plancheck.ResourceActionCreate)},
				},
			},
		},
	})
}

// checkTemplateField compares one key of the template on the server.
func (e testEnv) checkTemplateField(name, key, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}
		tpl, err := e.client.GetTemplate(context.Background(), rs.Primary.Attributes["template_id"])
		if err != nil {
			return err
		}
		got := tpl.Name
		if key != "name" {
			got = strings.Trim(string(tpl.Properties[key]), `"`)
		}
		if got != want {
			return fmt.Errorf("%s on the server: %s = %q, want %q", name, key, got, want)
		}
		return nil
	}
}

// checkNoValidateTemplates fails if a throwaway tf-validate-* template is left.
func (e testEnv) checkNoValidateTemplates() resource.TestCheckFunc {
	return func(*terraform.State) error {
		all, err := e.client.ListTemplates(context.Background())
		if err != nil {
			return err
		}
		for _, tpl := range all {
			if strings.HasPrefix(tpl.Name, "tf-validate-") {
				return fmt.Errorf("throwaway template %s (%s) was left on the server", tpl.Name, tpl.TemplateID)
			}
		}
		return nil
	}
}
