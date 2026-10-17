# project-only

Up: [Scenario tests](../README.md)

See also: [image-upload](../image-upload/README.md), [two-nodes-one-link](../two-nodes-one-link/README.md), [template-roundtrip](../template-roundtrip/README.md), [vpcs-startup-config](../vpcs-startup-config/README.md)

Builds one empty `gns3_project` named `tf-scenario-project-only`, with `grid_size = 50`, a 3000 × 1500 scene and the grid shown.

## What checks.ps1 verifies

- `GET /v2/projects/{project_id}` answers for the `project_id` output.
- The server has the name and `filename` that Terraform reports, plus `grid_size` 50, `scene_width` 3000, `scene_height` 1500 and `show_grid` true.
- `GET /v2/projects` lists exactly one project with that id.

## In the GNS3 GUI

While the scenario is applied (before `destroy`), **File > Open project** lists `tf-scenario-project-only`. Opening it shows an empty canvas with the grid drawn at 50-pixel spacing and nothing on it. After `destroy` the project is gone from the list.
