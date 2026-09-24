<#
=============================================================================
 Invoke-AssetAudit.ps1 -- unreferenced-asset inventory for a Clover project.

   Rule: an asset that entered the project must be REACHABLE from the text the
   game itself can read (code / config / table / level text).  This is the
   STOCK variant -- it REPORTS, it never deletes anything.

   Five coverage forms (any single hit == covered):
     1  the extension-less file name appears literally in the reference text
     2  the asset's directory path (relative to the resource root) is named
     2b the leaf directory name (>= -MinLeafLen chars) is named
     3  frame-series file (<prefix>_<index>): index < the authoritative frame
        count for <prefix> in the sibling <-ManifestName>.  A frame set whose
        action is NOT declared in a READABLE manifest is genuinely extra
        (uncovered); a missing / unreadable manifest is UNKNOWN and is KEPT --
        a check that can wrongly delete live art is worse than no check.
     4  resources loaded by directory (LoadAll< / Resources.LoadAll) AND the
        directory itself is named in the reference text
     5  generated directory (sibling <-ManifestName> exists) and the file is
        not frame-indexed

   Exit codes: 0 = PASS (or -Warn report), 1 = uncovered assets in gate mode,
               2 = bad argument / missing project root.

   ASCII-only on purpose: Windows PowerShell 5.1 decodes a BOM-less .ps1 with
   non-ASCII bytes as ANSI, which silently eats quotes and kills the script.

   Usage:
     powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-AssetAudit.ps1 `
         -ProjectRoot <project root> -Warn           # inventory, exit 0
     powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-AssetAudit.ps1 `
         -ProjectRoot <project root>                 # gate: uncovered > 0 => 1
     powershell ... -ProjectRoot <root> -ResourceRoot <dir> -SourceRoots <dir>,<dir>
=============================================================================
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ProjectRoot,
    [string]$AssetsRoot = '',
    [string]$ResourceRoot = '',
    [string[]]$SourceRoots = @(),
    [string[]]$TextGlobs = @('*.cs', '*.json', '*.txt', '*.prefab', '*.asset'),
    [string[]]$ExtraTextGlobs = @('Resources/Levels/*.txt', 'StreamingAssets/*.txt'),
    [string]$ManifestName = 'manifest.json',
    [string]$FrameIndexRegex = '_\d+$',
    [string]$FrameDigitsRegex = '(\d+)$',
    [int]$MinLeafLen = 3,
    [string]$SkipNamePrefix = '_',
    [string[]]$HighlightDirs = @(),
    [int]$TopN = 10,
    [string]$OutFile = '',
    [switch]$Warn
)

$ErrorActionPreference = 'Continue'

$report = New-Object 'System.Collections.Generic.List[string]'
function Emit([string]$line) { [void]$report.Add($line); Write-Output $line }
function Abort([string]$msg) { Write-Output ('FAIL asset-audit: ' + $msg); exit 2 }

function Resolve-CandDir([string]$Base, [string[]]$Candidates) {
    foreach ($c in $Candidates) {
        if (-not $c) { continue }
        $p = $c
        if (-not [System.IO.Path]::IsPathRooted($p)) { $p = Join-Path $Base $c }
        if (Test-Path -LiteralPath $p -PathType Container) { return (Resolve-Path -LiteralPath $p).Path }
    }
    return ''
}

# ---------------------------------------------------------------- paths
if (-not (Test-Path -LiteralPath $ProjectRoot -PathType Container)) {
    Abort ('project root not found -> ' + $ProjectRoot)
}
$ProjectRoot = (Resolve-Path -LiteralPath $ProjectRoot).Path

if ($AssetsRoot) {
    $AssetsRoot = Resolve-CandDir $ProjectRoot @($AssetsRoot)
} else {
    $AssetsRoot = Resolve-CandDir $ProjectRoot @('client\Assets', 'Assets')
}
if (-not $AssetsRoot) {
    Abort ('no Unity Assets directory under ' + $ProjectRoot + ' -- pass -AssetsRoot <dir>')
}

if ($ResourceRoot) {
    $ResourceRoot = Resolve-CandDir $ProjectRoot @($ResourceRoot)
} else {
    $ResourceRoot = Resolve-CandDir $ProjectRoot @((Join-Path $AssetsRoot 'Resources'))
}

if (-not $SourceRoots -or $SourceRoots.Count -eq 0) {
    $SourceRoots = @()
    foreach ($n in @('Scripts', 'Editor', 'Configs')) {
        $p = Join-Path $AssetsRoot $n
        if (Test-Path -LiteralPath $p -PathType Container) { $SourceRoots += $p }
    }
}

Emit '===== asset-audit (STOCK: report only, never deletes) ====='
Emit ('project-root  = ' + $ProjectRoot)
Emit ('assets-root   = ' + $AssetsRoot)
Emit ('resource-root = ' + $ResourceRoot)
Emit ('mode          = ' + $(if ($Warn) { 'warn (report only, exit 0)' } else { 'gate (uncovered > 0 => exit 1)' }))

