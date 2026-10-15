resource "gns3_project" "lab" {
  name = "tf-example-node"
}

# Built-in VPCS template: no image to download.
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

# A template-less node: node_type, compute_id and emulator properties.
resource "gns3_node" "pc2" {
  project_id = gns3_project.lab.project_id
  node_type  = "vpcs"
  compute_id = "local"
  name       = "PC2"
  x          = 150
  y          = 0
  properties = jsonencode({ startup_script = "set pcname PC2" })
}

output "pc1_console" {
  value = gns3_node.pc1.console
}
