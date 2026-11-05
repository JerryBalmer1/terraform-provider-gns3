# Actions need Terraform 1.14 or later.
resource "gns3_project" "lab" {
  name = "tf-example-stop-nodes"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"

  # Stop every node in the project before this node is changed.
  lifecycle {
    action_trigger {
      events  = [before_update]
      actions = [action.gns3_stop_nodes.all]
    }
  }
}

# No node_ids: every node in the project.
action "gns3_stop_nodes" "all" {
  config {
    project_id = gns3_project.lab.project_id
  }
}

# Only the nodes named. Run it on demand:
# terraform apply -invoke=action.gns3_stop_nodes.pc1
action "gns3_stop_nodes" "pc1" {
  config {
    project_id = gns3_project.lab.project_id
    node_ids   = [gns3_node.pc1.node_id]
  }
}
