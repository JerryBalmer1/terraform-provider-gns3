---
# generic: copy to any Invoke-Build repo
name: report-template
description: The fixed end-of-task report. Use it to close every task, including tasks that stopped early to ask a question.
---

## When to use

At the end of every task, and whenever you stop to ask. Use the headings below in this order. Never drop one; write "None." under a heading that has nothing.

## Steps

Write the report with exactly these headings:

```
## Summary
What was asked, what was done, and what state it is in now (staged / partly done / blocked). 2-5 lines.

## Files changed
One line per path: added / modified / deleted, and why. Say "staged" or "not staged".

## Checks run
One line per Invoke-Build task (or other command): name, PASS/FAIL, and the reason for any failure.
Name every check that was not run and why (for example: no live test server).

## Guesses
Every decision the prompt or docs did not settle, with the option chosen and the alternative.

## Breaking changes
Anything that makes a previously accepted config, command or workflow fail. Each one as a "Breaking:" line, or "None."

## Tooling gaps
Each build task that failed, lacked a switch, or forced a raw command:
- Task: <name>. Lacked: <what>. Proposed change: <the switch or task to add>.

## Questions
Numbered questions for the owner, the blocking ones first.
```

## Stop and ask when

- You can't fill "Checks run" honestly because a check could not run. Say so; never report it as passed.

## Report

The report is the last message of the task. Don't add a closing summary after it.
