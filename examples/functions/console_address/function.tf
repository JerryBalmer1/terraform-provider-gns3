# Provider functions need the provider in required_providers.
terraform {
  required_providers {
    gns3 = {
      source  = "JerryBalmer1/gns3"
      version = "~> 0.3"
    }
  }
}

resource "gns3_project" "lab" {
  name = "tf-example-console-address"
}

data "gns3_template" "vpcs" {
  name = "VPCS"
}

resource "gns3_node" "pc1" {
  project_id  = gns3_project.lab.project_id
  template_id = data.gns3_template.vpcs.template_id
  name        = "PC1"
}

# "127.0.0.1:5000", ready for telnet.
output "pc1_console" {
  value = provider::gns3::console_address("127.0.0.1", gns3_node.pc1.console)
}
