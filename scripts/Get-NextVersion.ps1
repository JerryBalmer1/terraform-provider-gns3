<#
.Synopsis
	Prints the next semver after the one in VERSION. No side effects.

.Example
	./scripts/Get-NextVersion.ps1              # 0.2.0 -> 0.2.1
	./scripts/Get-NextVersion.ps1 -Bump minor  # 0.2.0 -> 0.3.0
#>
param(
	[ValidateSet('major', 'minor', 'patch')]
	[string]$Bump = 'patch',
	[string]$Path = (Join-Path $PSScriptRoot '../VERSION')
)

$ErrorActionPreference = 'Stop'
$current = (Get-Content -Raw -LiteralPath $Path).Trim()
if ($current -notmatch '^(\d+)\.(\d+)\.(\d+)$') {
	throw "VERSION holds '$current', expected MAJOR.MINOR.PATCH"
}
$major, $minor, $patch = [int]$Matches[1], [int]$Matches[2], [int]$Matches[3]
switch ($Bump) {
	'major' { $major++; $minor = 0; $patch = 0 }
	'minor' { $minor++; $patch = 0 }
	'patch' { $patch++ }
}
"$major.$minor.$patch"
