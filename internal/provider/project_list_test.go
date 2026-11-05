package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// listQueryVersion is the first Terraform that runs terraform query.
var listQueryVersion = []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)}

func projectListTestCase(env testEnv) resource.TestCase {
	name := acctest.RandomWithPrefix("tf-acc-list-project")
	other := name + "-other"
	cfg := env.providerBlock + fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

resource "gns3_project" "other" {
  name      = %q
  grid_size = 100
}
`, name, other)
	byName := queryfilter.ByDisplayName(knownvalue.StringExact(other))
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   listQueryVersion,
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				// The first step's configuration stays in place, with the
				// provider block; Config is the .tfquery.hcl file.
				Query: true,
				Config: `
list "gns3_project" "all" {
  provider         = gns3
  include_resource = true
}
`,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast("gns3_project.all", 2),
					querycheck.ExpectResourceDisplayName("gns3_project.all", byName, knownvalue.StringExact(other)),
					querycheck.ExpectResourceKnownValues("gns3_project.all", byName, []querycheck.KnownValueCheck{
						{Path: tfjsonpath.New("name"), KnownValue: knownvalue.StringExact(other)},
						{Path: tfjsonpath.New("grid_size"), KnownValue: knownvalue.Int64Exact(100)},
						{Path: tfjsonpath.New("status"), KnownValue: knownvalue.StringExact("opened")},
					}),
				},
			},
		},
	}
}

// TestProjectList lists the fake's projects: exactly the two the first step
// created, each with its identity.
func TestProjectList(t *testing.T) {
	env := unitEnv(t)
	tc := projectListTestCase(env)
	tc.Steps[1].QueryResultChecks = append(tc.Steps[1].QueryResultChecks,
		querycheck.ExpectLength("gns3_project.all", 2),
		querycheck.ExpectIdentity("gns3_project.all", map[string]knownvalue.Check{"project_id": knownvalue.NotNull()}),
	)
	resource.UnitTest(t, tc)
}

func TestAccProjectList(t *testing.T) { resource.Test(t, projectListTestCase(accEnv(t))) }
