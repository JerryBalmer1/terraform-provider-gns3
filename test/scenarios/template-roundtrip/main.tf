terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "0.2.0"
    }
  }
}

# Empty: the provider defaults to http://127.0.0.1:3080 (GNS3_HOST and
# GNS3_PORT override it).
provider "gns3" {}

# A user template, not the built-in VPCS one: destroy deletes it again.
resource "gns3_template" "pc" {
  name          = "tf-scenario-template-roundtrip"
  template_type = "vpcs"
}

resource "gns3_project" "lab" {
  name = "tf-scenario-template-roundtrip"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = gns3_template.pc.template_id
  name        = "PC1"
  x           = 0
  y           = 0
}

output "project_id" {
  value = gns3_project.lab.project_id
}

output "template_id" {
  value = gns3_template.pc.template_id
}

output "node_id" {
  value = gns3_node.pc1.node_id
}
