# Git hooks

Up: [README](../README.md)

| Hook | What it runs |
|---|---|
| `pre-push` | The build before a push: `Release` for a tag, `Lint, Test, <hooks.rangeTask>` for `main`, `Lint, Test` for other branches. |
| `pre-commit` (shim for `pre-commit.ps1`) | The staged tree must pass `go vet ./...` and `go test ./...`. |

## Install

Once per clone (`Invoke-Build Setup` does the same):

```powershell
git config core.hooksPath .githooks
```

## What pre-commit does

1. `git write-tree` and `git checkout-index` export the staged tree (the index, not the working tree) to a temp dir. Unstaged edits are not checked.
2. `go vet ./...` and `go test ./...` in that dir.

The temp dir is removed afterwards. A full run takes several minutes (the provider tests dominate).

## Bypass

```powershell
git commit --no-verify
```

skips every hook.
