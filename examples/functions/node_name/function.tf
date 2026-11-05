# Provider functions need the provider in required_providers.
terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "~> 0.3"
    }
  }
}

resource "gns3_project" "lab" {
  name = "tf-example-node-name"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

# PC-01, PC-02 and PC-03.
resource "gns3_node" "pc" {
  count       = 3
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = provider::gns3::node_name("PC", count.index + 1)
  x           = count.index * 150
}
