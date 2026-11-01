# Git hooks

Up: [README](../README.md)

| Hook | What it runs |
|---|---|
| `pre-push` | The build before a push: `Release` for a tag, `Lint, Test, <hooks.rangeTask>` for `main`, `Lint, Test` for other branches. |
| `pre-commit` (shim for `pre-commit.ps1`) | The Loom gate: the staged tree must pass `go vet ./...` and `go test ./...`, and [terraform-provider-loom](https://github.com/JerryBalmer1/terraform-provider-loom) must build and test against it. |

## Install

Once per clone (`Invoke-Build Setup` does the same):

```powershell
git config core.hooksPath .githooks
```

The pre-commit gate needs `pwsh`, `go` and a Loom checkout with Invoke-Build installed. It finds Loom at `$env:LOOM_PATH`, else `git config hooks.loomPath`, else `C:\__Code\terraform-provider-loom`:

```powershell
git config hooks.loomPath D:\src\terraform-provider-loom
```

## What pre-commit does

1. `git write-tree` and `git checkout-index` export the staged tree (the index, not the working tree) to a temp dir. Unstaged edits are not checked.
2. `go vet ./...` and `go test ./...` in that dir.
3. `<loom>/.build.ps1 -Gns3Path <temp dir> -Gate`: Loom adds a temporary `replace github.com/JerryBalmer1/terraform-provider-gns3 => <temp dir>` to its `go.mod`, runs `go mod tidy`, `go build ./...` and `go test ./...`, then restores `go.mod` and `go.sum` byte for byte, pass or fail.
4. On pass it prints `LOOM GATE PASS at gns3 tree <hash>` and the commit goes ahead. On failure it prints the last 50 lines of Loom's output and blocks the commit.

The temp dir is removed afterwards. A full run takes several minutes (the provider tests dominate).

## Bypass

```powershell
git commit --no-verify
```

skips every hook. A commit made that way was **not** checked against Loom: say so in the commit message, and run the gate by hand before tagging (`pwsh <loom>/.build.ps1 -Gns3Path <this repo> -Gate`).
