resource "gns3_project" "lab" {
  name = "tf-example-link"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id      = gns3_project.lab.project_id
  template_id     = data.gns3_template.vpcs.template_id
  name            = "PC1"
  x               = -150
  start_on_create = true
}

resource "gns3_node" "pc2" {
  project_id      = gns3_project.lab.project_id
  template_id     = data.gns3_template.vpcs.template_id
  name            = "PC2"
  x               = 150
  start_on_create = true
}

# Ports come back from the server after the nodes are created.
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
