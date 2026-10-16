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

resource "gns3_project" "lab" {
  name = "tf-scenario-vpcs-startup-config"
}

# Built-in template on every GNS3 2.2 server: no image needed.
data "gns3_template" "vpcs" {
  name = "VPCS"
}

# startup_config is written to each node's startup.vpc. One file per node
# because the addresses differ; both land as startup.vpc on the server.
resource "gns3_node" "pc1" {
  project_id     = gns3_project.lab.project_id
  template_id    = data.gns3_template.vpcs.template_id
  name           = "PC1"
  x              = -150
  y              = 0
  startup_config = file("${path.module}/pc1.vpc")
}

resource "gns3_node" "pc2" {
  project_id     = gns3_project.lab.project_id
  template_id    = data.gns3_template.vpcs.template_id
  name           = "PC2"
  x              = 150
  y              = 0
  startup_config = file("${path.module}/pc2.vpc")
}

resource "gns3_link" "pc1_pc2" {
  project_id = gns3_project.lab.project_id

  node {
    node_id        = gns3_node.pc1.node_id
    adapter_number = gns3_node.pc1.ports[0].adapter_number
    port_number    = gns3_node.pc1.ports[0].port_number
  }

  node {
    node_id        = gns3_node.pc2.node_id
    adapter_number = gns3_node.pc2.ports[0].adapter_number
    port_number    = gns3_node.pc2.ports[0].port_number
  }
}

output "project_id" {
  value = gns3_project.lab.project_id
}

output "nodes" {
  value = {
    for n in [gns3_node.pc1, gns3_node.pc2] : n.name => {
      node_id = n.node_id
    }
  }
}

output "link_id" {
  value = gns3_link.pc1_pc2.link_id
}
