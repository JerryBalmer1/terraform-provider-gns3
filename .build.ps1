[CmdletBinding()]
Param(
    # Gns3Install: the GNS3 release to install, x.y.z or 'latest'.
    [string]
    $Version = 'latest',

    # Gns3Install: download and run the installer even when GNS3 is already installed.
    [switch]
    $Force,

    # TestAcc: start a server with Gns3Start when nothing answers.
    [switch]
    $AutoStart,

    # Gns3Status, Gns3Start, TestAcc: where the GNS3 server answers.
    [string]
    $Gns3Host = $(if ($env:GNS3_HOST) { $env:GNS3_HOST } else { '127.0.0.1' }),

    [int]
    $Gns3Port = $(if ($env:GNS3_PORT) { [int]$env:GNS3_PORT } else { 3080 }),

    # Checklist: the manual-check-list.md item to run, for example 1.2.
    [string]
    $Item,

    # Gns3Images: the compute whose images to list.
    [string]
    $ComputeId = 'local',

    # Gns3Range: run only the scenario with this name (its scenario.json "name").
    [string]
    $Scenario,

    # Gns3Range: use a go build of this checkout (dev_overrides) instead of the Registry release.
    [switch]
    $Dev,

    # Gns3Range: screenshot each applied scenario in the GNS3 GUI. A stub for now.
    [switch]
    $Capture,

    # Gns3Range -Scenario <name>: apply and check, skip destroy, start the nodes, open the project
    # in gns3.exe and print what to do next. The server stays running.
    [switch]
    $Inspect,

    # Gns3Range -Scenario <name>: destroy a scenario left up by -Inspect and clear its state.
    [switch]
    $Destroy
)

# InvokeBuild must already be installed (Install-Module InvokeBuild -Scope CurrentUser); this
# script never installs anything when it is loaded. The GNUmakefile stays the cross-platform
# path; these tasks are the same commands for Windows, where make is not installed, plus the
# GNS3 server tasks.

######################################################################################################
# InvokeBuild - ArgumentCompleters
######################################################################################################

Register-ArgumentCompleter -CommandName Invoke-Build.ps1 -ParameterName Task -ScriptBlock {
    param($commandName, $parameterName, $wordToComplete, $commandAst, $boundParameters)

    (Invoke-Build -Task ?? -File ($boundParameters['File'])).get_Keys() -like "$wordToComplete*" | .{process{
        New-Object System.Management.Automation.CompletionResult $_, $_, 'ParameterValue', $_
    }}
}

Register-ArgumentCompleter -CommandName Invoke-Build.ps1 -ParameterName File -ScriptBlock {
    param($commandName, $parameterName, $wordToComplete, $commandAst, $boundParameters)

    Get-ChildItem -Directory -Name "$wordToComplete*" | .{process{
        New-Object System.Management.Automation.CompletionResult $_, $_, 'ProviderContainer', $_
    }}

    if (!($boundParameters['Task'] -eq '**')) {
        Get-ChildItem -File -Name "$wordToComplete*.ps1" | .{process{
            New-Object System.Management.Automation.CompletionResult $_, $_, 'Command', $_
        }}
    }
}

######################################################################################################
# Helpers
######################################################################################################

# Everything the GNS3 tasks download or write lives here, never in Jerry's GNS3 settings.
# GetTempPath() is $env:TEMP on Windows and also works on the Linux CI runner, where $env:TEMP
# is unset. The GNS3 install folder exists only on Windows: Join-Path rejects a C:\ path on
# Linux while the script loads (Test-Docs), so it stays $null there and Get-Gns3Install throws.
$Gns3Work = Join-Path ([IO.Path]::GetTempPath()) 'gns3'
$Gns3PidFile = Join-Path $Gns3Work 'server.pid'
$Gns3DefaultFolder = if ($IsWindows) { Join-Path ($env:ProgramFiles ?? 'C:\Program Files') 'GNS3' } else { $null }

# The all-in-one installer's optional components, from its components page (GNS3 2.2.x).
# The provider's tests need only the GNS3 server and VPCS (plus uBridge, which the server uses
# for every VPCS link).
$Gns3Bundled = @(
    'GNS3 (GUI and gns3server)', 'Npcap', 'Wireshark', 'Dynamips', 'QEMU', 'VPCS', 'uBridge',
    'Solar-PuTTY', 'KiTTY', 'TightVNC Viewer', 'Virt-viewer', 'Intel HAXM', 'cpulimit', 'Tools'
)

