# Changelog

Maintained by hand. GoReleaser does not generate release notes (`changelog.disable: true`).

## Unreleased

`gns3_node`:

- New `startup_config` attribute: day-zero configuration as text, for example `file("${path.module}/startup.vpc")`. The provider writes it through the node files endpoint, to `startup.vpc` on `vpcs` nodes and to `configs/i<dynamips_id>_startup-config.cfg` on `dynamips` nodes, and reads it back so drift shows in the plan. A change is applied in place and takes effect at the node's next start. Setting it on `qemu` (the disk image holds the config), `iou` and `docker` (not supported yet) or any other node type is an error at plan time. On `vpcs` it cannot be combined with `properties.startup_script`.
- `properties.startup_config_content` and `properties.private_config_content` are now refused on `dynamips` nodes. GNS3 2.2 accepts them at create, then fails every later update of the node with HTTP 400 (see `docs/api-drift.md`). Use `startup_config` instead.

Testing:

- Scenario tests under `test/scenarios/` (`project-only`, `two-nodes-one-link`, `image-upload`). Each one is a Terraform root that uses the published `JerryBalmer1/gns3` 0.1.0 from the Registry, plus a `checks.ps1` that checks the result through the GNS3 v2 API.
- `Invoke-Build Gns3Range [-Scenario <name>]` runs them against a live server (starting one when none answers). For each scenario it runs init, apply, output, the checks and destroy, prints a summary table and fails if any scenario failed. Destroy runs even when the checks fail.

## 0.1.0 - Unreleased

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
