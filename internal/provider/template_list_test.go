package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// templateListTestCase lists the vpcs user template the first step creates,
// filtered by template_type, with its resource.
func templateListTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-list-template")
	byName := queryfilter.ByDisplayName(knownvalue.StringExact(name))
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   listQueryVersion,
		CheckDestroy:             resource.ComposeTestCheckFunc(env.checkGone("gns3_template"), env.checkGone("gns3_project")),
		Steps: []resource.TestStep{
			{Config: env.providerBlock + templateVpcsConfig(name, `{ console_type = "none" }`)},
			{
				Query: true,
				Config: `
list "gns3_template" "vpcs" {
  provider         = gns3
  include_resource = true
  config {
    template_type = "vpcs"
  }
}
`,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast("gns3_template.vpcs", 1),
					querycheck.ExpectResourceDisplayName("gns3_template.vpcs", byName, knownvalue.StringExact(name)),
					querycheck.ExpectResourceKnownValues("gns3_template.vpcs", byName, []querycheck.KnownValueCheck{
						{Path: tfjsonpath.New("template_type"), KnownValue: knownvalue.StringExact("vpcs")},
						{Path: tfjsonpath.New("builtin"), KnownValue: knownvalue.Bool(false)},
						{Path: tfjsonpath.New("compute_id"), KnownValue: knownvalue.StringExact("local")},
						{Path: tfjsonpath.New("category"), KnownValue: knownvalue.StringExact("guest")},
						{Path: tfjsonpath.New("properties"), KnownValue: knownvalue.Null()},
					}),
				},
			},
		},
	}
}

// TestTemplateList also checks, against the fake's known contents, that
// built-in templates are left out and that the filter drops other types.
func TestTemplateList(t *testing.T) {
	tc := templateListTestCase(unitEnv(t))
	tc.Steps = append(tc.Steps, resource.TestStep{
		Query: true,
		Config: `
list "gns3_template" "all" {
  provider = gns3
}

list "gns3_template" "qemu" {
  provider = gns3
  config {
    template_type = "qemu"
  }
}

list "gns3_template" "with_builtin" {
  provider = gns3
  config {
    include_builtin = true
  }
}

list "gns3_template" "vpcs_with_builtin" {
  provider         = gns3
  include_resource = true
  config {
    template_type   = "vpcs"
    include_builtin = true
  }
}
`,
		QueryResultChecks: []querycheck.QueryResultCheck{
			querycheck.ExpectLength("gns3_template.all", 1),
			querycheck.ExpectIdentity("gns3_template.all", map[string]knownvalue.Check{"template_id": knownvalue.NotNull()}),
			querycheck.ExpectLength("gns3_template.qemu", 0),
			// The fake's three built-ins (VPCS, Ethernet switch, Ethernet hub) plus the user template.
			querycheck.ExpectLength("gns3_template.with_builtin", 4),
			querycheck.ExpectLength("gns3_template.vpcs_with_builtin", 2),
			querycheck.ExpectResourceKnownValues("gns3_template.vpcs_with_builtin",
				queryfilter.ByDisplayName(knownvalue.StringExact("VPCS")), []querycheck.KnownValueCheck{
					{Path: tfjsonpath.New("builtin"), KnownValue: knownvalue.Bool(true)},
					{Path: tfjsonpath.New("template_id"), KnownValue: knownvalue.StringExact("19021f99-e36f-394d-b4a1-8aaa902ab9cc")},
				}),
		},
	})
	resource.UnitTest(t, tc)
}

func TestAccTemplateList(t *testing.T) { resource.Test(t, templateListTestCase(accEnv(t))) }