function Invoke-Native {
    param(
        [Parameter(Mandatory)]
        [string]
        $FilePath,

        [string[]]
        $Arguments = @()
    )
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Get-Gns3Install {
    # Gns3Status, Gns3Install, Gns3Start and any task that starts a server come through here.
    if (-not $IsWindows) {
        throw 'GNS3 server tasks run on Windows only.'
    }
    # Registry uninstall keys first (they carry the version), then the default folder.
    $keys = @(
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    $entry = Get-ItemProperty -Path $keys -ErrorAction SilentlyContinue |
        Where-Object { $_.PSObject.Properties['DisplayName'] -and $_.DisplayName -match '^GNS3( |$)' } |
        Select-Object -First 1

    $folder = $null
    if ($entry -and $entry.PSObject.Properties['InstallLocation'] -and $entry.InstallLocation) {
        $folder = $entry.InstallLocation.Trim('"').TrimEnd('\')
    }
    elseif ($entry -and $entry.PSObject.Properties['UninstallString'] -and $entry.UninstallString) {
        $folder = Split-Path -Path $entry.UninstallString.Trim('"') -Parent
    }
    if (-not $folder -and (Test-Path -LiteralPath (Join-Path $Gns3DefaultFolder 'gns3server.exe'))) {
        $folder = $Gns3DefaultFolder
    }

    $server = if ($folder) { Join-Path $folder 'gns3server.exe' } else { $null }
    $version = $null
    if ($entry -and $entry.PSObject.Properties['DisplayVersion'] -and $entry.DisplayVersion) {
        $version = $entry.DisplayVersion
    }
    elseif ($server -and (Test-Path -LiteralPath $server)) {
        $version = (Get-Item -LiteralPath $server).VersionInfo.ProductVersion
    }

    [pscustomobject]@{
        Installed = [bool]($server -and (Test-Path -LiteralPath $server))
        Version   = $version
        Folder    = $folder
        Server    = $server
        Source    = if ($entry) { "registry: $($entry.PSPath -replace '^.*::', '')" } elseif ($folder) { 'default folder' } else { $null }
    }
}

function Invoke-Gns3Version {
    param([string] $TargetHost = $Gns3Host, [int] $Port = $Gns3Port)
    try {
        Invoke-RestMethod -Uri "http://${TargetHost}:$Port/v2/version" -TimeoutSec 3 -ErrorAction Stop
    }
    catch {
        $null
    }
}

function Get-Gns3StartedProcess {
    # The server Gns3Start started, if it is still running: the PID file names the process
    # and its start time, so a reused PID never matches.
    if (-not (Test-Path -LiteralPath $Gns3PidFile)) {
        return $null
    }
    $record = Get-Content -LiteralPath $Gns3PidFile -Raw | ConvertFrom-Json
    $process = Get-Process -Id $record.pid -ErrorAction SilentlyContinue
    if ($process -and $process.StartTime.ToUniversalTime().Ticks -eq [long]$record.startTicks) {
        return $process
    }
    $null
}

function Get-Gns3Status {
    $install = Get-Gns3Install
    $processes = @(Get-Process -Name gns3server -ErrorAction SilentlyContinue)
    $ports = @(foreach ($p in $processes) {
        Get-NetTCPConnection -OwningProcess $p.Id -State Listen -ErrorAction SilentlyContinue |
            ForEach-Object { "$($_.LocalAddress):$($_.LocalPort)" }
    }) | Sort-Object -Unique
    $answer = Invoke-Gns3Version
    $ours = Get-Gns3StartedProcess

    [pscustomobject]@{
        Installed        = $install.Installed
        InstalledVersion = $install.Version
        InstallFolder    = $install.Folder
        FoundBy          = $install.Source
        ServerRunning    = $processes.Count -gt 0
        ServerPids       = ($processes.Id -join ', ')
        StartedByBuild   = if ($ours) { $ours.Id } else { $null }
        ListeningOn      = ($ports -join ', ')
        Endpoint         = "http://${Gns3Host}:$Gns3Port/v2/version"
        Answers          = [bool]$answer
        ApiVersion       = if ($answer) { $answer.version } else { $null }
        ApiLocal         = if ($answer) { $answer.local } else { $null }
    }
}

function Get-Gns3Release {
    param([string] $Wanted)
    $api = if ($Wanted -eq 'latest') {
        'https://api.github.com/repos/GNS3/gns3-gui/releases/latest'
    }
    else {
        "https://api.github.com/repos/GNS3/gns3-gui/releases/tags/v$Wanted"
    }
    $headers = @{ 'User-Agent' = 'terraform-provider-gns3-build'; Accept = 'application/vnd.github+json' }
    if ($env:GITHUB_TOKEN) { $headers.Authorization = "Bearer $env:GITHUB_TOKEN" }
    $release = Invoke-RestMethod -Uri $api -Headers $headers -ErrorAction Stop
    $tag = $release.tag_name.TrimStart('v')
    $exe = $release.assets | Where-Object name -eq "GNS3-$tag-all-in-one.exe"
    $sha = $release.assets | Where-Object name -eq "GNS3-$tag-all-in-one.exe.sha256"
    if (-not $exe -or -not $sha) {
        throw "Release $($release.tag_name) ($api) has no GNS3-$tag-all-in-one.exe with a .sha256 beside it. Assets: $(@($release.assets.name) -join ', ')"
    }
    [pscustomobject]@{
        Version   = $tag
        Page      = $release.html_url
        Published = $release.published_at
        ExeName   = $exe.name
        ExeUrl    = $exe.browser_download_url
        ExeSize   = $exe.size
        ShaUrl    = $sha.browser_download_url
    }
}

function Start-Gns3Server {
    # Returns the started process, or throws. Refuses when something already answers.
    if ($answer = Invoke-Gns3Version) {
        throw "A GNS3 server already answers on http://${Gns3Host}:$Gns3Port (version $($answer.version)). Gns3Start does not start a second one; Gns3Stop never stops one it did not start."
    }
    $install = Get-Gns3Install
    if (-not $install.Installed) {
        throw "GNS3 is not installed (no uninstall key, nothing at $Gns3DefaultFolder). Run: Invoke-Build Gns3Install"
    }

    # A config of its own, so Jerry's %APPDATA%\GNS3 settings and projects are untouched.
    $configDir = Join-Path $Gns3Work 'config'
    $dataDir = Join-Path $Gns3Work 'data'
    foreach ($d in $configDir, $dataDir, (Join-Path $dataDir 'projects'), (Join-Path $dataDir 'images')) {
        $null = New-Item -ItemType Directory -Force -Path $d -ErrorAction Stop
    }
    $ini = Join-Path $configDir 'gns3_server.ini'
    # 2.2.61 ships both in subfolders; an older install can leave a stale ubridge.exe in the root.
    $vpcs = @(Join-Path $install.Folder 'vpcs' 'vpcs.exe'; Join-Path $install.Folder 'vpcs.exe') | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    $ubridge = @(Join-Path $install.Folder 'ubridge' 'ubridge.exe'; Join-Path $install.Folder 'ubridge.exe') | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    # On Windows the built-in Ethernet hub and switch run on Dynamips ("Could not find Dynamips" without it).
    $dynamips = Join-Path $install.Folder 'dynamips' 'dynamips.exe'
    @(
        '[Server]'
        "host = $Gns3Host"
        "port = $Gns3Port"
        'auth = False'
        "projects_path = $(Join-Path $dataDir 'projects')"
        "images_path = $(Join-Path $dataDir 'images')"
        "configs_path = $(Join-Path $dataDir 'configs')"
        "appliances_path = $(Join-Path $dataDir 'appliances')"
        "symbols_path = $(Join-Path $dataDir 'symbols')"
        "report_errors = False"
        $(if ($ubridge) { "ubridge_path = $ubridge" })
        ''
        '[VPCS]'
        $(if ($vpcs) { "vpcs_path = $vpcs" })
        ''
        '[Dynamips]'
        $(if (Test-Path -LiteralPath $dynamips) { "dynamips_path = $dynamips" })
    ) | Set-Content -LiteralPath $ini -Encoding ascii -ErrorAction Stop

    $log = Join-Path $Gns3Work 'server.log'
    $arguments = @('--config', "`"$ini`"", '--host', $Gns3Host, '--port', $Gns3Port, '--local', '--log', "`"$log`"")
    Write-Host "Starting $($install.Server) $($arguments -join ' ')"
    $process = Start-Process -FilePath $install.Server -ArgumentList $arguments -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput (Join-Path $Gns3Work 'server.stdout.log') -RedirectStandardError (Join-Path $Gns3Work 'server.stderr.log') -ErrorAction Stop
    [ordered]@{ pid = $process.Id; startTicks = $process.StartTime.ToUniversalTime().Ticks; port = $Gns3Port } |
        ConvertTo-Json | Set-Content -LiteralPath $Gns3PidFile -Encoding ascii -ErrorAction Stop

    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            Remove-Item -LiteralPath $Gns3PidFile -Force -ErrorAction SilentlyContinue
            $tail = if (Test-Path -LiteralPath $log) { (Get-Content -LiteralPath $log -Tail 30) -join "`n" } else { '(no log)' }
            throw "gns3server exited with code $($process.ExitCode) before answering. Last lines of ${log}:`n$tail"
        }
        if ($answer = Invoke-Gns3Version) {
            Write-Host "gns3server $($answer.version) answers on http://${Gns3Host}:$Gns3Port (PID $($process.Id), log $log)" -ForegroundColor Green
            return $process
        }
        Start-Sleep -Milliseconds 500
    }
    Stop-Gns3Server
    throw "gns3server (PID $($process.Id)) did not answer GET /v2/version on port $Gns3Port within 60 s; stopped it. Log: $log"
}

function Stop-Gns3Server {
    $process = Get-Gns3StartedProcess
    if (-not $process) {
        if (Test-Path -LiteralPath $Gns3PidFile) {
            Remove-Item -LiteralPath $Gns3PidFile -Force -ErrorAction Stop
            Write-Host "The server recorded in $Gns3PidFile is no longer running; removed the PID file."
        }
        else {
            Write-Host "No server started by Gns3Start ($Gns3PidFile is absent). Nothing stopped; a server started from the GUI is never touched."
        }
        return
    }
    # Children first (vpcs.exe, ubridge.exe started for project nodes), then the server.
    $children = @(Get-CimInstance Win32_Process -Filter "ParentProcessId = $($process.Id)" -ErrorAction SilentlyContinue)
    foreach ($child in $children) {
        Stop-Process -Id $child.ProcessId -Force -ErrorAction SilentlyContinue
    }
    Stop-Process -Id $process.Id -Force -ErrorAction Stop
    $process.WaitForExit(15000) | Out-Null
    Remove-Item -LiteralPath $Gns3PidFile -Force -ErrorAction Stop
    Write-Host "Stopped gns3server PID $($process.Id) (and $($children.Count) child process(es))." -ForegroundColor Green
}

function New-ScenarioDummyImage {
    # test/images/scenario-dummy.img for scenarios that require "image": 1 MiB, the same bytes on
    # every run, because GNS3 2.2 cannot delete images and gns3_image refuses to upload over a
    # different file of the same name. Not ELF, so the qemu list shows it.
    $dir = Join-Path $PSScriptRoot 'test' 'images'
    $null = New-Item -ItemType Directory -Force -Path $dir -ErrorAction Stop
    $file = Join-Path $dir 'scenario-dummy.img'
    $bytes = [byte[]]::new(1MB)
    for ($i = 0; $i -lt $bytes.Length; $i++) { $bytes[$i] = [byte]($i % 251) }
    $header = [Text.Encoding]::ASCII.GetBytes("terraform-provider-gns3 scenario dummy image; not bootable`n")
    [Array]::Copy($header, $bytes, $header.Length)
    [IO.File]::WriteAllBytes($file, $bytes)
    $file
}

function ConvertFrom-TerraformOutput {
    # terraform output -json gives { name: { sensitive, type, value } }; checks.ps1 gets name -> value.
    param([string] $Json)
    $outputs = @{}
    $raw = $Json | ConvertFrom-Json -AsHashtable
    if ($raw) {
        foreach ($key in $raw.Keys) { $outputs[$key] = $raw[$key].value }
    }
    $outputs
}

function Get-ScenarioState {
    # The resource addresses in a scenario's state, or none when it has no state file.
    param([string] $Dir)
    if (-not (Test-Path -LiteralPath (Join-Path $Dir 'terraform.tfstate'))) {
        return @()
    }
    $left = @(terraform "-chdir=$Dir" state list 2>&1 | ForEach-Object { "$_" } | Where-Object { $_ })
    if ($LASTEXITCODE -ne 0) { throw "terraform state list failed with exit code ${LASTEXITCODE}: $($left -join '; ')" }
    $left
}

function Get-ScenarioTeardownCommand {
    # The command that destroys a scenario left up by -Inspect, with the switches that differ from
    # the defaults, so the same provider build and server are used.
    param([string] $Name, [switch] $Dev)
    $command = "Invoke-Build Gns3Range -Scenario $Name -Destroy"
    if ($Dev) { $command += ' -Dev' }
    if ($Gns3Host -ne '127.0.0.1') { $command += " -Gns3Host $Gns3Host" }
    if ($Gns3Port -ne 3080) { $command += " -Gns3Port $Gns3Port" }
    $command
}

function Start-ScenarioNodes {
    # Starts every node of the project and returns the nodes as the server reports them afterwards.
    # POST /v2/projects/{project_id}/nodes/start: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodesstart.html
    # GET /v2/projects/{project_id}/nodes: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/node/projectsprojectidnodes.html
    param([string] $Server, [string] $ProjectId)
    $null = Invoke-RestMethod -Method Post -Uri "$Server/v2/projects/$ProjectId/nodes/start" -ContentType 'application/json' -Body '{}' -TimeoutSec 300 -ErrorAction Stop
    $nodes = @(Invoke-RestMethod -Uri "$Server/v2/projects/$ProjectId/nodes" -TimeoutSec 30 -ErrorAction Stop | ForEach-Object { $_ })
    $stopped = @($nodes | Where-Object status -ne 'started')
    if ($stopped) {
        throw "POST /v2/projects/$ProjectId/nodes/start returned, but these nodes are not started: $(@($stopped | ForEach-Object { "$($_.name) ($($_.status))" }) -join ', ')"
    }
    $nodes
}

function Open-ScenarioProject {
    # Opens the project's .gns3 file in gns3.exe when GNS3 is installed here and the server is
    # local; otherwise points at the web UI. Returns the line for the "what to do now" block.
    # GET /v2/projects/{project_id}: https://gns3-server.readthedocs.io/en/stable/api/v2/controller/project/projectsprojectid.html
    # The web UI: 2.2.61 redirects GET / to /static/web-ui/bundled.
    param([string] $Server, [string] $ProjectId)
    $project = Invoke-RestMethod -Uri "$Server/v2/projects/$ProjectId" -TimeoutSec 30 -ErrorAction Stop
    $web = "$Server/static/web-ui/bundled"
    $local = $Gns3Host -in '127.0.0.1', 'localhost', '::1'
    $install = if ($IsWindows) { Get-Gns3Install } else { $null }
    $gui = if ($install -and $install.Folder) { Join-Path $install.Folder 'gns3.exe' } else { $null }
    if ($local -and $gui -and (Test-Path -LiteralPath $gui)) {
        # A second gns3.exe opens a second window (seen on 2.2.61), so a running GUI is reused.
        $running = @(Get-Process -Name gns3 -ErrorAction SilentlyContinue)
        if ($running) {
            return "gns3.exe is already running (PID $(@($running.Id) -join ', ')): use File > Open project > '$($project.name)', or the web UI: $web"
        }
        $file = Join-Path $project.path $project.filename
        Start-Process -FilePath $gui -ArgumentList "`"$file`"" -ErrorAction Stop
        return "gns3.exe opened $file. If it shows no project, use File > Open project > '$($project.name)', or the web UI: $web"
    }
    $why = if ($local) { 'gns3.exe is not installed here' } else { "the server ($Gns3Host) is not this machine" }
    "$why. Open $web in a browser and open the project '$($project.name)'."
}

function Format-InspectGuide {
    # The numbered "what to do now" block printed after an -Inspect run.
    param([pscustomobject] $Meta, [object[]] $Nodes, [string] $GuiLine, [string] $Teardown)
    $n = 0
    if ($GuiLine) {
        "$((++$n)). Look at the project: $GuiLine"
    }
    if ($Nodes) {
        "$((++$n)). Open a console on each node (click the link, or in the GUI right-click the node > Console):"
        foreach ($node in $Nodes | Sort-Object name) {
            $consoleHost = if (-not $node.console_host -or $node.console_host -in '0.0.0.0', '::') { $Gns3Host } else { $node.console_host }
            switch ($node.console_type) {
                'telnet' { "     $($node.name): telnet://${consoleHost}:$($node.console)   (or: telnet $consoleHost $($node.console))" }
                'vnc' { "     $($node.name): vnc://${consoleHost}:$($node.console)" }
                default { "     $($node.name): console type '$($node.console_type)', nothing to open" }
            }
        }
    }
    $checks = if ($Meta.PSObject.Properties['inspect']) { @($Meta.inspect) } else { @() }
    if ($checks) {
        "$((++$n)). In each console, type the command and compare with the expected result:"
        foreach ($c in $checks) {
            $missing = if ($Nodes -and -not ($Nodes | Where-Object name -eq $c.node)) { "   (the project has no node named $($c.node))" } else { '' }
            "     $($c.node)> $($c.command)$missing"
            "        expect: $($c.expect)"
        }
    }
    "$((++$n)). Tear down when done: $Teardown"
}

function Get-ScenarioSkip {
    # The skip message for a published run when scenario.json sets minVersion above the version
    # main.tf pins for JerryBalmer1/gns3, or $null when the scenario runs. -Dev never skips: its
    # build has every unreleased feature. release-prep bumps the pins, and the skip goes away.
    param([string] $Dir, [pscustomobject] $Meta, [switch] $Dev)
    if ($Dev -or -not $Meta.PSObject.Properties['minVersion']) { return $null }
    $main = Get-Content -LiteralPath (Join-Path $Dir 'main.tf') -Raw
    $m = [regex]::Match($main, '(?s)source\s*=\s*"JerryBalmer1/gns3".*?version\s*=\s*"([^"]+)"')
    if (-not $m.Success) {
        throw "$($Meta.name): scenario.json sets minVersion, but main.tf pins no version for JerryBalmer1/gns3."
    }
    $pin = $m.Groups[1].Value
    if ([version]$pin -lt [version]$Meta.minVersion) {
        return "requires provider >= $($Meta.minVersion), pinned $pin"
    }
    $null
}

function Invoke-Scenario {
    # Runs one scenario folder: init, a stale-state check, apply, output, checks.ps1, then destroy,
    # which runs whenever init and the state check succeeded (also after a failed apply or failed
    # checks). With -Inspect there is no destroy: the nodes are started, the project is opened in
    # the GUI and the row carries the "what to do now" block in Guide. Returns a summary row.
    param([string] $Dir, [pscustomobject] $Meta, [string] $Server, [switch] $Dev, [switch] $Capture, [switch] $Inspect)
    $row = [pscustomobject]@{ Scenario = $Meta.name; Action = $(if ($Inspect) { 'inspect' } else { 'run' }); Result = 'FAIL'; FailedStep = $null; Seconds = 0; Message = ''; Guide = @() }
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $step = 'requires'
    $initDone = $false
    $outputs = @{}
    try {
        foreach ($need in @($Meta.requires)) {
            switch ($need) {
                'server' { }
                'image' { Write-Host "Wrote $(New-ScenarioDummyImage)" }
                default { throw "scenario.json requires '$need', which Gns3Range does not know (server, image)." }
            }
        }
        $step = 'init'
        Invoke-Native terraform "-chdir=$Dir", init, -input=false, -upgrade, -no-color | Out-Host
        # Resources left in state by an earlier run that never destroyed are not this run's to
        # delete: fail without apply and without destroy. -Destroy is the way to remove them.
        $step = 'state'
        $left = Get-ScenarioState -Dir $Dir
        if ($left) {
            throw "stale state from a previous run; destroy it first: $(Get-ScenarioTeardownCommand -Name $Meta.name -Dev:$Dev). In state: $($left -join ', ')"
        }
        $initDone = $true
        # The lock file names where the provider came from: it must be the Registry build. Under
        # -Dev the dev override replaces it, so the lock file says nothing about what runs.
        if (-not $Dev) {
            $lock = Get-Content -LiteralPath (Join-Path $Dir '.terraform.lock.hcl') -Raw
            if ($lock -notmatch 'provider "registry\.terraform\.io/jerrybalmer1/gns3" \{\s+version\s+=\s+"0\.2\.0"') {
                throw "init did not lock registry.terraform.io/jerrybalmer1/gns3 0.2.0. Lock file:`n$lock"
            }
        }
        $step = 'apply'
        Invoke-Native terraform "-chdir=$Dir", apply, -auto-approve, -input=false, -no-color | Out-Host
        $step = 'output'
        $json = terraform "-chdir=$Dir" output -json -no-color
        if ($LASTEXITCODE -ne 0) { throw "terraform output -json failed with exit code $LASTEXITCODE" }
        $outputs = ConvertFrom-TerraformOutput -Json ($json -join "`n")
        if ($Capture) {
            # Stub: opening the project in gns3.exe and saving a screenshot is not written yet.
            $step = 'capture'
            Write-Host "$($Meta.name): capture not implemented" -ForegroundColor Yellow
        }
        $step = 'checks'
        & (Join-Path $Dir 'checks.ps1') -Server $Server -Outputs $outputs
        if ($Inspect -and $outputs['project_id']) {
            $step = 'start'
            $nodes = @(Start-ScenarioNodes -Server $Server -ProjectId $outputs['project_id'])
            Write-Host "$($Meta.name): started $($nodes.Count) node(s)."
            $step = 'gui'
            $gui = Open-ScenarioProject -Server $Server -ProjectId $outputs['project_id']
            $row.Guide = @(Format-InspectGuide -Meta $Meta -Nodes $nodes -GuiLine $gui -Teardown (Get-ScenarioTeardownCommand -Name $Meta.name -Dev:$Dev))
        }
        elseif ($Inspect) {
            $row.Guide = @(Format-InspectGuide -Meta $Meta -GuiLine "the scenario has no project_id output, so there is no project to open; see test/scenarios/$($Meta.name)/README.md" -Teardown (Get-ScenarioTeardownCommand -Name $Meta.name -Dev:$Dev))
        }
        $row.Result = 'PASS'
    }
    catch {
        $row.FailedStep = $step
        $row.Message = "$_"
        Write-Host "$($Meta.name): $step failed: $_" -ForegroundColor Red
        if ($Inspect -and $initDone) {
            $row.Guide = @(Format-InspectGuide -Meta $Meta -Teardown (Get-ScenarioTeardownCommand -Name $Meta.name -Dev:$Dev))
        }
    }
    finally {
        if ($initDone -and -not $Inspect) {
            try {
                Invoke-Native terraform "-chdir=$Dir", destroy, -auto-approve, -input=false, -no-color | Out-Host
            }
            catch {
                $row.Result = 'FAIL'
                if (-not $row.FailedStep) { $row.FailedStep = 'destroy'; $row.Message = "$_" }
                else { $row.Message += " (destroy also failed: $_)" }
                Write-Host "$($Meta.name): destroy failed: $_" -ForegroundColor Red
            }
        }
        $row.Seconds = [int]$clock.Elapsed.TotalSeconds
    }
    $row
}

