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
  name = "tf-scenario-actions-start-stop"
}

# Built-in template on every GNS3 2.2 server: no image needed.
data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"
  x           = -150
  y           = 0
}

resource "gns3_node" "pc2" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC2"
  x           = 150
  y           = 0
}

# No node_ids: both actions act on every node in the project.
action "gns3_start_nodes" "start" {
  config {
    project_id = gns3_project.lab.project_id
  }
}

# Triggered by nothing here: checks.ps1 runs
# terraform apply -invoke=action.gns3_stop_nodes.stop.
action "gns3_stop_nodes" "stop" {
  config {
    project_id = gns3_project.lab.project_id
  }
}

# The link references both nodes, so it is created last: after_create on it
# runs once PC1 and PC2 both exist.
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

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.gns3_start_nodes.start]
    }
  }
}

output "project_id" {
  value = gns3_project.lab.project_id
}

output "nodes" {
  value = {
    for n in [gns3_node.pc1, gns3_node.pc2] : n.name => n.node_id
  }
}
