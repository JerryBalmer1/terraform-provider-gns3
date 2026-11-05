<#
.Synopsis
	pre-commit gate: the staged gns3 tree must pass its own vet and tests.

.Description
	1. Exports the staged tree (the index, not the working tree) to a temp dir.
	2. go vet ./... and go test ./... in that dir.
	Any failure blocks the commit. See .githooks/README.md.
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

# The staged tree. write-tree and checkout-index honour GIT_INDEX_FILE, so
# `git commit -a` and `git commit <paths>` are checked as they will be committed.
$tree = (git write-tree).Trim()
if ($LASTEXITCODE -ne 0) { Write-Host 'pre-commit: git write-tree failed; commit blocked.' -ForegroundColor Red; exit 1 }
$snap = Join-Path ([IO.Path]::GetTempPath()) "gns3-precommit-$($tree.Substring(0, 12))-$PID"
New-Item -ItemType Directory -Force $snap | Out-Null

try {
	git checkout-index --all --force "--prefix=$(($snap -replace '\\', '/').TrimEnd('/'))/"
	if ($LASTEXITCODE -ne 0) { Write-Host 'pre-commit: git checkout-index failed; commit blocked.' -ForegroundColor Red; exit 1 }

	Invoke-Step 'go vet ./... (staged tree)' $snap go @('vet', './...')
	Invoke-Step 'go test ./... (staged tree)' $snap go @('test', '-count=1', './...')
	exit 0
}
finally {
	Remove-Item -Recurse -Force -LiteralPath $snap -ErrorAction SilentlyContinue
}
