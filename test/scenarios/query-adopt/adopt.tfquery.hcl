# Read by terraform query only (plan and apply ignore .tfquery.hcl files).
# checks.ps1 runs: terraform query -json -var project_id=<project_id output>
# List blocks query the server, not state, so the project ID comes in as a
# variable rather than as a reference to gns3_project.lab.

variable "project_id" {
  type        = string
  description = "Project whose nodes list.gns3_node.in_project lists: the project_id output of main.tf."
}

# Every project on the server.
list "gns3_project" "all" {
  provider = gns3
}

# Every node in the scenario's project: none, because main.tf creates no node.
list "gns3_node" "in_project" {
  provider = gns3

  config {
    project_id = var.project_id
  }
}
