# query-adopt

Up: [Scenario tests](../README.md)

See also: [actions-start-stop](../actions-start-stop/README.md), [image-upload](../image-upload/README.md), [project-only](../project-only/README.md), [template-roundtrip](../template-roundtrip/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Builds one empty `gns3_project` named `tf-scenario-query-adopt`, then runs `terraform query` against [adopt.tfquery.hcl](adopt.tfquery.hcl). That file declares two list blocks:

- `list "gns3_project" "all"`, with no `config`: every project on the server.
- `list "gns3_node" "in_project"`, with `config { project_id = var.project_id }`: every node in the scenario's project.

A list block queries the server, not the state, so the query file cannot refer to `gns3_project.lab`. It declares `variable "project_id"`, and `checks.ps1` passes the `project_id` output with `-var`. Plan and apply read only `.tf` files, so `main.tf` applies without that variable.

`terraform query` is how you find objects to adopt: `terraform query -generate-config-out=adopt.tf` writes `import` and `resource` blocks for every result. This scenario checks only the listing and does not generate configuration.

List resources need Terraform 1.14 or later and first ship in provider 0.3.0. `main.tf` pins the published 0.2.0 like the other scenarios. `scenario.json` sets `"minVersion": "0.3.0"`, so published runs skip it (`requires provider >= 0.3.0, pinned 0.2.0`) until release-prep bumps the pins. `-Dev` runs it.

## What checks.ps1 verifies

- `GET /v2/projects/{project_id}` answers with the name `tf-scenario-query-adopt`, and `GET /v2/projects/{project_id}/nodes` returns no nodes.
- `terraform query -json -var project_id=<id>` in this folder exits 0 and prints no `error`-level message. The script uses `$env:TERRAFORM_EXE` if it is set, then `%LOCALAPPDATA%\Programs\Terraform\terraform.exe`, then `terraform` on `PATH`. It inherits the CLI config that `Gns3Range` sets.
- Among the `list_resource_found` messages for `list.gns3_project.all`, exactly one has an `identity` holding the `project_id` output, with `resource_type` `gns3_project`. That block's `list_complete` `total` equals its number of results.
- `list.gns3_node.in_project` has no `list_resource_found` messages, and its `list_complete` `total` is 0. That is the expected value, because the scenario creates no nodes.

## In the GNS3 GUI

While the scenario is applied (before `destroy`), **File > Open project** lists `tf-scenario-query-adopt`. Opening it shows an empty canvas. `terraform query` changes nothing on the server, so nothing in the GUI changes when it runs. After `destroy` the project is gone from the list.
