# List resources need Terraform 1.14 or later. Put this file beside the root
# module's .tf files and run: terraform query
list "gns3_template" "all" {
  provider = gns3
}

# Only qemu templates, with every attribute rather than the identity alone.
list "gns3_template" "qemu" {
  provider         = gns3
  include_resource = true

  config {
    template_type = "qemu"
  }
}

# Built-in templates too (VPCS, Ethernet switch, ...), for a look at what the
# server has. Built-ins can't be imported into gns3_template.
list "gns3_template" "everything" {
  provider = gns3

  config {
    include_builtin = true
  }
}
