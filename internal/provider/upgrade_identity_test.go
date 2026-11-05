package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// requiredProvidersPublished names the provider by its Registry address, so
// that state written by the published release and the in-process provider
// (served under jerrybalmer1/gns3 through TF_ACC_PROVIDER_NAMESPACE) belong to
// the same provider.
const requiredProvidersPublished = `
terraform {
  required_providers {
    gns3 = { source = "JerryBalmer1/gns3" }
  }
}
`

// TestAccIdentityUpgradeFrom020 applies a configuration with the published
// 0.2.0, which writes no resource identity, then plans it with this checkout:
// the plan must be empty, and the refresh fills in every identity. It is an
// acceptance test (TF_ACC=1, live server) because it downloads 0.2.0 from the
// Registry, so it needs network access.
func TestAccIdentityUpgradeFrom020(t *testing.T) {
	t.Setenv("TF_ACC_PROVIDER_NAMESPACE", "jerrybalmer1")
	env := accEnv(t)
	name := acctest.RandomWithPrefix("tf-acc-upgrade")
	src := writeTempImage(t, "terraform-provider-gns3 upgrade image "+name)
	cfg := env.providerBlock + linkConfig(name, "pc2", "") + imageConfig(name+".img", src, "")
	notNull := knownvalue.NotNull()
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		CheckDestroy:           env.checkGone("gns3_project"),
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"gns3": {Source: "JerryBalmer1/gns3", VersionConstraint: "0.2.0"},
				},
				Config: cfg,
			},
			{
				ProtoV6ProviderFactories: protoV6Factories,
				Config:                   requiredProvidersPublished + cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("gns3_project.test", map[string]knownvalue.Check{"project_id": notNull}),
					statecheck.ExpectIdentity("gns3_node.pc1", map[string]knownvalue.Check{"project_id": notNull, "node_id": notNull}),
					statecheck.ExpectIdentity("gns3_link.test", map[string]knownvalue.Check{"project_id": notNull, "link_id": notNull}),
					statecheck.ExpectIdentity("gns3_image.test", map[string]knownvalue.Check{
						"compute_id": knownvalue.StringExact("local"), "emulator": knownvalue.StringExact("qemu"),
						"filename": knownvalue.StringExact(name + ".img"),
					}),
				},
			},
		},
	})
}