function Invoke-ScenarioDestroy {
    # Destroys a scenario left up by -Inspect (or by any run that never destroyed): init, destroy,
    # then a check that the state is empty, then removes terraform.tfstate and its backup. There is
    # no stale-state refusal and no lock-file check: this is the path that clears stale state.
    param([string] $Dir, [pscustomobject] $Meta)
    $row = [pscustomobject]@{ Scenario = $Meta.name; Action = 'destroy'; Result = 'FAIL'; FailedStep = $null; Seconds = 0; Message = ''; Guide = @() }
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $step = 'requires'
    try {
        # gns3_image reads its source file on refresh, so an image scenario needs it back.
        if (@($Meta.requires) -contains 'image') { Write-Host "Wrote $(New-ScenarioDummyImage)" }
        $step = 'init'
        Invoke-Native terraform "-chdir=$Dir", init, -input=false, -upgrade, -no-color | Out-Host
        $step = 'destroy'
        $before = @(Get-ScenarioState -Dir $Dir | Where-Object { $_ -notlike 'data.*' })
        Invoke-Native terraform "-chdir=$Dir", destroy, -auto-approve, -input=false, -no-color | Out-Host
        $step = 'state'
        $left = @(Get-ScenarioState -Dir $Dir)
        if ($left) {
            throw "destroy succeeded but the state still holds: $($left -join ', ')"
        }
        foreach ($file in 'terraform.tfstate', 'terraform.tfstate.backup') {
            $path = Join-Path $Dir $file
            if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Force -ErrorAction Stop }
        }
        $row.Message = if ($before) { "destroyed $($before.Count) resource(s); state cleared" } else { 'nothing in state; state cleared' }
        $row.Result = 'PASS'
    }
    catch {
        $row.FailedStep = $step
        $row.Message = "$_"
        Write-Host "$($Meta.name): $step failed: $_" -ForegroundColor Red
    }
    finally {
        $row.Seconds = [int]$clock.Elapsed.TotalSeconds
    }
    $row
}

