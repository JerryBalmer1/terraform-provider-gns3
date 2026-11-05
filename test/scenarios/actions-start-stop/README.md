# actions-start-stop

Up: [Scenario tests](../README.md)

See also: [image-upload](../image-upload/README.md), [project-only](../project-only/README.md), [query-adopt](../query-adopt/README.md), [template-roundtrip](../template-roundtrip/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Builds a project `tf-scenario-actions-start-stop` with two VPCS nodes, `PC1` at (-150, 0) and `PC2` at (150, 0), made from the built-in `VPCS` template. One `gns3_link` joins their first ports, the same shape as [two-nodes-one-link](../two-nodes-one-link/README.md). It also declares two actions on the project. Neither sets `node_ids`, so each one acts on every node:

- `action.gns3_start_nodes.start`, which `lifecycle { action_trigger { events = [after_create] } }` on the link runs.
- `action.gns3_stop_nodes.stop`, which nothing in the configuration triggers. `checks.ps1` invokes it.

The trigger is on the link because the link is the last resource created. It references both nodes, so Terraform creates it only after `PC1` and `PC2` exist. A trigger on one node could fire before the other node exists, and that node would stay stopped. A trigger on the project would fire before any node exists.

Actions need Terraform 1.14 or later and first ship in provider 0.3.0. `main.tf` pins 0.3.0 like the other scenarios, and `scenario.json` sets `"minVersion": "0.3.0"`, so a published run with an older pin would skip it.

## What checks.ps1 verifies

- After apply, `GET /v2/projects/{project_id}/nodes/{node_id}` reports `status` `started` for `PC1` and `PC2`, with ids from the `nodes` output. This shows that the `after_create` action ran.
- `terraform apply -invoke=action.gns3_stop_nodes.stop -auto-approve -input=false` in this folder exits 0. The script uses `$env:TERRAFORM_EXE` if it is set, then `%LOCALAPPDATA%\Programs\Terraform\terraform.exe`, then `terraform` on `PATH`. It inherits the CLI config that `Gns3Range` sets, which is the dev override under `-Dev`.
- Afterwards both nodes report `status` `stopped`.

`Gns3Range` then destroys the scenario as usual.

## In the GNS3 GUI

Not yet seen in the GUI. This needs a GNS3 GUI session while the scenario is applied: run `Invoke-Build Gns3Range -Dev -Scenario actions-start-stop -Inspect`, then `-Destroy`.

What should show: while it is applied, the project `tf-scenario-actions-start-stop` (**File > Open project**) shows PC1 (left) and PC2 (right) joined by one cable. Right after apply both nodes are started, with green link indicators. Once `checks.ps1` has invoked the stop action they are stopped, with red indicators. `-Inspect` runs `checks.ps1` and then starts every node again, so an inspected scenario shows them started. After `destroy` the project is gone.
