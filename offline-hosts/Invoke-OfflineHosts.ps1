<#
=============================================================================
 Invoke-OfflineHosts.ps1 -- run EVERY offline self-check host under a root.

   Discovery is a glob over *.csproj (bin/ and obj/ excluded), never a hand-kept
   list: the source project's one-click entry held 12 names while 14 hosts
   existed, so two hosts were never run by the entry point at all. A list is a
   silent omission waiting to happen; the filesystem is the list.

   Each host is compiled + run with `dotnet run` (offline, seconds, no editor).
   A per-host timeout turns a hung host into a FAIL instead of an unattended
   wait. The Unity editor directory is auto-resolved and passed down as
   -p:UnityEditorRoot, so no csproj carries a machine-specific version.

   Usage:
     powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-OfflineHosts.ps1 `
         -HostsRoot <dir> [-Filter '*check.csproj'] [-TimeoutSec 300] [-Quiet]
   Exit codes: 0 = every host passed, 1 = at least one host failed,
               2 = argument / discovery problem (no hosts root, no csproj).
=============================================================================
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$HostsRoot,
    [string]$Filter = '',
    [int]$TimeoutSec = 300,
    [string]$UnityEditorRoot = '',
    [switch]$Quiet
)

$ErrorActionPreference = 'Continue'
. (Join-Path $PSScriptRoot 'clover-paths.ps1')

function Quote-Arg([string]$a) {
    if ($a -match '\s') { return ('"' + $a + '"') }
    return $a
}

if (-not (Test-Path -LiteralPath $HostsRoot -PathType Container)) {
    Write-Output ('FAIL Invoke-OfflineHosts: hosts root not found -> ' + $HostsRoot)
    exit 2
}
$HostsRoot = (Resolve-Path -LiteralPath $HostsRoot).Path

$editor = Resolve-UnityEditorRoot -Root $UnityEditorRoot

$projs = @(Get-ChildItem -LiteralPath $HostsRoot -Recurse -File -Filter *.csproj -ErrorAction SilentlyContinue |
           Where-Object { $_.FullName -notmatch '\\(bin|obj)\\' } |
           Sort-Object -Property FullName)
if ($Filter) {
    $projs = @($projs | Where-Object { $_.Name -like $Filter -or $_.BaseName -like $Filter })
}
if ($projs.Count -eq 0) {
    Write-Output ('FAIL Invoke-OfflineHosts: no .csproj found under ' + $HostsRoot + ' (filter = "' + $Filter + '") -- run New-OfflineHost.ps1 first')
    exit 2
}

Write-Output '===== offline self-check hosts ====='
Write-Output ('hosts-root   = ' + $HostsRoot)
Write-Output ('discovered   = ' + $projs.Count + ' host(s)')
if ($editor) { Write-Output ('unity-editor = ' + $editor) }
else { Write-Output 'unity-editor = NOT RESOLVED -- set env UNITY_EDITOR_ROOT or pass -UnityEditorRoot; hosts will report the path they tried' }
Write-Output ('timeout      = ' + $TimeoutSec + ' s per host')

$tmp = [System.IO.Path]::GetTempPath()
$fail = 0
$failNames = @()
$i = 0
foreach ($p in $projs) {
    $i++
    $name = $p.BaseName
    $outF = Join-Path $tmp ('cloverhost-' + [guid]::NewGuid().ToString('N') + '.out')
    $errF = Join-Path $tmp ('cloverhost-' + [guid]::NewGuid().ToString('N') + '.err')

    $dotnetArgs = @('run', '--project', (Quote-Arg $p.FullName), '--nologo', '-v', 'q')
    if ($editor) { $dotnetArgs += @('-p:UnityEditorRoot=' + (Quote-Arg $editor)) }

    $t0 = Get-Date
    $proc = Start-Process -FilePath 'dotnet' -ArgumentList $dotnetArgs -WorkingDirectory $p.DirectoryName `
                          -NoNewWindow -PassThru -RedirectStandardOutput $outF -RedirectStandardError $errF
    $timedOut = $false
    if (-not $proc.WaitForExit($TimeoutSec * 1000)) {
        $timedOut = $true
        try { $proc.Kill() } catch { }
    } else {
        # MEASURED DEFECT this line fixes: with -RedirectStandardOutput/-Error the
        # timed WaitForExit(ms) can return before the async readers finish, and
        # $proc.ExitCode then reads back EMPTY -- a host that printed "ALL PASS"
        # was reported as FAIL by the harness. The parameterless WaitForExit()
        # after it publishes the exit code; $null is still treated as a FAIL.
        try { $proc.WaitForExit() } catch { }
    }
    $code = $null
    if (-not $timedOut) {
        try { if ($proc.HasExited) { $code = [int]$proc.ExitCode } } catch { $code = $null }
    }
    $secs = [math]::Round(((Get-Date) - $t0).TotalSeconds, 1)

    $out = ''
    if (Test-Path -LiteralPath $outF) { $out = [System.IO.File]::ReadAllText($outF) }
    $err = ''
    if (Test-Path -LiteralPath $errF) { $err = [System.IO.File]::ReadAllText($errF) }
    Remove-Item -LiteralPath $outF, $errF -Force -ErrorAction SilentlyContinue

    $codeText = '?'
    if ($null -ne $code) { $codeText = [string]$code }
    if ($timedOut) { $status = 'TIMEOUT' }
    elseif ($null -eq $code) { $status = 'FAIL (exit code unreadable)' }
    elseif ($code -eq 0) { $status = 'PASS' }
    else { $status = 'FAIL' }

    if ($status -ne 'PASS') { $fail++; $failNames += $name }

    Write-Output (('[{0}/{1}] ' -f $i, $projs.Count) + $name.PadRight(20) + ' exit=' + $codeText + '  ' + $status + '  (' + $secs + ' s)')

    if (-not $Quiet) {
        $merged = ($out + "`n" + $err)
        $lines = @(($merged -split "`r?`n") | Where-Object { $_ -match 'CHECK |ALL PASS|FAILED|\[FAIL\]|error [A-Z]{2}\d+' } | Select-Object -Last 4)
        if ($lines.Count -eq 0) {
            $lines = @(($merged -split "`r?`n") | Where-Object { $_.Trim() } | Select-Object -Last 2)
        }
        foreach ($l in $lines) { Write-Output ('         | ' + $l.Trim()) }
    }
}

Write-Output ('TOTAL_HOSTS=' + $projs.Count + ' FAILED=' + $fail)
if ($fail -gt 0) {
    Write-Output ('failed: ' + ($failNames -join ', '))
    exit 1
}
exit 0