######################################################################################################
# InvokeBuild - Tasks
######################################################################################################

# Whether GNS3 is installed, its version, whether gns3server runs and where, and what
# GET /v2/version answers. Never installs or starts anything.
task Gns3Status {
    Get-Gns3Status
}

# Installs GNS3 from the all-in-one installer on the GitHub release (gns3-gui), after checking
# its SHA-256 against the .sha256 file beside it. Skips when the requested version is already
# installed, unless -Force.
task Gns3Install {
    $install = Get-Gns3Install
    $release = Get-Gns3Release -Wanted $Version
    Write-Host "Requested $Version -> GNS3 $($release.Version) ($($release.Page), published $($release.Published))"
    if ($install.Installed -and -not $Force) {
        if ($install.Version -eq $release.Version) {
            Write-Host "GNS3 $($install.Version) is already installed in $($install.Folder). Nothing to do." -ForegroundColor Green
        }
        else {
            Write-Host "GNS3 $($install.Version) is installed in $($install.Folder); $($release.Version) was requested. Nothing changed: rerun with -Force to install $($release.Version) over it." -ForegroundColor Yellow
        }
        return
    }

    $null = New-Item -ItemType Directory -Force -Path $Gns3Work -ErrorAction Stop
    $exe = Join-Path $Gns3Work $release.ExeName
    $shaFile = "$exe.sha256"
    Write-Host "Downloading $($release.ShaUrl)"
    Invoke-WebRequest -Uri $release.ShaUrl -OutFile $shaFile -ErrorAction Stop
    $expected = ((Get-Content -LiteralPath $shaFile -Raw).Trim() -split '\s+')[0].ToLowerInvariant()
    if ($expected -notmatch '^[0-9a-f]{64}$') {
        throw "$shaFile does not start with a SHA-256 hash: $(Get-Content -LiteralPath $shaFile -Raw)"
    }
    $actual = if (Test-Path -LiteralPath $exe) { (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant() } else { $null }
    if ($actual -ne $expected) {
        Write-Host "Downloading $($release.ExeUrl) ($([math]::Round($release.ExeSize / 1MB)) MB)"
        $progress = $ProgressPreference
        $ProgressPreference = 'SilentlyContinue'
        try { Invoke-WebRequest -Uri $release.ExeUrl -OutFile $exe -ErrorAction Stop }
        finally { $ProgressPreference = $progress }
        $actual = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    else {
        Write-Host "$exe is already downloaded and matches its hash."
    }
    if ($actual -ne $expected) {
        Remove-Item -LiteralPath $exe -Force -ErrorAction SilentlyContinue
        throw "SHA-256 mismatch for $($release.ExeName): expected $expected, got $actual. Deleted the download; not installing."
    }
    Write-Host "SHA-256 OK: $actual" -ForegroundColor Green

    Write-Host "The all-in-one installer bundles: $($Gns3Bundled -join ', ')."
    Write-Host 'This provider needs only the GNS3 server and VPCS (and uBridge, which links VPCS nodes).'
    Write-Host 'The NSIS silent switch (/S) installs the installer''s default component set; it has no command-line switch to deselect components, so the default extras come along. Npcap''s free edition has no silent mode, so a silent run may skip it or show its own window.' -ForegroundColor Yellow

    $admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $admin) {
        Write-Host 'This session is not elevated and the installer needs administrator rights (it installs to Program Files): Windows will show a UAC prompt. Accept it to continue.' -ForegroundColor Yellow
    }
    try {
        Write-Host "Running $exe /S"
        $run = Start-Process -FilePath $exe -ArgumentList '/S' -Wait -PassThru -ErrorAction Stop
    }
    catch {
        # Win32 error 740: the installer's manifest asks for administrator rights.
        if ($_.Exception.Message -notmatch 'requires elevation') { throw }
        Write-Host 'The installer needs administrator rights. Starting it elevated: accept the UAC prompt to continue.' -ForegroundColor Yellow
        $run = Start-Process -FilePath $exe -ArgumentList '/S' -Verb RunAs -Wait -PassThru -ErrorAction Stop
    }
    if ($run.ExitCode -ne 0) {
        throw "$($release.ExeName) /S exited with code $($run.ExitCode)."
    }
    $after = Get-Gns3Install
    if (-not $after.Installed) {
        throw "The installer exited 0 but no gns3server.exe was found (registry or $Gns3DefaultFolder)."
    }
    Write-Host "Installed GNS3 $($after.Version) in $($after.Folder)." -ForegroundColor Green

    $doc = Join-Path $PSScriptRoot 'docs' 'gns3-install.md'
    @(
        '# GNS3 server install record'
        ''
        'Up: [README](../README.md)'
        ''
        'See also: [Provider docs](index.md), [API drift](api-drift.md), [Emulator images](images.md), [Looking at a scenario in the GUI](manual-steps.md)'
        ''
        'Written by `Invoke-Build Gns3Install` (`.build.ps1`); rerunning it with `-Force` rewrites this file.'
        'This page is not generated by tfplugindocs and is not part of the registry docs.'
        ''
        '| | |'
        '|---|---|'
        "| Installed version | $($after.Version) |"
        "| Install folder | ``$($after.Folder)`` |"
        "| Installer | $($release.ExeName) ($($release.ExeSize) bytes) |"
        "| Download URL | $($release.ExeUrl) |"
        "| Release page | $($release.Page) |"
        "| SHA-256 (verified against $($release.ExeName).sha256) | ``$actual`` |"
        "| Installed on | $(Get-Date -Format 'yyyy-MM-dd') |"
        "| Installer switches | ``/S`` (NSIS silent; default component set) |"
        ''
        "Bundled components: $($Gns3Bundled -join ', '). The provider and its tests need only the GNS3 server, VPCS and uBridge."
    ) | Set-Content -LiteralPath $doc -Encoding utf8NoBOM -ErrorAction Stop
    Write-Host "Recorded the install in $doc"
}

# Starts gns3server.exe in the background on -Gns3Port with its own config under
# $env:TEMP\gns3\config, waits up to 60 s for GET /v2/version, and records the PID.
task Gns3Start {
    $null = Start-Gns3Server
}

# Stops only the server Gns3Start started ($env:TEMP\gns3\server.pid), never one started
# from the GUI.
task Gns3Stop {
    Stop-Gns3Server
}

# Lists the images on -ComputeId (default local) for each emulator through the controller:
# GET /v2/computes/{compute_id}/{qemu,dynamips,iou}/images, then .../docker/images. Uses the
# provider's GNS3_HOST, GNS3_PORT, GNS3_USERNAME and GNS3_PASSWORD. Downloads and uploads nothing.
task Gns3Images {
    $headers = @{}
    if ($env:GNS3_USERNAME) {
        $pair = [Text.Encoding]::UTF8.GetBytes("$($env:GNS3_USERNAME):$($env:GNS3_PASSWORD)")
        $headers.Authorization = 'Basic ' + [Convert]::ToBase64String($pair)
    }
    $base = "http://${Gns3Host}:$Gns3Port/v2/computes/$([uri]::EscapeDataString($ComputeId))"
    if (-not (Invoke-Gns3Version)) {
        throw "No GNS3 server answers on http://${Gns3Host}:$Gns3Port/v2/version. Start one (Invoke-Build Gns3Start) and rerun."
    }
    # A compute without an emulator (iou and docker on Windows) answers 404 "GET /iou/images not found".
    $get = {
        param([string] $Emulator)
        try {
            $list = Invoke-RestMethod -Uri "$base/$Emulator/images" -Headers $headers -TimeoutSec 60 -ErrorAction Stop
            [pscustomobject]@{ Ok = $true; Items = @($list | ForEach-Object { $_ }) }
        }
        catch {
            $status = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { $null }
            $message = $_.Exception.Message
            if ($_.ErrorDetails.Message) {
                $message = try { ($_.ErrorDetails.Message | ConvertFrom-Json).message } catch { $_.ErrorDetails.Message }
            }
            Write-Host "${Emulator}: $(if ($status) { "HTTP $status, " })$message$(if ($status -eq 404) { ' (this compute has no ' + $Emulator + ' support, or the compute does not exist)' })" -ForegroundColor Yellow
            [pscustomobject]@{ Ok = $false; Items = @() }
        }
    }
    $rows = foreach ($emulator in 'qemu', 'dynamips', 'iou') {
        foreach ($image in (& $get $emulator).Items) {
            [pscustomobject]@{
                Emulator = $emulator
                Filename = $image.filename
                SizeMB   = [math]::Round($image.filesize / 1MB, 1)
                MD5      = $image.md5sum
                Path     = $image.path
            }
        }
    }
    Write-Host "Images on compute '$ComputeId' at http://${Gns3Host}:${Gns3Port}:"
    $rows | Format-Table -AutoSize | Out-String -Width 250 | Write-Host
    $docker = & $get 'docker'
    if ($docker.Ok) {
        Write-Host "Docker images known to the compute's daemon: $(if ($docker.Items.Count) { ($docker.Items.image | Sort-Object) -join ', ' } else { '(none)' })"
    }
}

task Build {
    Invoke-Native go build, -o, ., ./...
}

task Install {
    Invoke-Native go install, .
}

# Client fixture tests and provider tests against the in-memory fake (terraform on PATH).
task Test {
    Invoke-Native go test, ./..., -count=1, -timeout, 10m
}

task Lint {
    Invoke-Native go vet, ./...
    # staticcheck v0.7.0 cannot read export data from newer Go releases (1.27 on Jerry's machine),
    # so it runs under the go.mod toolchain. GOTOOLCHAIN is set for this command only.
    $savedToolchain = $env:GOTOOLCHAIN
    $env:GOTOOLCHAIN = 'go1.25.8'
    try {
        Invoke-Native go run, honnef.co/go/tools/cmd/staticcheck@v0.7.0, ./...
    }
    finally {
        $env:GOTOOLCHAIN = $savedToolchain
    }
    $unformatted = gofmt -l .
    if ($LASTEXITCODE -ne 0) { throw "gofmt -l failed with exit code $LASTEXITCODE" }
    if ($unformatted) {
        throw "gofmt needed:`n$($unformatted -join "`n")"
    }
    Write-Host 'go vet, staticcheck and gofmt are clean.' -ForegroundColor Green
}

# Regenerates docs/ (never edited by hand) and validates it.
task Docs {
    Invoke-Native go run, github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs, generate, --provider-name, gns3
    Invoke-Native go run, github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs, validate, --provider-name, gns3
}

# The acceptance tests against a live server. Starts one with -AutoStart when nothing answers,
# and stops afterwards only the server it started. The log is acc-run-<timestamp>.log.
task TestAcc {
    $status = Get-Gns3Status
    $status | Format-List | Out-String | Write-Host
    $started = $null
    if (-not $status.Answers) {
        if (-not $AutoStart) {
            throw "No GNS3 server answers on http://${Gns3Host}:$Gns3Port/v2/version. Start one and rerun:`n  Invoke-Build Gns3Start`n  Invoke-Build TestAcc`nor let TestAcc start and stop it: Invoke-Build TestAcc -AutoStart"
        }
        $started = Start-Gns3Server
    }

    $log = Join-Path $PSScriptRoot "acc-run-$(Get-Date -Format 'yyyyMMdd-HHmmss').log"
    $env:TF_ACC = '1'
    $env:GNS3_HOST = $Gns3Host
    $env:GNS3_PORT = [string]$Gns3Port
    try {
        Write-Host "go test ./internal/provider/ -run TestAcc -count=1 -v -timeout 60m  (log: $log)"
        go test ./internal/provider/ -run TestAcc -count=1 -v -timeout 60m 2>&1 | ForEach-Object { "$_" } | Tee-Object -FilePath $log | Write-Host
        $code = $LASTEXITCODE
    }
    finally {
        Remove-Item Env:TF_ACC -ErrorAction SilentlyContinue
        if ($started) {
            Stop-Gns3Server
        }
    }
    $summary = Select-String -LiteralPath $log -Pattern '^(--- (PASS|FAIL|SKIP)|ok |FAIL)' | ForEach-Object Line
    Write-Host ($summary -join "`n")
    if ($code -ne 0) {
        throw "Acceptance tests failed (exit code $code). Log: $log"
    }
    Write-Host "Acceptance tests passed. Log: $log" -ForegroundColor Green
}

