# template-roundtrip

Up: [Scenario tests](../README.md)

See also: [image-upload](../image-upload/README.md), [project-only](../project-only/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Builds a user VPCS template `tf-scenario-template-roundtrip` with `gns3_template` (no `properties`, so the server's defaults apply), a project of the same name, and one node `PC1` at (0, 0) whose `template_id` is `gns3_template.pc.template_id`. Nothing is linked and the node is not started.

`gns3_template` first ships in 0.3.0, and `main.tf` pins the published 0.2.0 like the other scenarios. `scenario.json` sets `"minVersion": "0.3.0"`, so a published run (including the pre-push hook's run for `main`) skips it with `requires provider >= 0.3.0, pinned 0.2.0`. Without the skip, apply fails with `Error: Invalid resource type` (seen 2026-10-16). `-Dev` runs it. Once release-prep bumps the pins to 0.3.0, the skip goes away.

## What checks.ps1 verifies

- `GET /v2/templates/{template_id}` for the `template_id` output answers, with name `tf-scenario-template-roundtrip`, `template_type` `vpcs`, `builtin` false and `compute_id` `local`.
- The project exists under the `project_id` output and has the expected name.
- `GET /v2/projects/{project_id}/nodes` returns exactly one node, `PC1`, with the `node_id` Terraform reports, `node_type` `vpcs` and `template_id` equal to the template's.

`checks.ps1` runs before destroy, so it can't show that destroy removed the template. `Gns3Range` then runs `destroy`, which fails if the template or the project can't be deleted. Check afterwards by hand: `Invoke-RestMethod http://127.0.0.1:3080/v2/templates | ForEach-Object { $_ } | Where-Object name -eq tf-scenario-template-roundtrip` prints nothing.

## In the GNS3 GUI

Not yet seen in the GUI: needs a GNS3 GUI session while the scenario is applied (`Invoke-Build Gns3Range -Dev -Scenario template-roundtrip -Inspect`, then `-Destroy`).

What should show: while it is applied, the device list (**Browse End Devices**) has a `tf-scenario-template-roundtrip` entry next to the built-in VPCS, and the project `tf-scenario-template-roundtrip` (**File > Open project**) shows one stopped PC1. After `destroy` the project and the template entry are both gone.
