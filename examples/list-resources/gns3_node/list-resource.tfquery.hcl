# List resources need Terraform 1.14 or later. Put this file beside the root
# module's .tf files and run: terraform query
list "gns3_node" "lab" {
  provider = gns3

  config {
    project_id = "2d37e1d1-0bbd-4525-8777-529b1bba7f85"
  }
}
