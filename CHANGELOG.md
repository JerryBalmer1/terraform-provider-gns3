# Changelog

Maintained by hand. GoReleaser does not generate release notes (`changelog.disable: true`).

## 0.3.0 - 2026-11-04

Actions, list resources, resource identity and provider functions. Existing configurations keep working on older Terraform; only the new kinds need a newer one (actions and list resources: Terraform 1.14+; identity: 1.12+; functions: 1.8+).

New actions (Terraform 1.14+, run from `lifecycle { action_trigger { ... } }` or `terraform apply -invoke=action.<type>.<name>`):

- `gns3_start_nodes`, `gns3_stop_nodes`, `gns3_suspend_nodes`, `gns3_reload_nodes`: `project_id` (required) and `node_ids` (optional set). With no `node_ids` the action makes one project-wide call (`POST /v2/projects/{id}/nodes/{start|stop|suspend|reload}`); otherwise it calls each node in sorted order.
- `gns3_snapshot`: `project_id`, `name` and `restore` (default `false`). `false` creates a snapshot named `name` (GNS3 refuses a duplicate name with 409); `true` restores the one snapshot with that name and fails when there is none or more than one. On GNS3 2.2.61 a restore leaves every node stopped.
- On GNS3 2.2.61, VPCS has no suspend: `gns3_suspend_nodes` succeeds and VPCS nodes keep running. The action re-reads each targeted node after the call and warns for every node whose status did not change ("suspend had no effect on PC1 (vpcs); ..."); nodes that were already suspended are not warned about. On a closed project the project-wide calls fail with 403 "The project is not opened". Details in `docs/api-drift.md` ("Node actions and snapshots (2.2.61)").

New list resources (Terraform 1.14+, `terraform query`):

