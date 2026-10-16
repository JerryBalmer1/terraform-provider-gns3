# Scenario tests

Up: [README](../../README.md)

See also: [CLAUDE.md "Scenario tests"](../../CLAUDE.md#scenario-tests), [manual check list](../../manual-check-list.md), [Looking at a scenario in the GUI](../../docs/manual-steps.md)

Each folder is one Terraform root that uses the published `JerryBalmer1/gns3` from the Registry, plus a `checks.ps1` that checks the result through the GNS3 v2 API. `Invoke-Build Gns3Range [-Scenario <name>]` runs them against a live server: init, apply, output, checks, destroy.

## Inspect a scenario

```powershell
Invoke-Build Gns3Range -Scenario vpcs-startup-config -Inspect    # add -Dev for a build of this checkout
Invoke-Build Gns3Range -Scenario vpcs-startup-config -Destroy    # the command -Inspect prints last
```

`-Inspect` runs init, apply and `checks.ps1`, then skips destroy. It starts every node (`POST /v2/projects/{project_id}/nodes/start`), opens the project in gns3.exe (when gns3.exe is already running it says to use **File > Open project**; it prints the web UI address when gns3.exe is not installed or the server is remote) and leaves the server running. It then prints a numbered "what to do now" block: one `telnet://host:port` line per node, read from the API; the scenario's `inspect` commands and expected results; and the teardown command.

`-Destroy` destroys the scenario, checks that its state is empty and removes `terraform.tfstate` and its backup. It stops the server only when the `-Inspect` run started it and no other scenario is still inspected. A default run on a scenario still left up fails at `state` and names the `-Destroy` command. Both runs appear in the summary table, in the `Action` column (`inspect`, `destroy`; `run` for a default run). [docs/manual-steps.md](../../docs/manual-steps.md) lists which manual steps this replaces.

`scenario.json` may carry an optional `inspect` array, printed by `-Inspect` and never run by the build:

```json
"inspect": [
  { "node": "PC1", "command": "show ip", "expect": "IP/MASK     : 10.0.0.1/24" }
]
```

| Scenario | Requires | What it builds |
|---|---|---|
| [image-upload](image-upload/README.md) | server, image | A generated 1 MiB dummy file uploaded to the local compute's qemu images with `gns3_image` |
| [project-only](project-only/README.md) | server | One empty `gns3_project` with non-default grid and scene settings |
| [two-nodes-one-link](two-nodes-one-link/README.md) | server | A project with two VPCS nodes from the built-in VPCS template, joined by one link |
| [vpcs-startup-config](vpcs-startup-config/README.md) | server | Two linked VPCS nodes whose `startup.vpc` is set through `startup_config` |