function Invoke-Gns3Range {
    # The body of Gns3Range, a function so that Release can run it with -Dev.
    param([switch] $Dev, [switch] $Capture, [switch] $Inspect, [switch] $Destroy)
    if ($Inspect -and $Destroy) {
        throw 'Gns3Range takes -Inspect or -Destroy, not both.'
    }
    if (($Inspect -or $Destroy) -and -not $Scenario) {
        throw "Gns3Range -$(if ($Inspect) { 'Inspect' } else { 'Destroy' }) needs -Scenario <name>."
    }
    $found = @(Get-ChildItem -Path (Join-Path $PSScriptRoot 'test' 'scenarios' '*' 'scenario.json') -File | ForEach-Object {
        [pscustomobject]@{ Dir = $_.DirectoryName; Meta = Get-Content -LiteralPath $_.FullName -Raw | ConvertFrom-Json }
    })
    if (-not $found) {
        throw 'No scenarios: test/scenarios/*/scenario.json matched nothing.'
    }
    $run = if ($Scenario) { @($found | Where-Object { $_.Meta.name -eq $Scenario }) } else { $found }
    if (-not $run) {
        throw "No scenario named '$Scenario'. Scenarios: $(@($found.Meta.name) -join ', ')"
    }

    # -Inspect records here whether it started the server, so that -Destroy stops only a server an
    # -Inspect run started, and only once no other inspected scenario is left up.
    $inspectDir = Join-Path $Gns3Work 'inspect'
    $marker = if ($Scenario) { Join-Path $inspectDir "$Scenario.json" } else { $null }
    $started = $null
    if (-not (Invoke-Gns3Version)) {
        $started = Start-Gns3Server
    }
    elseif ($Destroy -and (Test-Path -LiteralPath $marker)) {
        $record = Get-Content -LiteralPath $marker -Raw | ConvertFrom-Json
        $ours = Get-Gns3StartedProcess
        if ($record.startedServer -and $ours -and $ours.Id -eq $record.serverPid) {
            $started = $ours
        }
    }
    $null = New-Item -ItemType Directory -Force -Path $Gns3Work -ErrorAction Stop
    $cliConfig = Join-Path $Gns3Work 'scenario-terraform.rc'
    $mode = 'published provider from the Registry'
    if ($Dev) {
        # A build of this checkout in a folder of its own, named the way dev_overrides expects.
        $devDir = Join-Path $Gns3Work 'dev-provider'
        $null = New-Item -ItemType Directory -Force -Path $devDir -ErrorAction Stop
        $exe = Join-Path $devDir $(if ($IsWindows) { 'terraform-provider-gns3.exe' } else { 'terraform-provider-gns3' })
        Invoke-Native go build, -o, $exe, $PSScriptRoot
        $mode = "DEV BUILD $exe"
        @(
            'provider_installation {'
            '  dev_overrides {'
            "    `"JerryBalmer1/gns3`" = `"$($devDir -replace '\\', '/')`""
            '  }'
            '  direct {}'
            '}'
        ) | Set-Content -LiteralPath $cliConfig -Encoding ascii -ErrorAction Stop
    }
    else {
        "provider_installation {`n  direct {}`n}" | Set-Content -LiteralPath $cliConfig -Encoding ascii -ErrorAction Stop
    }
    Write-Host "Gns3Range: $mode" -ForegroundColor Cyan
    $saved = @{}
    foreach ($name in 'TF_CLI_CONFIG_FILE', 'TF_IN_AUTOMATION', 'GNS3_HOST', 'GNS3_PORT') {
        $saved[$name] = [Environment]::GetEnvironmentVariable($name)
    }
    $env:TF_CLI_CONFIG_FILE = $cliConfig
    $env:TF_IN_AUTOMATION = '1'
    $env:GNS3_HOST = $Gns3Host
    $env:GNS3_PORT = [string]$Gns3Port
    $server = "http://${Gns3Host}:$Gns3Port"
    $rows = @()
    try {
        $rows = foreach ($s in $run) {
            Write-Host "== $($s.Meta.name): $($s.Meta.description)" -ForegroundColor Cyan
            $skip = Get-ScenarioSkip -Dir $s.Dir -Meta $s.Meta -Dev:$Dev
            if ($skip) {
                Write-Host "$($s.Meta.name): SKIP: $skip" -ForegroundColor Yellow
                [pscustomobject]@{ Scenario = $s.Meta.name; Action = $(if ($Destroy) { 'destroy' } elseif ($Inspect) { 'inspect' } else { 'run' }); Result = 'SKIP'; FailedStep = $null; Seconds = 0; Message = $skip; Guide = @() }
            }
            elseif ($Destroy) {
                Invoke-ScenarioDestroy -Dir $s.Dir -Meta $s.Meta
            }
            else {
                Invoke-Scenario -Dir $s.Dir -Meta $s.Meta -Server $server -Dev:$Dev -Capture:$Capture -Inspect:$Inspect
            }
        }
    }
    finally {
        foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name]) }
        $keepServer = $false
        if ($Inspect -and @($rows | Where-Object Result -ne 'SKIP')) {
            # The scenario stays up, so the server stays running; the marker tells -Destroy whose it is.
            $null = New-Item -ItemType Directory -Force -Path $inspectDir -ErrorAction Stop
            [ordered]@{ scenario = $Scenario; dev = [bool]$Dev; startedServer = [bool]$started; serverPid = $(if ($started) { $started.Id } else { $null }); at = (Get-Date).ToString('o') } |
                ConvertTo-Json | Set-Content -LiteralPath $marker -Encoding ascii -ErrorAction Stop
            $keepServer = $true
            if ($started) { Write-Host "Left gns3server PID $($started.Id) running for the inspected scenario; -Destroy stops it." -ForegroundColor Yellow }
        }
        elseif ($Destroy) {
            if (@($rows | Where-Object Result -eq 'PASS') -and (Test-Path -LiteralPath $marker)) {
                Remove-Item -LiteralPath $marker -Force -ErrorAction Stop
            }
            $others = @(Get-ChildItem -Path $inspectDir -Filter '*.json' -File -ErrorAction SilentlyContinue)
            if ($others) {
                $keepServer = $true
                if ($started) { Write-Host "Left gns3server PID $($started.Id) running: still inspected: $(@($others.BaseName) -join ', ')." -ForegroundColor Yellow }
            }
        }
        if ($started -and -not $keepServer) {
            Stop-Gns3Server
        }
    }
    Write-Host "== Gns3Range summary ($mode)" -ForegroundColor Cyan
    $rows | Format-Table -Property Scenario, Action, Result, FailedStep, Seconds, Message -AutoSize -Wrap | Out-String -Width 250 | Write-Host
    $guide = @($rows | ForEach-Object { $_.Guide })
    if ($guide) {
        Write-Host "== What to do now ($($Scenario))" -ForegroundColor Cyan
        Write-Host ($guide -join "`n")
        Write-Host ''
    }
    $failed = @($rows | Where-Object Result -notin 'PASS', 'SKIP')
    if ($failed) {
        throw "$($failed.Count) of $(@($rows).Count) scenario(s) failed ($mode): $(@($failed.Scenario) -join ', ')"
    }
    $skipped = @($rows | Where-Object Result -eq 'SKIP')
    $passed = @($rows).Count - $skipped.Count
    $skipNote = if ($skipped) { ", $($skipped.Count) skipped ($(@($skipped.Scenario) -join ', '))" } else { '' }
    Write-Host "$passed scenario(s) passed$skipNote against $server ($mode)." -ForegroundColor Green
}