- `gns3_project` (no arguments), `gns3_node` (`project_id` required) and `gns3_template` (optional `template_type` filter; built-in templates are left out unless `include_builtin = true`, because `gns3_template` can't manage or import them). Each result carries the resource identity and, with `include_resource = true`, the full resource.

Resource identity (Terraform 1.12+):

- `gns3_project` {`project_id`}, `gns3_node` {`project_id`, `node_id`}, `gns3_link` {`project_id`, `link_id`}, `gns3_template` {`template_id`} and `gns3_image` {`compute_id`, `emulator`, `filename`}. Import works with an `import` block's `identity` and still with the existing ID strings. State written by 0.2.0 gains its identity on the next refresh, and the plan stays empty (`TestAccIdentityUpgradeFrom020`).

New provider functions:

- `provider::gns3::console_address(host, console)` returns `host:port` (IPv6 hosts in brackets); `provider::gns3::node_name(prefix, index)` returns `prefix-01` (index zero-padded to two digits).

`pkg/gns3`:

- `NodesAPI` gains `ReloadNode`, `SuspendNode`, `StartAllNodes`, `StopAllNodes`, `SuspendAllNodes` and `ReloadAllNodes`. New `SnapshotsAPI` (`ListSnapshots`, `CreateSnapshot`, `RestoreSnapshot`, `DeleteSnapshot`, type `Snapshot`), embedded in `Client`. Their fixtures are recorded from GNS3 2.2.61, including the 409 on a duplicate snapshot name and the 403 on a closed project.
- **Breaking** (`pkg/gns3` only, not the provider): code outside this module that implements `NodesAPI` or `Client` must add the new methods.

Tests, docs and build:

- New scenarios `actions-start-stop` (an `after_create` trigger starts two linked VPCS nodes, then `checks.ps1` runs `terraform apply -invoke` on `gns3_stop_nodes`) and `query-adopt` (`terraform query` finds the applied project). Both set `minVersion` 0.3.0.
- New examples under `examples/actions/`, `examples/list-resources/` (`list-resource.tfquery.hcl`) and `examples/functions/`, plus `import-by-identity.tf` for each resource; tfplugindocs generates `docs/actions/`, `docs/list-resources/` and `docs/functions/`.
- `.build.ps1`: terraform is found through `$env:TERRAFORM_EXE`, then the default Windows install path, then `PATH`, and Test, TestAcc, Docs, Checklist and Gns3Range all use it. New `Preflight` task prints the go, terraform, framework and plugin-testing versions and fails below Terraform 1.14. `Test-Docs` also exempts the generated `docs/actions/`, `docs/list-resources/`, `docs/functions/` and `docs/ephemeral-resources/` pages from the `Up:` rule.
- `Gns3Range -Quiet` prints only the summary table and failing steps; each scenario's terraform and `checks.ps1` output goes to `$env:TEMP\gns3\range-logs\<scenario>.log`, and a failure prints the last 40 lines of it.
- New `RecordFixture` task (`-Name <area>/<name> -Method -Path [-Body] [-Source]`): makes one call to the server `Gns3Start` started and writes it to `pkg/gns3/testdata/` with `_header`, `_source` and the live response.
- CI runs the tests against Terraform 1.14.x and the latest release. The check is now named `test (terraform 1.14.x)` / `test (terraform latest)`.
- New acceptance test `TestAccIdentityUpgradeFrom020` downloads the published 0.2.0 provider from the Registry, applies with it, then plans with this checkout and requires an empty plan.

New resource `gns3_template`:

- Manages a user template: `name`, `template_type`, `compute_id` (default `local`) and `properties`, a JSON object of type-specific fields (`ram`, `adapters`, `hda_disk_image`, ...) where only the keys you set are compared with the server. `template_id`, `builtin`, `category`, `symbol` and `default_name_format` are read from the server. Import by `template_id`.
- `gns3_node.template_id` can reference `gns3_template.<name>.template_id`.
- `name` and `properties` change in place. `template_type` and `compute_id` force a new template, because GNS3 2.2 ignores them in an update. GNS3 2.2 does not check type-specific keys in an update either, so a `properties` change is first tried on a throwaway template (`tf-validate-<random>`, deleted straight away) and refused if GNS3 rejects it.
- At plan time, `dynamips` needs `properties.platform` and `properties.image`, `docker` needs `properties.image`, and common fields in `properties` are refused. Built-in templates can't be imported or managed.
- What GNS3 2.2.61 does on template create, update and delete is in `docs/api-drift.md` ("Template create, update and delete (2.2.61)").
- `pkg/gns3`: `TemplatesAPI` gains `CreateTemplate`, `UpdateTemplate` and `DeleteTemplate`, and `Template.Properties` holds the type-specific keys.
- **Breaking** (`pkg/gns3` only, not the provider): `gns3.Template` has a new map field, `Properties`, so Go code that compares two `Template` values with `==` or uses one as a map key no longer compiles. Code outside this module that implements `TemplatesAPI` must add the three new methods.

Lab template and tests:

- New `lab-template/`: a root module to copy for your own lab (project, a `gns3_template`, two nodes with startup configs from `configs/`, one link), with a README. `Test-Docs` now requires `Up:` lines under `lab-template/` too.
- New scenario `template-roundtrip`: a vpcs `gns3_template` and one node created from it. It sets `minVersion` 0.3.0, so published runs skip it until the pins are bumped.
- `scenario.json` takes an optional `minVersion`. A published `Gns3Range` skips a scenario whose `main.tf` pins a lower provider version (`SKIP`, "requires provider >= X, pinned Y"); a skip does not fail the run. `-Dev` ignores it. The summary line counts skipped scenarios.
- New example `examples/resources/gns3_template`, applied by `TestExamples` and `TestAccExamples`.

`gns3_node`:

- The qemu `startup_config` error now explains that GNS3 2.2 config disks don't work on Windows and points to Ansible or cloud-init.

Agent and contributor tooling:

- Skills under `.claude/skills/`: `build-and-verify`, `release-prep`, `docs` and `report-template` (generic, for any Invoke-Build repo), plus `probe` and `add-scenario` (this repo).
- `.claude/settings.json`: a PreToolUse hook (`.claude/hooks/block-git-write.ps1`) blocks agent shell commands that run `git commit`, `git push` or `git tag`. `git tag -l` and `git tag --list` are allowed.
- `.githooks/pre-push` runs the build before a push: `Invoke-Build Release` when a tag is pushed, `Lint`, `Test` and `Gns3Range` for `main`, and `Lint` and `Test` for any other branch. A failing task blocks the push.
- pre-commit hook runs only this repository's vet and tests
- `CLAUDE.md` gains generic working rules: when to stop and ask, edit and stage only, the report template, and reporting tooling gaps.

Build (`.build.ps1`):

- New `Setup` task: sets `core.hooksPath=.githooks` and `hooks.rangeTask=Gns3Range` in this clone's git config.
- New `Test-Docs` task, also run in CI after the docs step. It fails on any relative link or image in a `.md` that does not resolve, and on any page under `docs/` or `test/scenarios/` without an `Up:` line. The generated Registry pages are exempt from the `Up:` rule.
- New `Release` task: `Lint`, `Test`, `Gns3Range -Dev` and `Test-Docs`, then a check that `docs/screenshots` has no uncommitted changes.
- `Gns3Range` runs `terraform init -upgrade`. It fails a scenario without applying or destroying when its state still holds resources from an earlier run.
- New `Gns3Range -Dev`: runs the scenarios with a `go build` of the checkout through a `dev_overrides` CLI config of its own, skips the lock-file version check and prints `DEV BUILD` in the summary.
- New `Gns3Range -Capture`: a stub that prints "capture not implemented". `docs/screenshots/` is tracked for it.
- New `Gns3Range -Scenario <name> -Inspect`: applies and checks one scenario, skips destroy, starts its nodes (`POST /v2/projects/{project_id}/nodes/start`), opens the project in gns3.exe unless one is already running (otherwise it prints where to open it, including the web UI address), leaves the server running and prints a numbered "what to do now" block with a `telnet://host:port` line per node, the scenario's expected commands and results, and the teardown command.
- New `Gns3Range -Scenario <name> -Destroy`: destroys a scenario left up by `-Inspect`, removes its state files and stops the server only if that `-Inspect` run started it. The stale-state refusal now names this command.
- The `Gns3Range` summary table has an `Action` column (`run`, `inspect`, `destroy`).
- `scenario.json` takes an optional `inspect` array (`node`, `command`, `expect`), printed by `-Inspect`. `vpcs-startup-config` has two: PC1 `show ip` and PC1 `ping 10.0.0.2`.
- The script now loads on Linux (CI): the GNS3 install folder is only resolved on Windows, and the GNS3 server tasks fail there with "GNS3 server tasks run on Windows only".

Docs:

- Hand-written pages under `docs/` and each scenario README have `Up:` and `See also:` lines. `test/scenarios/README.md` lists the scenarios.
- `templates/` holds the tfplugindocs default templates, unchanged.
- New hand-written `docs/manual-steps.md`: each step of looking at a scenario in the GUI by hand, and whether `-Inspect` automates it, prints it or leaves it manual.

## 0.2.0 - 2026-10-15

`gns3_node`:

- New `startup_config` attribute: day-zero configuration as text, for example `file("${path.module}/startup.vpc")`. The provider writes it through the node files endpoint, to `startup.vpc` on `vpcs` nodes and to `configs/i<dynamips_id>_startup-config.cfg` on `dynamips` nodes, and reads it back so drift shows in the plan. A change is applied in place and takes effect at the node's next start. Setting it on `qemu` (the disk image holds the config), `iou` and `docker` (not supported yet) or any other node type is an error at plan time. On `vpcs` it cannot be combined with `properties.startup_script`.
- Breaking: `properties.startup_config_content` and `properties.private_config_content` are now refused on `dynamips` nodes. GNS3 2.2 accepts them at create, then fails every later update of the node with HTTP 400 (see `docs/api-drift.md`). Use `startup_config` instead.

Testing:

- Scenario tests under `test/scenarios/` (`project-only`, `two-nodes-one-link`, `image-upload`, `vpcs-startup-config`). Each one is a Terraform root that uses the published `JerryBalmer1/gns3` from the Registry (pinned to 0.2.0), plus a `checks.ps1` that checks the result through the GNS3 v2 API.
- `Invoke-Build Gns3Range [-Scenario <name>]` runs them against a live server (starting one when none answers). For each scenario it runs init, apply, output, the checks and destroy, prints a summary table and fails if any scenario failed. Destroy runs even when the checks fail.

## 0.1.0 - 2026-10-15

First release. Targets GNS3 server 2.2.x over the REST v2 API.

Resources:

- `gns3_project`
- `gns3_node`
- `gns3_link`
- `gns3_image`

Data sources:

- `gns3_template`
- `gns3_compute`
- `gns3_project`
- `gns3_version`
- `gns3_image`

The GNS3 REST client is a public Go package, `github.com/JerryBalmer1/terraform-provider-gns3/pkg/gns3`.

CI:

- GitHub Actions moved to current majors (checkout v7, setup-go v7, setup-terraform v4, goreleaser-action v7, ghaction-import-gpg v7) and pinned to commit SHAs with a version comment.
- Runner image pinned to `ubuntu-24.04`.
- Dependabot updates actions and Go modules weekly, with minor and patch updates grouped.
