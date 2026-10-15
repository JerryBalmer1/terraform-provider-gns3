package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func projectConfig(name string, gridSize int) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name      = %q
  grid_size = %d
}
`, name, gridSize)
}

func projectTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-project")
	const addr = "gns3_project.test"
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + projectConfig(name, 75),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", name),
					resource.TestCheckResourceAttr(addr, "status", "opened"),
					resource.TestCheckResourceAttr(addr, "filename", name+".gns3"),
					resource.TestCheckResourceAttr(addr, "grid_size", "75"),
					resource.TestCheckResourceAttr(addr, "auto_close", "true"),
					resource.TestCheckResourceAttr(addr, "scene_width", "2000"),
					resource.TestCheckResourceAttrSet(addr, "project_id"),
					resource.TestCheckResourceAttrSet(addr, "path"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: env.providerBlock + projectConfig(name+"-renamed", 100),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", name+"-renamed"),
					resource.TestCheckResourceAttr(addr, "grid_size", "100"),
				),
			},
			{
				ResourceName:                         addr,
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    attrFromState(addr, "project_id"),
				ImportStateVerifyIdentifierAttribute: "project_id",
			},
		},
	}
}

func TestProjectResource(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, projectTestCase(env))
	if p, n, l := env.fake.counts(); p+n+l != 0 {
		t.Errorf("fake still holds %d projects, %d nodes, %d links after destroy", p, n, l)
	}
}

// TestProjectResourceDeleteRetriesLockedDirectory: on Windows the server
// answers DELETE with 409 "Cannot delete project directory ... [WinError 32]"
// while a just-deleted node's emulator still holds a file open; destroy
// retries until the directory is free.
func TestProjectResourceDeleteRetriesLockedDirectory(t *testing.T) {
	env := unitEnv(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{{
			Config: env.providerBlock + projectConfig(acctest.RandomWithPrefix("tf-unit-locked"), 75),
			Check: func(*terraform.State) error {
				env.fake.lockProjectDeletes(2)
				return nil
			},
		}},
	})
	if p, _, _ := env.fake.counts(); p != 0 {
		t.Errorf("fake still holds %d projects after destroy", p)
	}
}

func TestAccProjectResource(t *testing.T) {
	resource.Test(t, projectTestCase(accEnv(t)))
}
