---
# repo-specific: terraform-provider-gns3
name: add-scenario
description: Add a scenario test under test/scenarios/ (scenario.json, main.tf pinned to the published provider, checks.ps1, README with GUI expectations) for a resource, attribute or behaviour not yet covered.
---

## When to use

When asked to add a scenario, or when a released behaviour has no scenario.

## Steps

1. Read the schema (`internal/provider/<name>_resource.go` or `_data_source.go`), its page under `docs/`, and the "Scenario tests" section of `CLAUDE.md`. Check `test/scenarios/README.md` so you don't duplicate an existing scenario.
2. Create `test/scenarios/<name>/`:
   - `scenario.json`: `{ "name": "<name>", "description": "<one sentence>", "requires": ["server"] }`, or `["server","image"]` to get `test/images/scenario-dummy.img`. When the scenario has nodes with a console, add an optional `"inspect": [{ "node", "command", "expect" }]` array: the command to type in a node's console and the output to compare it with. `Gns3Range -Inspect` prints it and never runs it. Put in `expect` only output you have seen on a running node; otherwise ask. When the scenario uses a resource or attribute that is not in the pinned release yet, add `"minVersion": "<first release with it>"`: a published `Gns3Range` then skips it (`requires provider >= X, pinned Y`) instead of failing, and `-Dev` still runs it. Leave the pin at the published version; release-prep bumps it.
   - `main.tf`: a real root module. `required_providers` uses `source = "JerryBalmer1/gns3"` and `version` = the **published** version the other scenarios pin. The provider block is empty. Resources are named `tf-scenario-<name>`, and every value `checks.ps1` needs is an `output`. Use a built-in template (the `gns3_template` data source) when one fits. When the scenario needs any other template, declare it with the `gns3_template` resource, named `tf-scenario-<name>`, so destroy deletes it again (see `template-roundtrip`). Don't rely on a template made by hand in the GUI.
   - `checks.ps1`: `param([Parameter(Mandatory)] [string] $Server, [Parameter(Mandatory)] [hashtable] $Outputs)` and `$ErrorActionPreference = 'Stop'`. Call the v2 API with `Invoke-RestMethod`, unroll lists with `| ForEach-Object { $_ }`, cite each endpoint's docs URL in a comment, and throw a message naming the field, the expected value and the server's value on any mismatch.
   - `README.md`: H1 `# <name>`, then `Up: [Scenario tests](../README.md)` and `See also:` the sibling scenarios, then what it builds, "What checks.ps1 verifies" and "In the GNS3 GUI" (what Jerry sees while it is applied and after destroy).
3. Add a row to `test/scenarios/README.md`, and a `See also:` link to the new scenario in each sibling README.
4. `Invoke-Build Test-Docs`, then `Invoke-Build Gns3Range -Dev -Scenario <name>` (a build of this checkout) and `Invoke-Build Gns3Range -Scenario <name>` (the pinned Registry release; it passes once the behaviour is released, and shows `SKIP` until then when `minVersion` is set).
5. If the scenario has an `inspect` array: `Invoke-Build Gns3Range -Scenario <name> -Inspect` (add `-Dev` before the release), run each command in the printed consoles and compare with `expect`, then run the printed `-Destroy` command. Never leave a scenario inspected at the end of a task: a default run refuses it as stale state.

## Stop and ask when

- What the scenario should assert is still unclear after reading the schema and docs. **Ask rather than invent expected values.**
- The behaviour exists only in unreleased code and you can't tell which release will ship it (the `minVersion` to set). Ask; then say in the scenario README that the published mode skips it until that release.
- The scenario would leave anything on the server that destroy can't remove (images, for example).

## Report

Use the report-template skill. List under Summary what the scenario proves and both `Gns3Range -Scenario` results (`-Dev` and published).
