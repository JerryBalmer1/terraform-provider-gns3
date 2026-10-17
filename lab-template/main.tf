# A starting point for your own lab: copy this folder, edit, apply, destroy.
# See README.md beside this file.

terraform {
  required_providers {
    gns3 = {
      source = "JerryBalmer1/gns3"
      # gns3_template first ships in 0.3.0. This folder is meant to be copied,
      # so it takes any later release; pin an exact version in your copy if you
      # want repeatable runs.
      version = ">= 0.3.0"
    }
  }
}

# Defaults to http://127.0.0.1:3080. Set host/port here, or GNS3_HOST and
# GNS3_PORT in the environment, for another server.
provider "gns3" {}

locals {
  # Rename this before the first apply: it names the project and the template.
  lab = "my-lab"
}

resource "gns3_project" "lab" {
  name = local.lab
}

# The lab's own template. Destroy deletes it again; the built-in VPCS template
# is left alone. For a router, use template_type = "qemu" and set ram,
# adapters and hda_disk_image in properties (see docs/resources/template.md).
resource "gns3_template" "pc" {
  name          = "${local.lab}-pc"
  template_type = "vpcs"
}

# startup_config is written to each node's startup.vpc and takes effect when
# the node starts. Edit the files in configs/ and apply again to change it.
resource "gns3_node" "pc1" {
  project_id     = gns3_project.lab.project_id
  template_id    = gns3_template.pc.template_id
  name           = "PC1"
  x              = -150
  y              = 0
  startup_config = file("${path.module}/configs/pc1.vpc")
}

resource "gns3_node" "pc2" {
  project_id     = gns3_project.lab.project_id
  template_id    = gns3_template.pc.template_id
  name           = "PC2"
  x              = 150
  y              = 0
  startup_config = file("${path.module}/configs/pc2.vpc")
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

output "consoles" {
  value = { for n in [gns3_node.pc1, gns3_node.pc2] : n.name => n.console }
}
