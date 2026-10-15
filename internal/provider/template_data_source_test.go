package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func templateDataSourceTestCase(env testEnv) resource.TestCase {
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + `
data "gns3_template" "by_name" {
  name = "VPCS"
}

data "gns3_template" "by_id" {
  template_id = data.gns3_template.by_name.template_id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.gns3_template.by_name", "template_type", "vpcs"),
					resource.TestCheckResourceAttr("data.gns3_template.by_name", "builtin", "true"),
					resource.TestCheckResourceAttr("data.gns3_template.by_name", "category", "guest"),
					resource.TestCheckResourceAttrSet("data.gns3_template.by_name", "template_id"),
					resource.TestCheckResourceAttrSet("data.gns3_template.by_name", "symbol"),
					resource.TestCheckResourceAttrPair("data.gns3_template.by_id", "name", "data.gns3_template.by_name", "name"),
				),
			},
			{
				Config:      env.providerBlock + "data \"gns3_template\" \"missing\" {\n  name = \"no-such-template-tfacc\"\n}\n",
				ExpectError: regexp.MustCompile(`No template named "no-such-template-tfacc"`),
			},
		},
	}
}

func TestTemplateDataSource(t *testing.T) {
	resource.UnitTest(t, templateDataSourceTestCase(unitEnv(t)))
}

func TestAccTemplateDataSource(t *testing.T) { resource.Test(t, templateDataSourceTestCase(accEnv(t))) }
