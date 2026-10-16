---
# generic: copy to any Invoke-Build repo
name: release-prep
description: Prepare a release (changelog heading and date, version bump, docs regen, scenario pin bump, goreleaser check) up to the point where the owner tags. Proposes the version from the changelog and asks the owner to confirm it.
---

## When to use

When asked to prepare, cut or get ready for a release. The owner (Jerry) tags. You never tag, push or run `goreleaser release`.

## Steps

1. Read `CLAUDE.md` and the `## Unreleased` section of `CHANGELOG.md`. Sort each entry into fix, addition, or plan-breaking.
2. **Propose the version** with the pre-1.0 semver rule:
   - patch (0.x.Y): fixes only.
   - minor (0.X.0): any new attribute, resource or data source, **or** anything that makes a previously accepted config fail at plan (removed or renamed attribute, changed default, new validation).
   - major (1.0.0): only when the owner declares 1.0.
   Every plan-breaking entry must have its own `Breaking:` line in the changelog; add one where it is missing.
3. **Ask the owner to confirm the version.** Show the proposal and the entries that decided it. Don't edit anything until it is confirmed.
4. Rename `## Unreleased` to `## X.Y.Z - YYYY-MM-DD` (today's date).
5. Bump the version everywhere the repo records it. CLAUDE.md names the places. Example, labelled as such: in terraform-provider-gns3 that is `var version` in `main.go`, the README and CLAUDE.md, manual-check-list item 0.1, every `test/scenarios/*/main.tf` pin, and the lock-file version that `Gns3Range` checks.
6. `Invoke-Build Docs`, then `Invoke-Build Test-Docs`.
7. `goreleaser check` (validates the config only).
8. Run the build-and-verify skill. Stage the files. Stop: Jerry tags.

## Stop and ask when

- The changelog has no `## Unreleased` section, or an entry's category is unclear.
- The bump is breaking (minor before 1.0 because of a plan-breaking entry, or any major).
- `<RangeTask> -Dev` fails. The `Release` task runs the range task in `-Dev` mode, against a build of this checkout, because the version being tagged is not on the Registry yet. The published mode can only pass for the new version after it is released.

## Report

Use the report-template skill. Put the confirmed version and the deciding entries under Summary, and every `Breaking:` line under "Breaking changes".
