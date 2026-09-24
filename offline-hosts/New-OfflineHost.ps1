<#
=============================================================================
 New-OfflineHost.ps1 -- scaffold one offline self-check host.

   Creates
     <HostsRoot>\Directory.Build.props        (copied from templates\, only if absent)
     <HostsRoot>\<Name>\<Name>.csproj         (from templates\Host.csproj.template)
     <HostsRoot>\<Name>\Program.cs            (from templates\Program.cs.template)
   and prints the paths it resolved (project / engine / unity editor), so a
   wrong tree is visible BEFORE the host is ever run.

   Why a generator instead of "copy the neighbouring host": the source project
   carried 13 hand-copied csproj files, each with the Unity version and the
   relative depth written by hand (54 HintPath lines) -- a machine change broke
   all 13 at once, and a new host silently copied someone else's 4-level depth.

   Usage:
     powershell -NoProfile -ExecutionPolicy Bypass -File New-OfflineHost.ps1 `
         -Name CoreCheck -HostsRoot <dir> [-ProjectRoot <dir>] [-EngineRoot <dir>]
         [-TargetFramework net10.0] [-UnityEditorRoot <dir>] [-Force]
   Exit codes: 0 = created, 2 = bad argument / template missing.
=============================================================================
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Name,
    [string]$HostsRoot = '',
    [string]$ProjectRoot = '',
    [string]$EngineRoot = '',
    [string]$TargetFramework = 'net10.0',
    [string]$UnityEditorRoot = '',
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'clover-paths.ps1')

function Abort([string]$m) { Write-Output ('FAIL New-OfflineHost: ' + $m); exit 2 }

if ($Name -notmatch '^[A-Za-z][A-Za-z0-9_.]*$') {
    Abort ('invalid -Name "' + $Name + '": letters / digits / dot / underscore, starting with a letter (e.g. CoreCheck)')
}

if (-not $HostsRoot) { $HostsRoot = Join-Path $PSScriptRoot 'hosts' }
if (-not (Test-Path -LiteralPath $HostsRoot)) { New-Item -ItemType Directory -Force -Path $HostsRoot | Out-Null }
$HostsRoot = (Resolve-Path -LiteralPath $HostsRoot).Path

if (-not $ProjectRoot) { $ProjectRoot = Find-CloverProjectRoot -Start $HostsRoot }
$unityDir = ''
if ($ProjectRoot) { $unityDir = Find-CloverUnityDir -Start $ProjectRoot }
if (-not $EngineRoot) { $EngineRoot = Find-CloverEngineRuntime -Start $HostsRoot }
$editor = Resolve-UnityEditorRoot -Root $UnityEditorRoot

if (-not $ProjectRoot -or -not $unityDir) {
    Abort ('no Unity project found by upward search from ' + $HostsRoot + ' -- pass -ProjectRoot <dir>')
}

$tplDir = Join-Path $PSScriptRoot 'templates'
$propsTpl = Join-Path $tplDir 'Directory.Build.props'
$csprojTpl = Join-Path $tplDir 'Host.csproj.template'
$progTpl = Join-Path $tplDir 'Program.cs.template'
foreach ($t in @($propsTpl, $csprojTpl, $progTpl)) {
    if (-not (Test-Path -LiteralPath $t)) { Abort ('template missing -> ' + $t) }
}

$utf8Bom = New-Object System.Text.UTF8Encoding($true)

# ---------------------------------------------------------------- props
$propsDst = Join-Path $HostsRoot 'Directory.Build.props'
$propsState = 'kept (exists)'
if (-not (Test-Path -LiteralPath $propsDst)) {
    Copy-Item -LiteralPath $propsTpl -Destination $propsDst -Force
    $propsState = 'created'
}

# ---------------------------------------------------------------- host dir
$hostDir = Join-Path $HostsRoot $Name
if (-not (Test-Path -LiteralPath $hostDir)) { New-Item -ItemType Directory -Force -Path $hostDir | Out-Null }
$ns = $Name.Replace('.', '_')

$csprojDst = Join-Path $hostDir ($Name + '.csproj')
$csprojState = 'created'
if ((Test-Path -LiteralPath $csprojDst) -and -not $Force) {
    $csprojState = 'kept (exists; -Force overwrites)'
} else {
    $csproj = [System.IO.File]::ReadAllText($csprojTpl, [System.Text.Encoding]::UTF8)
    $csproj = $csproj.Replace('@@HOST_NAME@@', $Name).Replace('@@NS@@', $ns).Replace('@@TFM@@', $TargetFramework)
    [System.IO.File]::WriteAllText($csprojDst, $csproj, $utf8Bom)
}

$progDst = Join-Path $hostDir 'Program.cs'
$progState = 'created'
if ((Test-Path -LiteralPath $progDst) -and -not $Force) {
    $progState = 'kept (exists; -Force overwrites)'
} else {
    $prog = [System.IO.File]::ReadAllText($progTpl, [System.Text.Encoding]::UTF8)
    $prog = $prog.Replace('@@HOST_NAME@@', $Name).Replace('@@NS@@', $ns)
    [System.IO.File]::WriteAllText($progDst, $prog, $utf8Bom)
}

$runner = Join-Path $PSScriptRoot 'Invoke-OfflineHosts.ps1'

Write-Output '=== New-OfflineHost ==='
Write-Output ('name           = ' + $Name)
Write-Output ('hosts-root     = ' + $HostsRoot)
Write-Output ('project-root   = ' + $ProjectRoot)
Write-Output ('unity-dir      = ' + $unityDir)
Write-Output ('assets-dir     = ' + (Join-Path $unityDir 'Assets'))
if ($EngineRoot) { Write-Output ('engine-runtime = ' + $EngineRoot) }
else { Write-Output 'engine-runtime = NOT FOUND (hosts that link engine sources need -p:CloverEngineRuntimeDir=<dir> or env CLOVER_ENGINE_ROOT)' }
if ($editor) { Write-Output ('unity-editor   = ' + $editor) }
else { Write-Output 'unity-editor   = NOT FOUND (set env UNITY_EDITOR_ROOT, or pass -UnityEditorRoot)' }
Write-Output ('props          = ' + $propsState + '  ' + $propsDst)
Write-Output ('csproj         = ' + $csprojState + '  ' + $csprojDst)
Write-Output ('program.cs     = ' + $progState + '  ' + $progDst)
Write-Output ('NEXT 1: edit ' + $csprojDst)
Write-Output '        add one <Compile Include="$(CloverAssetsDir)\<dir>\*.cs" /> per source family this host owns'
Write-Output '        add <Compile Include="$(CloverEngineRuntimeDir)\Core\Rng.cs" /> for engine sources it forwards to'
Write-Output ('NEXT 2: write the assertions in ' + $progDst + ' -- Check(name, condition, detail)')
Write-Output ('NEXT 3: run  powershell -NoProfile -ExecutionPolicy Bypass -File ' + $runner + ' -HostsRoot ' + $HostsRoot)
exit 0
