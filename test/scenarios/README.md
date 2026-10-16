# Scenario tests

Up: [README](../../README.md)

See also: [CLAUDE.md "Scenario tests"](../../CLAUDE.md#scenario-tests), [manual check list](../../manual-check-list.md)

Each folder is one Terraform root that uses the published `JerryBalmer1/gns3` from the Registry, plus a `checks.ps1` that checks the result through the GNS3 v2 API. `Invoke-Build Gns3Range [-Scenario <name>]` runs them against a live server: init, apply, output, checks, destroy.

| Scenario | Requires | What it builds |
|---|---|---|
| [image-upload](image-upload/README.md) | server, image | A generated 1 MiB dummy file uploaded to the local compute's qemu images with `gns3_image` |
| [project-only](project-only/README.md) | server | One empty `gns3_project` with non-default grid and scene settings |
| [two-nodes-one-link](two-nodes-one-link/README.md) | server | A project with two VPCS nodes from the built-in VPCS template, joined by one link |
| [vpcs-startup-config](vpcs-startup-config/README.md) | server | Two linked VPCS nodes whose `startup.vpc` is set through `startup_config` |
