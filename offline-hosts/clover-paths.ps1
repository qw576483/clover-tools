# ============================================================================
#  clover-paths.ps1 -- shared path resolution for the offline-host harness.
#
#  Dot-source it from a wrapper script:
#      . "$PSScriptRoot\clover-paths.ps1"
#
#  Every resolver is a bounded UPWARD SEARCH (default 1..8 levels above the
#  starting directory) looking for a MARKER, never a hard-coded relative depth
#  (the source project hard-coded ..\..\..\..\  in 13 csproj files -- moving a
#  host one level, or renaming a folder, silently resolved to a different tree).
#
#  ASCII-only on purpose: Windows PowerShell 5.1 decodes a BOM-less .ps1 with
#  non-ASCII bytes as ANSI and then eats quotes, killing the whole script.
# ============================================================================

function Find-CloverUnityDir {
    <#
      Returns the directory that CONTAINS the Unity Assets folder
      (e.g. <root>\client when <root>\client\Assets exists, else <root>),
      or '' when no marker is found within -MaxUp levels.
    #>
    [CmdletBinding()]
    param(
        [string]$Start = (Get-Location).Path,
        [string]$AssetsDirName = 'Assets',
        [int]$MaxUp = 8
    )
    if (-not (Test-Path -LiteralPath $Start -PathType Container)) { return '' }
    $d = (Resolve-Path -LiteralPath $Start).Path
    for ($i = 0; $i -le $MaxUp; $i++) {
        if (Test-Path -LiteralPath (Join-Path $d ('client\' + $AssetsDirName)) -PathType Container) {
            return (Join-Path $d 'client')
        }
        if (Test-Path -LiteralPath (Join-Path $d $AssetsDirName) -PathType Container) { return $d }
        $parent = Split-Path $d -Parent
        if (-not $parent -or $parent -eq $d) { break }
        $d = $parent
    }
    return ''
}

function Find-CloverProjectRoot {
    <#
      Returns the project root (the directory that holds client/ or Assets/),
      or '' when not found within -MaxUp levels.
    #>
    [CmdletBinding()]
    param(
        [string]$Start = (Get-Location).Path,
        [string]$AssetsDirName = 'Assets',
        [int]$MaxUp = 8
    )
    $u = Find-CloverUnityDir -Start $Start -AssetsDirName $AssetsDirName -MaxUp $MaxUp
    if (-not $u) { return '' }
    if ((Split-Path $u -Leaf) -ieq 'client') { return (Split-Path $u -Parent) }
    return $u
}

function Find-CloverEngineRuntime {
    <#
      Returns the engine package's Runtime directory, or ''.
      Looks for <ancestor>\<EngineDirName>\Runtime and
      <ancestor>\Packages\<EngineDirName>\Runtime, 1..MaxUp levels up.
    #>
    [CmdletBinding()]
    param(
        [string]$Start = (Get-Location).Path,
        [string]$EngineDirName = 'clover-client-unity-engine',
        [int]$MaxUp = 8
    )
    if (-not (Test-Path -LiteralPath $Start -PathType Container)) { return '' }
    $d = (Resolve-Path -LiteralPath $Start).Path
    for ($i = 0; $i -le $MaxUp; $i++) {
        foreach ($c in @((Join-Path $d $EngineDirName), (Join-Path $d ('Packages\' + $EngineDirName)))) {
            $rt = Join-Path $c 'Runtime'
            if (Test-Path -LiteralPath $rt -PathType Container) { return (Resolve-Path -LiteralPath $rt).Path }
        }
        $parent = Split-Path $d -Parent
        if (-not $parent -or $parent -eq $d) { break }
        $d = $parent
    }
    return ''
}

function Resolve-UnityEditorRoot {
    <#
      Returns a Unity editor directory whose managed DLLs are really on disk, or ''.
      Order: -Root argument  ->  env UNITY_EDITOR_ROOT  ->  -Version under -HubRoot
             -> highest version directory name under -HubRoot that owns the DLL.
      The last step is the auto-detect; it removes the "which editor version is
      installed on this machine" hard-coding that breaks every csproj on a
      machine change.
    #>
    [CmdletBinding()]
    param(
        [string]$Root = '',
        [string]$HubRoot = '',
        [string]$Version = '',
        [string]$ProbeRelative = 'Data\Managed\UnityEngine\UnityEngine.CoreModule.dll'
    )
    $probe = $ProbeRelative
    foreach ($cand in @($Root, $env:UNITY_EDITOR_ROOT)) {
        if (-not $cand) { continue }
        if (Test-Path -LiteralPath (Join-Path $cand $probe) -PathType Leaf) {
            return (Resolve-Path -LiteralPath $cand).Path
        }
    }
    if (-not $HubRoot) { $HubRoot = Join-Path $env:ProgramFiles 'Unity\Hub\Editor' }
    if (-not (Test-Path -LiteralPath $HubRoot -PathType Container)) { return '' }
    if ($Version) {
        $try = Join-Path $HubRoot ($Version + '\Editor')
        if (Test-Path -LiteralPath (Join-Path $try $probe) -PathType Leaf) { return $try }
    }
    $dirs = @(Get-ChildItem -LiteralPath $HubRoot -Directory -ErrorAction SilentlyContinue |
              Sort-Object -Property Name -Descending)
    foreach ($d in $dirs) {
        $ed = Join-Path $d.FullName 'Editor'
        if (Test-Path -LiteralPath (Join-Path $ed $probe) -PathType Leaf) { return $ed }
    }
    return ''
}