# Runs each scenario under test/scenarios (or only -Scenario <name>) against a live server: init,
# a stale-state check, apply, output -json, checks.ps1, destroy. Starts a server like Gns3Start
# when nothing answers and stops afterwards only the server it started. Terraform runs with a CLI
# config of its own, so Jerry's terraform.rc never applies. By default that config is direct {}
# only and the scenarios run the provider published on the Registry; with -Dev it adds a
# dev_overrides entry for a go build of this checkout. -Capture is a stub. A published run skips a
# scenario whose scenario.json minVersion is above the version its main.tf pins; -Dev never skips.
# -Scenario <name> -Inspect skips destroy, starts the nodes, opens the project in gns3.exe (or
# prints the web UI address), leaves the server running and prints a numbered "what to do now"
# block: console links, the scenario.json "inspect" commands and the teardown command.
# -Scenario <name> -Destroy destroys it, clears its state and stops a server -Inspect started.
task Gns3Range {
    Invoke-Gns3Range -Dev:$Dev -Capture:$Capture -Inspect:$Inspect -Destroy:$Destroy
}

# generic: copy to any Invoke-Build repo
# Walks every .md in the repo (not .git, .terraform or node_modules) and fails on any relative
# link or image path that does not resolve, and on any .md under docs/, test/scenarios/ or lab-template/ without
# an "Up:" line. The pages tfplugindocs generates for the Registry (docs/index.md, docs/resources/,
# docs/data-sources/) need no Up: line; their links are still checked. Fenced code blocks and
# inline code are skipped. A leading / resolves from the repo root, as on GitHub.
task Test-Docs {
    $skip = '[\\/](\.git|\.terraform|node_modules)([\\/]|$)'
    $files = @(Get-ChildItem -Path $PSScriptRoot -Recurse -File -Filter '*.md' -Force | Where-Object { $_.FullName -notmatch $skip })
    $upRequired = '^(docs|test[\\/]scenarios|lab-template)[\\/]'
    $upExempt = '^docs[\\/]((resources|data-sources)[\\/]|index\.md$)'
    $problems = foreach ($file in $files) {
        $rel = [IO.Path]::GetRelativePath($PSScriptRoot, $file.FullName)
        $text = Get-Content -LiteralPath $file.FullName -Raw
        if (-not $text) { $text = '' }
        $prose = [regex]::Replace($text, '(?ms)^[ \t]*(```|~~~).*?^[ \t]*\1[^\r\n]*$', '')
        $prose = [regex]::Replace($prose, '``[^`]*``|`[^`\r\n]*`', '')
        if ($rel -match $upRequired -and $rel -notmatch $upExempt -and $prose -notmatch '(?m)^Up:\s') {
            "${rel}: no 'Up:' line"
        }
        # Inline links and images [text](target "title"), plus reference definitions [x]: target.
        $targets = @([regex]::Matches($prose, '!?\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)') | ForEach-Object { $_.Groups[1].Value }) +
            @([regex]::Matches($prose, '(?m)^\s{0,3}\[[^\]]+\]:\s*<?(\S+?)>?(?:\s+.*)?$') | ForEach-Object { $_.Groups[1].Value })
        foreach ($target in $targets) {
            if ($target -match '^[a-zA-Z][a-zA-Z0-9+.-]*:' -or $target.StartsWith('#')) { continue }
            $path = [uri]::UnescapeDataString(($target -split '[#?]', 2)[0])
            if (-not $path) { continue }
            $base = if ($path.StartsWith('/')) { $PSScriptRoot } else { $file.DirectoryName }
            if (-not (Test-Path -LiteralPath (Join-Path $base $path.TrimStart('/')))) {
                "${rel}: link target '$target' does not resolve"
            }
        }
    }
    if ($problems) {
        throw "Test-Docs found $(@($problems).Count) problem(s) in $($files.Count) .md file(s):`n$($problems -join "`n")"
    }
    Write-Host "Test-Docs: $($files.Count) .md file(s); every relative link resolves and every page under docs/, test/scenarios/ and lab-template/ that needs an Up: line has one." -ForegroundColor Green
}

# The full chain before a release tag (the pre-push hook runs it for any tag): lint, unit tests,
# the scenarios against a live server with a dev build of this checkout (the version being tagged
# is not on the Registry yet), the docs link check, and no uncommitted screenshots.
task Release Lint, Test, { Invoke-Gns3Range -Dev }, Test-Docs, {
    $dirty = git -C $PSScriptRoot status --porcelain -- docs/screenshots
    if ($LASTEXITCODE -ne 0) { throw "git status --porcelain docs/screenshots failed with exit code $LASTEXITCODE" }
    if ($dirty) {
        throw "docs/screenshots has uncommitted changes; commit or discard them before tagging:`n$($dirty -join "`n")"
    }
    Write-Host 'Release checks passed.' -ForegroundColor Green
}

