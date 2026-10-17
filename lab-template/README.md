# Lab template

Up: [README](../README.md)

See also: [gns3_template](../docs/resources/template.md), [Scenario tests](../test/scenarios/README.md), [Emulator images](../docs/images.md)

A small, complete Terraform root module to start a GNS3 lab from: a project, one lab template (`gns3_template`, VPCS), two nodes created from it with day-zero configs, and one link between them. It needs no image.

| Path | What it is |
|---|---|
| [main.tf](main.tf) | provider, project, template, two nodes, one link, outputs |
| [configs/](configs/README.md) | one startup config per node, read with `file()` |

## Use it

1. **Copy this folder** somewhere of your own, for example `Copy-Item -Recurse lab-template C:\labs\my-lab`.
2. **Edit** `main.tf`: set `local.lab` (it names the project and the template), narrow the provider `version` (it takes `>= 0.3.0`, the first release with `gns3_template`) to the release you use, and add nodes and links. Edit or add files under `configs/` (`startup_config` supports `vpcs` and `dynamips` nodes).
3. **Apply**: `terraform init`, then `terraform apply`. Open the project in the GNS3 GUI (**File > Open project**) and start the nodes; each one reads its config at start.
4. **Destroy**: `terraform destroy` removes the link, the nodes, the project and the lab template. The built-in templates and any images stay.

`gns3_template` is not in the published 0.2.0. Until the next release, run it with a build of this checkout through `dev_overrides` (CLAUDE.md, "Local dev loop") and skip `terraform init`.

## How it is tested

`TestExamples` does not apply this folder. It applies `examples/resources/*/resource.tf` under its own provider block pointed at the in-memory fake, and this folder is a root module with its own `terraform` and `provider` blocks. Everything it uses is covered elsewhere: the `gns3_template` example and `TestTemplateResource*` cover the template, a node created from it and the destroy; `TestNodeStartupConfig*` and the `vpcs-startup-config` scenario cover `startup_config` and the link. `Invoke-Build Test-Docs` checks this page's links.
