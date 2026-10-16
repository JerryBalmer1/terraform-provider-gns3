---
# generic: copy to any Invoke-Build repo
name: docs
description: Where documentation lives and how pages link to each other (README showroom, docs/ manual, folder READMEs, Up and See also links, generated pages via templates). Use when writing or moving any .md, or deciding where new knowledge goes.
---

## When to use

Whenever you add, move or edit a `.md` file, or have learned something that should be written down.

## Steps

1. **Pick the home for new knowledge:**
   - Observed server or API behaviour that differs from its docs: the repo's drift page. Example, labelled as such: `docs/api-drift.md` here, with the server version and date.
   - Rules for agents and contributors: `CLAUDE.md`.
   - What a scenario proves: that scenario's `README.md`.
   - Anything a user of the project needs: `docs/`.
2. **README is the showroom, `docs/` is the manual.** The GitHub README sells and orients the reader; `docs/` explains. Badges, banners and HTML go only in the GitHub README, never in `docs/index.md` (the Terraform Registry sanitises them).
3. **Every nested folder has a short README** listing what it contains, one line per item.
4. **Every `.md` gets an `Up:` line** linking to its parent page, and a `See also:` line linking to its siblings, both right under the H1. Exception: pages generated for a registry (here `docs/index.md`, `docs/resources/`, `docs/data-sources/`) carry none, because their relative links may not resolve there.
5. **Generated pages are never edited.** Resource and data-source pages are tfplugindocs output. Change the schema descriptions, `examples/` or `templates/`, then run `Invoke-Build Docs`.
6. Run `Invoke-Build Test-Docs`. It fails on any relative link that does not resolve and on any page under `docs/` or `test/scenarios/` without an `Up:` line, apart from the generated registry pages.

## Stop and ask when

- The right home for the knowledge is unclear, or it would duplicate an existing page.
- A change would move or delete a page that other pages link to.

## Report

Use the report-template skill. List each page added or moved and where its `Up:` line points.
