package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func computeDataSourceTestCase(env testEnv) resource.TestCase {
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		Steps: []resource.TestStep{{
			Config: env.providerBlock + `
data "gns3_compute" "local" {
  compute_id = "local"
}

data "gns3_compute" "by_name" {
  name = data.gns3_compute.local.name
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.gns3_compute.local", "compute_id", "local"),
				resource.TestCheckResourceAttrSet("data.gns3_compute.local", "host"),
				resource.TestCheckResourceAttrSet("data.gns3_compute.local", "port"),
				resource.TestCheckResourceAttr("data.gns3_compute.local", "connected", "true"),
				resource.TestCheckResourceAttr("data.gns3_compute.by_name", "compute_id", "local"),
			),
		}},
	}
}

func TestComputeDataSource(t *testing.T) { resource.UnitTest(t, computeDataSourceTestCase(unitEnv(t))) }

func TestAccComputeDataSource(t *testing.T) { resource.Test(t, computeDataSourceTestCase(accEnv(t))) }
