<#
.Synopsis
	pre-commit gate: the staged gns3 tree must pass its own vet and tests, and
	terraform-provider-loom must build and test against it.

.Description
	1. Exports the staged tree (the index, not the working tree) to a temp dir.
	2. go vet ./... and go test ./... in that dir.
	3. Runs Loom's gate against it: <loom>/.build.ps1 -Gns3Path <dir> -Gate, which
	   adds a temporary replace to Loom's go.mod and always restores it.
	Any failure blocks the commit. See .githooks/README.md.

	Loom checkout: $env:LOOM_PATH, else `git config hooks.loomPath`, else
	C:\__Code\terraform-provider-loom.
#>
$ErrorActionPreference = 'Stop'

function Invoke-Step([string]$Name, [string]$Dir, [string]$Exe, [string[]]$Arguments) {
	Write-Host "pre-commit: $Name" -ForegroundColor Cyan
	Push-Location $Dir
	try { & $Exe @Arguments 2>&1 | Out-Host; $code = $LASTEXITCODE }
	finally { Pop-Location }
	if ($code -ne 0) {
		Write-Host "pre-commit: $Name failed (exit $code); commit blocked." -ForegroundColor Red
		exit 1
	}
}

$root = (git rev-parse --show-toplevel).Trim()
$loom = $env:LOOM_PATH
if (-not $loom) { $loom = (git config --get hooks.loomPath) }
if (-not $loom) { $loom = 'C:\__Code\terraform-provider-loom' }
$loomBuild = Join-Path $loom '.build.ps1'
if (-not (Test-Path -LiteralPath $loomBuild)) {
	Write-Host "pre-commit: Loom build script not found at $loomBuild; commit blocked. Set LOOM_PATH or git config hooks.loomPath." -ForegroundColor Red
	exit 1
}

# The staged tree. write-tree and checkout-index honour GIT_INDEX_FILE, so
# `git commit -a` and `git commit <paths>` are checked as they will be committed.
$tree = (git write-tree).Trim()
if ($LASTEXITCODE -ne 0) { Write-Host 'pre-commit: git write-tree failed; commit blocked.' -ForegroundColor Red; exit 1 }
$snap = Join-Path ([IO.Path]::GetTempPath()) "gns3-precommit-$($tree.Substring(0, 12))-$PID"
New-Item -ItemType Directory -Force $snap | Out-Null

try {
	git checkout-index --all --force "--prefix=$(($snap -replace '\\', '/').TrimEnd('/'))/"
	if ($LASTEXITCODE -ne 0) { Write-Host 'pre-commit: git checkout-index failed; commit blocked.' -ForegroundColor Red; exit 1 }

	# Git exports GIT_DIR, GIT_INDEX_FILE etc. to hooks. Loom's build runs git and
	# go (VCS stamping) in its own repo, so they must not see this repo's.
	foreach ($v in 'GIT_DIR', 'GIT_INDEX_FILE', 'GIT_WORK_TREE', 'GIT_PREFIX', 'GIT_COMMON_DIR', 'GIT_OBJECT_DIRECTORY') {
		Remove-Item "Env:$v" -ErrorAction SilentlyContinue
	}

	# a. gns3's own checks, on the staged tree.
	Invoke-Step 'go vet ./... (staged tree)' $snap go @('vet', './...')
	Invoke-Step 'go test ./... (staged tree)' $snap go @('test', '-count=1', './...')

	# b + c. Loom against the staged tree.
	Write-Host "pre-commit: Loom gate ($loomBuild -Gns3Path <staged tree> -Gate)" -ForegroundColor Cyan
	$out = & pwsh -NoProfile -NonInteractive -File $loomBuild -Gns3Path $snap -Gate 2>&1 | ForEach-Object { "$_" }
	$code = $LASTEXITCODE
	if ($code -eq 0) {
		Write-Host ($out | Select-Object -Last 1)
		Write-Host "LOOM GATE PASS at gns3 tree $tree" -ForegroundColor Green
		exit 0
	}
	Write-Host ($out | Select-Object -Last 50 | Out-String)
	Write-Host "LOOM GATE FAIL at gns3 tree $tree (exit $code); commit blocked." -ForegroundColor Red
	exit 1
}
finally {
	Remove-Item -Recurse -Force -LiteralPath $snap -ErrorAction SilentlyContinue
}