if (-not $ResourceRoot) {
    Emit 'RESULT: resources=0 covered=0 uncovered=0 unknown=0 uncovered-MB=0 -> SKIP (exit 0)'
    if ($OutFile) { [System.IO.File]::WriteAllLines($OutFile, $report, (New-Object System.Text.UTF8Encoding($false))) }
    exit 0
}

# ---------------------------------------------------------------- reference text
$sb = New-Object System.Text.StringBuilder
$srcFiles = 0

foreach ($root in $SourceRoots) {
    if (-not (Test-Path -LiteralPath $root -PathType Container)) { Emit ('  skip (missing) ' + $root); continue }
    foreach ($g in $TextGlobs) {
        foreach ($f in @(Get-ChildItem -LiteralPath $root -Recurse -File -Filter $g -ErrorAction SilentlyContinue)) {
            try { [void]$sb.AppendLine([System.IO.File]::ReadAllText($f.FullName, [System.Text.Encoding]::UTF8)); $srcFiles++ } catch { }
        }
    }
}
foreach ($g in $ExtraTextGlobs) {
    if (-not $g) { continue }
    $p = $g
    if (-not [System.IO.Path]::IsPathRooted($p)) { $p = Join-Path $AssetsRoot $g }
    foreach ($f in @(Get-ChildItem -Path $p -File -ErrorAction SilentlyContinue)) {
        try { [void]$sb.AppendLine([System.IO.File]::ReadAllText($f.FullName, [System.Text.Encoding]::UTF8)); $srcFiles++ } catch { }
    }
}
$blob = $sb.ToString()
Emit ('ref-source files = ' + $srcFiles + ' ; text chars = ' + $blob.Length)

$hasLoadAll = ($blob -match 'LoadAll<') -or ($blob -match 'Resources\.LoadAll')
$hasFrameHelper = ($blob -match 'FrameCount\w*\s*=') -or ($blob -match '\bFrame\s*\(')
if (-not $hasFrameHelper -and -not $hasLoadAll) {
    Emit 'WARN no frame-series helper / LoadAll found in the scanned sources -- check -SourceRoots / -TextGlobs before trusting the counts'
}

# ---------------------------------------------------------------- asset scan
$all = @(Get-ChildItem -LiteralPath $ResourceRoot -Recurse -File -ErrorAction SilentlyContinue |
         Where-Object { $_.Extension -ne '.meta' })
$totalFiles = $all.Count
$totalBytes = 0
foreach ($a in $all) { $totalBytes += $a.Length }

$covered = 0
$unknown = 0
$skipped = 0
$unref = New-Object 'System.Collections.Generic.List[object]'
$nameCache = @{}

