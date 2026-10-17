# A user VPCS template. properties holds the type-specific fields; only the
# keys set here are compared with the server.
resource "gns3_template" "pc" {
  name          = "tf-example-vpcs"
  template_type = "vpcs"
  properties    = jsonencode({ console_type = "telnet" })
}

# A qemu template for an appliance disk image already on the compute (see
# gns3_image). Creating the template does not start anything.
resource "gns3_template" "router" {
  name          = "tf-example-qemu"
  template_type = "qemu"
  properties = jsonencode({
    ram            = 512
    adapters       = 3
    platform       = "x86_64"
    hda_disk_image = "vyos-1.4.qcow2"
  })
}

resource "gns3_project" "lab" {
  name = "tf-example-template"
}

# Nodes take the template's template_id like a built-in's.
resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = gns3_template.pc.template_id
  name        = "PC1"
  x           = 0
  y           = 0
}
