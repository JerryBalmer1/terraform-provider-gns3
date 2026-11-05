# List resources need Terraform 1.14 or later. Put this file beside the root
# module's .tf files and run: terraform query
list "gns3_project" "all" {
  provider = gns3
}
