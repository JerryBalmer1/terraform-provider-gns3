# vpcs-startup-config

Up: [Scenario tests](../README.md)

See also: [image-upload](../image-upload/README.md), [project-only](../project-only/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md)

Builds a project `tf-scenario-vpcs-startup-config` with two VPCS nodes from the built-in `VPCS` template, `PC1` at (-150, 0) and `PC2` at (150, 0), joined by one `gns3_link` between their first ports. Each node sets `startup_config = file(...)`: `pc1.vpc` (`set pcname PC1`, `ip 10.0.0.1/24`) and `pc2.vpc` (`set pcname PC2`, `ip 10.0.0.2/24`). The provider writes each one to the node's `startup.vpc`. The nodes are not started.

## What checks.ps1 verifies

- The project exists under the `project_id` output and has the expected name.
- `GET /v2/projects/{project_id}/nodes` returns exactly two nodes, `PC1` and `PC2`, each with the `node_id` Terraform reports and `node_type` `vpcs`.
- For each node, `GET /v2/projects/{project_id}/nodes/{node_id}/files/startup.vpc` returns exactly the content of its file beside `main.tf`, and that content has the line `ip 10.0.0.1/24` (PC1) or `ip 10.0.0.2/24` (PC2).
- `GET /v2/projects/{project_id}/links` returns exactly one link, with the `link_id` Terraform reports, and its ends are PC1 and PC2.

## In the GNS3 GUI

`Invoke-Build Gns3Range -Scenario vpcs-startup-config -Inspect` opens the project and starts both nodes (step 1) for you, and prints the console links and the commands in steps 2 and 3 (from the `inspect` array in `scenario.json`). `-Destroy` tears it down.

While the scenario is applied, open `tf-scenario-vpcs-startup-config` (**File > Open project**). The canvas shows PC1 (left) and PC2 (right) joined by one cable, both stopped. To confirm the configuration took effect:

1. Start both nodes (**Control > Start/Resume all nodes**, or right-click each node > **Start**). The link indicators turn green.
2. Right-click PC1 > **Console**. `show ip` lists `10.0.0.1/24`.
3. In the same console, `ping 10.0.0.2` gets replies from PC2.

Stop the nodes before `destroy`, or let `destroy` delete them; after `destroy` the project is gone.
