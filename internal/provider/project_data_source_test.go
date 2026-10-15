package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func projectDataSourceTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-ds-project")
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{{
			Config: env.providerBlock + fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

data "gns3_project" "test" {
  name       = gns3_project.test.name
  depends_on = [gns3_project.test]
}
`, name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.gns3_project.test", "project_id", "gns3_project.test", "project_id"),
				resource.TestCheckResourceAttrPair("data.gns3_project.test", "path", "gns3_project.test", "path"),
				resource.TestCheckResourceAttr("data.gns3_project.test", "status", "opened"),
			),
		}},
	}
}

func TestProjectDataSource(t *testing.T) { resource.UnitTest(t, projectDataSourceTestCase(unitEnv(t))) }

func TestAccProjectDataSource(t *testing.T) { resource.Test(t, projectDataSourceTestCase(accEnv(t))) }