foreach ($f in $all) {
    $name = [System.IO.Path]::GetFileNameWithoutExtension($f.Name)
    if ($SkipNamePrefix -and $name.StartsWith($SkipNamePrefix)) { $skipped++; continue }

    $relD = $f.DirectoryName.Substring($ResourceRoot.Length).TrimStart('\', '/').Replace('\', '/')
    if ($relD) { $relD = $relD + '/' }
    $leaf = (($relD.TrimEnd('/')) -split '/')[-1]

    # form 1: exact file name
    $hit = $nameCache[$name]
    if ($null -eq $hit) {
        $hit = ($blob -match [regex]::Escape($name))
        $nameCache[$name] = $hit
    }
    if ($hit) { $covered++; continue }

    # form 2: directory path named anywhere
    if ($relD -and ($blob -match [regex]::Escape($relD.TrimEnd('/')))) { $covered++; continue }

    # form 2b: leaf directory name named anywhere
    if ($leaf.Length -ge $MinLeafLen -and ($blob -match [regex]::Escape($leaf))) { $covered++; continue }

    # form 3: frame series, bounded by the authoritative count in the manifest
    if ($name -match $FrameIndexRegex) {
        $manState = 'missing'
        $bound = -1
        $man = Join-Path $f.DirectoryName $ManifestName
        if (Test-Path -LiteralPath $man) {
            try {
                $j = Get-Content -LiteralPath $man -Raw -Encoding UTF8 | ConvertFrom-Json
                $manState = 'readable'
                if (-not $j.actions) { $manState = 'unreadable' }
                if ($manState -eq 'readable') {
                    $pre = $name -replace $FrameIndexRegex, ''
                    $parts = $pre -split '_'
                    if ($parts.Count -ge 2) {
                        $act = ($parts[0..($parts.Count - 2)] -join '_')
                        $ap = $j.actions.PSObject.Properties | Where-Object { $_.Name -ieq $act } | Select-Object -First 1
                        if ($null -ne $ap) { $bound = [int]$ap.Value.frames }
                    }
                }
            } catch { $manState = 'unreadable' }
        }
        if ($manState -ne 'readable') {
            # missing / unreadable -> KEEP (fail-safe), never reported as unreferenced
            $unknown++
            continue
        }
        if ($bound -lt 0) { $unref.Add($f); continue }   # action not declared -> genuinely extra
        # MEASURED DEFECT this fixes: reading the frame index from a capture group
        # of -FrameIndexRegex silently yields 0 when that pattern has no group
        # (= no group 1), so every frame passed the bound check and the frame
        # form whitelisted the whole series. -FrameDigitsRegex owns the group.
        $dm = [regex]::Match($name, $FrameDigitsRegex)
        if (-not $dm.Success) {
            $unknown++      # fail-safe: cannot read the index -> keep, never "unreferenced"
            continue
        }
        $digVal = $dm.Value
        if ($dm.Groups.Count -ge 2) { $digVal = $dm.Groups[1].Value }
        $idx = [int]$digVal
        if ($idx -lt $bound) { $covered++; continue }
        # idx >= declared frames -> genuinely extra frame (report only)
        $unref.Add($f); continue
    }

    # form 4: directory-level load, only when this directory is itself named
    if ($hasLoadAll -and $relD -and (($blob -match [regex]::Escape($leaf)) -or ($blob -match [regex]::Escape($relD.TrimEnd('/'))))) {
        $covered++
        continue
    }

    # form 5: generated directory, file is not frame-indexed
    if (($name -notmatch $FrameIndexRegex) -and (Test-Path -LiteralPath (Join-Path $f.DirectoryName $ManifestName))) {
        $covered++
        continue
    }

    # A file sitting directly inside the resource root has an EMPTY relative
    # directory -- that is a normal layout, not a parse failure. (The template
    # this tool came from counted those as UNKNOWN, i.e. it never reported
    # root-level orphans at all.)
    $unref.Add($f)
}

$unrefBytes = 0
foreach ($u in $unref) { $unrefBytes += $u.Length }
$unrefMB = [math]::Round($unrefBytes / 1MB, 2)

Emit ('resource files = ' + $totalFiles + ' (size = ' + [math]::Round($totalBytes / 1MB, 1) + ' MB)')
Emit ('skipped (name starts with ' + $SkipNamePrefix + ') = ' + $skipped)
Emit ('resources=' + $totalFiles + ' covered=' + $covered + ' uncovered=' + $unref.Count + ' unknown=' + $unknown + ' uncovered-MB=' + $unrefMB + ' total-MB=' + [math]::Round($totalBytes / 1MB, 1))

foreach ($hd in $HighlightDirs) {
    $n = 0
    foreach ($u in $unref) {
        $r = $u.FullName.Substring($ResourceRoot.Length).TrimStart('\', '/')
        if (($r -split '[\\/]')[0] -ieq $hd) { $n++ }
    }
    Emit ('highlight dir ' + $hd + ' -> uncovered = ' + $n)
}

if ($unref.Count -gt 0) {
    Emit ('uncovered by directory (top ' + $TopN + '):')
    $grp = @($unref | ForEach-Object { $_.DirectoryName.Substring($ResourceRoot.Length).TrimStart('\', '/') } |
             Group-Object | Sort-Object Count -Descending | Select-Object -First $TopN)
    foreach ($g in $grp) {
        $label = $g.Name
        if (-not $label) { $label = '(resource root)' }
        Emit ('  ' + $label + '  ' + $g.Count)
    }
    Emit 'uncovered list (one line per file):'
    foreach ($u in $unref) { Emit ('UNREF  ' + $u.FullName.Substring($ResourceRoot.Length).TrimStart('\', '/')) }
}

if ($Warn) {
    Emit ('RESULT: resources=' + $totalFiles + ' covered=' + $covered + ' uncovered=' + $unref.Count + ' unknown=' + $unknown + ' uncovered-MB=' + $unrefMB + ' -> WARN (exit 0)')
    if ($OutFile) { [System.IO.File]::WriteAllLines($OutFile, $report, (New-Object System.Text.UTF8Encoding($false))) }
    exit 0
}
if ($unref.Count -gt 0) {
    Emit ('RESULT: resources=' + $totalFiles + ' covered=' + $covered + ' uncovered=' + $unref.Count + ' unknown=' + $unknown + ' uncovered-MB=' + $unrefMB + ' -> FAIL (uncovered assets exist, exit 1)')
    if ($OutFile) { [System.IO.File]::WriteAllLines($OutFile, $report, (New-Object System.Text.UTF8Encoding($false))) }
    exit 1
}
Emit ('RESULT: resources=' + $totalFiles + ' covered=' + $covered + ' uncovered=0 unknown=' + $unknown + ' uncovered-MB=0 -> PASS (exit 0)')
if ($OutFile) { [System.IO.File]::WriteAllLines($OutFile, $report, (New-Object System.Text.UTF8Encoding($false))) }
exit 0