# Points git at the tracked hooks in .githooks (pre-push runs the build before a push) and tells
# the generic pre-push hook which task runs the scenarios. Changes only this clone's .git/config.
task Setup {
    Invoke-Native git '-C', $PSScriptRoot, config, core.hooksPath, '.githooks'
    Invoke-Native git '-C', $PSScriptRoot, config, hooks.rangeTask, Gns3Range
    Write-Host 'core.hooksPath = .githooks, hooks.rangeTask = Gns3Range. git push now runs .githooks/pre-push.' -ForegroundColor Green
}

# Runs one manual-check-list.md item's PowerShell block in a fresh pwsh and prints its output
# beside the item's Expect line.
task Checklist {
    if (-not $Item) {
        throw 'Checklist needs -Item <n.m>, for example: Invoke-Build Checklist -Item 1.2'
    }
    $text = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'manual-check-list.md') -Raw
    $pattern = '(?ms)^### ' + [regex]::Escape($Item) + ' (?<title>[^\r\n]+)\r?\n(?<body>.*?)(?=^### |^## |\z)'
    $match = [regex]::Match($text, $pattern)
    if (-not $match.Success) {
        throw "manual-check-list.md has no item ### $Item."
    }
    $body = $match.Groups['body'].Value
    $block = [regex]::Match($body, '(?ms)^```powershell\r?\n(?<code>.*?)^```')
    if (-not $block.Success) {
        throw "Item $Item has no powershell block."
    }
    $expect = [regex]::Match($body, '(?m)^Expect: .*$').Value

    $script = Join-Path ([System.IO.Path]::GetTempPath()) "gns3-checklist-$Item-$([guid]::NewGuid().ToString('n')).ps1"
    Set-Content -LiteralPath $script -Value $block.Groups['code'].Value -Encoding utf8NoBOM -ErrorAction Stop
    Write-Host "== $Item $($match.Groups['title'].Value) (fresh pwsh)" -ForegroundColor Cyan
    try {
        $output = & pwsh -NoProfile -NonInteractive -File $script 2>&1 | ForEach-Object { "$_" }
        $code = $LASTEXITCODE
    }
    finally {
        Remove-Item -LiteralPath $script -Force -ErrorAction SilentlyContinue
    }
    Write-Host '-- Output' -ForegroundColor Cyan
    Write-Host ($output -join "`n")
    Write-Host '-- Expect' -ForegroundColor Cyan
    Write-Host $expect
    if ($code -ne 0) {
        throw "Item $Item exited with code $code."
    }
}

task . Build, Lint, Test
