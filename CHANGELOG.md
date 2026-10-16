# Changelog

Maintained by hand. GoReleaser does not generate release notes (`changelog.disable: true`).

## Unreleased

No provider changes.

Agent and contributor tooling:

- Skills under `.claude/skills/`: `build-and-verify`, `release-prep`, `docs` and `report-template` (generic, for any Invoke-Build repo), plus `probe` and `add-scenario` (this repo).
- `.claude/settings.json`: a PreToolUse hook (`.claude/hooks/block-git-write.ps1`) blocks agent shell commands that run `git commit`, `git push` or `git tag`. `git tag -l` and `git tag --list` are allowed.
- `.githooks/pre-push` runs the build before a push: `Invoke-Build Release` when a tag is pushed, `Lint`, `Test` and `Gns3Range` for `main`, and `Lint` and `Test` for any other branch. A failing task blocks the push.
- `CLAUDE.md` gains generic working rules: when to stop and ask, edit and stage only, the report template, and reporting tooling gaps.

Build (`.build.ps1`):

- New `Setup` task: sets `core.hooksPath=.githooks` and `hooks.rangeTask=Gns3Range` in this clone's git config.
- New `Test-Docs` task, also run in CI after the docs step. It fails on any relative link or image in a `.md` that does not resolve, and on any page under `docs/` or `test/scenarios/` without an `Up:` line. The generated Registry pages are exempt from the `Up:` rule.
- New `Release` task: `Lint`, `Test`, `Gns3Range -Dev` and `Test-Docs`, then a check that `docs/screenshots` has no uncommitted changes.
- `Gns3Range` runs `terraform init -upgrade`. It fails a scenario without applying or destroying when its state still holds resources from an earlier run.
- New `Gns3Range -Dev`: runs the scenarios with a `go build` of the checkout through a `dev_overrides` CLI config of its own, skips the lock-file version check and prints `DEV BUILD` in the summary.
- New `Gns3Range -Capture`: a stub that prints "capture not implemented". `docs/screenshots/` is tracked for it.
- The script now loads on Linux (CI): the GNS3 install folder is only resolved on Windows, and the GNS3 server tasks fail there with "GNS3 server tasks run on Windows only".

Docs:

- Hand-written pages under `docs/` and each scenario README have `Up:` and `See also:` lines. `test/scenarios/README.md` lists the scenarios.
- `templates/` holds the tfplugindocs default templates, unchanged.

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
