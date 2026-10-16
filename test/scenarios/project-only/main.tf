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
  name         = "tf-scenario-project-only"
  grid_size    = 50
  scene_width  = 3000
  scene_height = 1500
  show_grid    = true
}

output "project_id" {
  value = gns3_project.lab.project_id
}

output "name" {
  value = gns3_project.lab.name
}

output "filename" {
  value = gns3_project.lab.filename
}
