terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "0.3.0"
    }
  }
}

# Empty: the provider defaults to http://127.0.0.1:3080 (GNS3_HOST and
# GNS3_PORT override it).
provider "gns3" {}

# The project that terraform query must find. It has no nodes, so the
# gns3_node list in adopt.tfquery.hcl must come back empty.
resource "gns3_project" "lab" {
  name = "tf-scenario-query-adopt"
}

output "project_id" {
  value = gns3_project.lab.project_id
}

output "name" {
  value = gns3_project.lab.name
}
