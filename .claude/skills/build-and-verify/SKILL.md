---
# generic: copy to any Invoke-Build repo
name: build-and-verify
description: Build, lint and test a change through the repo's Invoke-Build tasks before staging it. Use for any code, test or docs change, and whenever asked to build, test or verify.
---

## When to use

Before staging any change, and whenever asked to build, test, verify or "run the checks".

## Steps

1. Read `CLAUDE.md` and the `## Unreleased` section of `CHANGELOG.md` (note it in the report if there is none). Then open only the files the task names, plus the files those point to.
2. **When an Invoke-Build task exists for a job, run the task, never the raw command.** `Invoke-Build ?` lists the tasks. If no task fits, or a task lacks a switch you need, report it under "Tooling gaps" instead of working around it.
3. Run in this order and stop at the first failure:
   1. `Invoke-Build Lint`
   2. `Invoke-Build Test`
   3. `Invoke-Build TestAcc -AutoStart` (starts and stops its own test server)
   4. `Invoke-Build <RangeTask> -Dev`, then `Invoke-Build <RangeTask>`. Placeholder: the repo's scenario or range task, named by `git config hooks.rangeTask` or in CLAUDE.md. Example, labelled as such: in terraform-provider-gns3 it is `Gns3Range`.
      - `-Dev` runs the scenarios with a build of this checkout. It proves the change.
      - Without `-Dev` they run the published release. That proves what users have now; expect it to lack anything unreleased.
      - Add `-Quiet` when the range task has it, so that only the summary table and failing steps reach the console. Example, labelled as such: `Invoke-Build Gns3Range -Dev -Quiet` writes each scenario's terraform and `checks.ps1` output to `$env:TEMP\gns3\range-logs\<scenario>.log` and, on a failure, prints the failing step and the last 40 lines of that log.
4. If the change touches an API call that replay fixtures cover, re-record the fixture from a live test server with the repo's fixture task rather than editing a response by hand. Example, labelled as such: `Invoke-Build RecordFixture -Name <area>/<name> -Method <verb> -Path <path> [-Body <json>] [-Source <docs URL>]` in terraform-provider-gns3, against a server `Gns3Start` started (the probe skill has the details).
5. If docs, examples or doc templates changed: `Invoke-Build Docs`, then `Invoke-Build Test-Docs`.
   If the GoReleaser config (`.goreleaser.yml`) or the release workflow changed: `Invoke-Build GoreleaserCheck` (pinned `goreleaser check`; config only, no build or release). `Release` also runs it.
6. Stage the changed files by name (`git add <paths>`). Never commit, push or tag.

## Stop and ask when

- A task fails twice, even after a fix.
- The fix would touch files the prompt did not name.
- A step needs a live environment that isn't available. Report the step as not run; never mark it passed.
- Anything would be deleted.

## Report

Use the report-template skill. Under "Checks run", list every task from step 3 with PASS, FAIL or NOT RUN (and why).
