# Actions need Terraform 1.14 or later.
resource "gns3_project" "lab" {
  name = "tf-example-snapshot"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"

  # Take a snapshot called "baseline" once the node exists.
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.gns3_snapshot.baseline]
    }
  }
}

action "gns3_snapshot" "baseline" {
  config {
    project_id = gns3_project.lab.project_id
    name       = "baseline"
  }
}

# Roll the project back to "baseline". Run it on demand:
# terraform apply -invoke=action.gns3_snapshot.restore_baseline
action "gns3_snapshot" "restore_baseline" {
  config {
    project_id = gns3_project.lab.project_id
    name       = "baseline"
    restore    = true
  }
}
