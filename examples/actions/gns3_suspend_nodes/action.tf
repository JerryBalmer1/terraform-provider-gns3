# Actions need Terraform 1.14 or later.
resource "gns3_project" "lab" {
  name = "tf-example-suspend-nodes"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"

  # Suspend every node in the project after this node is changed.
  lifecycle {
    action_trigger {
      events  = [after_update]
      actions = [action.gns3_suspend_nodes.all]
    }
  }
}

# No node_ids: every node in the project.
action "gns3_suspend_nodes" "all" {
  config {
    project_id = gns3_project.lab.project_id
  }
}

# Only the nodes named. Run it on demand:
# terraform apply -invoke=action.gns3_suspend_nodes.pc1
action "gns3_suspend_nodes" "pc1" {
  config {
    project_id = gns3_project.lab.project_id
    node_ids   = [gns3_node.pc1.node_id]
  }
}
