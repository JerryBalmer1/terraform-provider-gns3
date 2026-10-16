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
4. If docs, examples or doc templates changed: `Invoke-Build Docs`, then `Invoke-Build Test-Docs`.
5. Stage the changed files by name (`git add <paths>`). Never commit, push or tag.

## Stop and ask when

- A task fails twice, even after a fix.
- The fix would touch files the prompt did not name.
- A step needs a live environment that isn't available. Report the step as not run; never mark it passed.
- Anything would be deleted.

## Report

Use the report-template skill. Under "Checks run", list every task from step 3 with PASS, FAIL or NOT RUN (and why).
