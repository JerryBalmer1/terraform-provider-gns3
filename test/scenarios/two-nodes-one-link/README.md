# two-nodes-one-link

Up: [Scenario tests](../README.md)

See also: [image-upload](../image-upload/README.md), [project-only](../project-only/README.md), [template-roundtrip](../template-roundtrip/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Builds a project `tf-scenario-two-nodes-one-link` with two VPCS nodes, `PC1` at (-150, 0) and `PC2` at (150, 0), made from the built-in `VPCS` template (`gns3_template` data source), and one `gns3_link` between the first port of each (`ports[0]`, which is `Ethernet0`, adapter 0 port 0 on VPCS). The nodes are not started.

## What checks.ps1 verifies

- The project exists under the `project_id` output and has the expected name.
- `GET /v2/projects/{project_id}/nodes` returns exactly two nodes, `PC1` and `PC2`, each with the `node_id` Terraform reports, `node_type` `vpcs` and `status` `stopped`.
- `GET /v2/projects/{project_id}/links` returns exactly one link, with the `link_id` Terraform reports and `link_type` `ethernet`. Its two ends are PC1 and PC2, on the adapter and port numbers Terraform read from each node's `ports[0]`.

## In the GNS3 GUI

While the scenario is applied, open `tf-scenario-two-nodes-one-link` (**File > Open project**). The canvas shows two computer icons labelled PC1 (left) and PC2 (right), joined by one cable with red (stopped) link indicators at both ends. The topology summary lists both nodes as stopped. After `destroy` the project is gone.
