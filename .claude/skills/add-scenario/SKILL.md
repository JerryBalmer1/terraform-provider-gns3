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
   - `scenario.json`: `{ "name": "<name>", "description": "<one sentence>", "requires": ["server"] }`, or `["server","image"]` to get `test/images/scenario-dummy.img`.
   - `main.tf`: a real root module. `required_providers` uses `source = "JerryBalmer1/gns3"` and `version` = the **published** version the other scenarios pin. The provider block is empty. Resources are named `tf-scenario-<name>`, and every value `checks.ps1` needs is an `output`.
   - `checks.ps1`: `param([Parameter(Mandatory)] [string] $Server, [Parameter(Mandatory)] [hashtable] $Outputs)` and `$ErrorActionPreference = 'Stop'`. Call the v2 API with `Invoke-RestMethod`, unroll lists with `| ForEach-Object { $_ }`, cite each endpoint's docs URL in a comment, and throw a message naming the field, the expected value and the server's value on any mismatch.
   - `README.md`: H1 `# <name>`, then `Up: [Scenario tests](../README.md)` and `See also:` the sibling scenarios, then what it builds, "What checks.ps1 verifies" and "In the GNS3 GUI" (what Jerry sees while it is applied and after destroy).
3. Add a row to `test/scenarios/README.md`, and a `See also:` link to the new scenario in each sibling README.
4. `Invoke-Build Test-Docs`, then `Invoke-Build Gns3Range -Dev -Scenario <name>` (a build of this checkout) and `Invoke-Build Gns3Range -Scenario <name>` (the pinned Registry release; it passes only once the behaviour is released).

## Stop and ask when

- What the scenario should assert is still unclear after reading the schema and docs. **Ask rather than invent expected values.**
- The behaviour exists only in unreleased code: say in the scenario README that the published mode will fail until the release.
- The scenario would leave anything on the server that destroy can't remove (images, for example).

## Report

Use the report-template skill. List under Summary what the scenario proves and both `Gns3Range -Scenario` results (`-Dev` and published).
