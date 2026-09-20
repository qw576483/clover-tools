# skill-health.ps1 -- self-check for the clover-engine skill package
#
# Why: "an instruction is a request, a gate is a guarantee". The rule layer stays small only if
#      something fails loudly when it grows back. Run this after ANY edit of this skill.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File <skill-dir>\scripts\skill-health.ps1 -RepoRoot <repo-copy-dir>
#
# Checks (each line prints PASS / FAIL / WARN / SKIP):
#   1 skill-size          SKILL.md must stay under a byte budget (progressive disclosure entry file)
#   2 router-reachable    every file referenced by SKILL.md must exist (no dangling pointers)
#   3 no-relaxed-phrasing the rule layer must never be relaxed by "project wins over global" wording
#   4 copies-synced       host install copy == repo source copy (name + size)
#   5 archive-pending     the transitional full-rules archive must eventually be digested away
#
# ASCII-only on purpose (PowerShell 5.1 parses non-ASCII .ps1 without BOM as ANSI).

param([string]$RepoRoot = '')

$ErrorActionPreference = 'Continue'
$root = Split-Path $PSScriptRoot -Parent
$fail = 0

function Say([string]$status, [string]$name, [string]$detail) {
    Write-Output ("{0,-5} {1}  {2}" -f $status, $name, $detail)
}

# -- 1) size budget -------------------------------------------------------------------
$skill = Join-Path $root 'SKILL.md'
if (-not (Test-Path $skill)) {
    $fail++; Say 'FAIL' 'skill-size' ('missing ' + $skill)
} else {
    $bytes = (Get-Item $skill).Length
    $limit = 16384
    if ($bytes -le $limit) { Say 'PASS' 'skill-size' ('' + $bytes + ' bytes <= ' + $limit) }
    else { $fail++; Say 'FAIL' 'skill-size' ('' + $bytes + ' bytes > ' + $limit + ' -- move details to reference/** and link them from the router table') }
}

# -- 2) router reachability -----------------------------------------------------------
if (Test-Path $skill) {
    $txt = [System.IO.File]::ReadAllText($skill, [System.Text.Encoding]::UTF8)
    $miss = @()
    foreach ($m in [regex]::Matches($txt, '(reference|patterns|scaffold|experience|scripts)/[A-Za-z0-9_\-/]+\.(md|ps1|py)')) {
        $rel = $m.Value -replace '/', '\'
        if (-not (Test-Path (Join-Path $root $rel))) { $miss += $m.Value }
    }
    $miss = @($miss | Sort-Object -Unique)
    if ($miss.Count -eq 0) { Say 'PASS' 'router-reachable' 'all referenced files exist' }
    else { $fail++; Say 'FAIL' 'router-reachable' ($miss -join ', ') }
}

# -- 3) relaxed phrasing: a hit is only suspicious OUTSIDE forbid/check wording --------
#    (a check that cries wolf is worse than no check -- the same phrase legitimately
#     appears in "never let 'project wins' relax a hard rule" and in grep instructions)
$cProjectWins = ([char[]]@(0x4EE5, 0x672C, 0x9879, 0x76EE, 0x4E3A, 0x51C6) -join '')   # "yi ben xiang mu wei zhun"
$cGlobalFirst = ([char[]]@(0x4F18, 0x5148, 0x4E8E, 0x5168, 0x5C40) -join '')           # "you xian yu quan ju"
$ctxWords = @(
    ([char[]]@(0x4E0D, 0x8BB8) -join ''),        # bu xu      (must not)
    ([char[]]@(0x7981, 0x6B62) -join ''),        # jin zhi    (forbidden)
    ([char[]]@(0x653E, 0x5BBD) -join ''),        # fang kuan  (relax)
    ([char[]]@(0x52A0, 0x4E25) -join ''),        # jia yan    (tighten)
    ([char[]]@(0x68C0, 0x67E5) -join ''),        # jian cha   (check)
    ([char[]]@(0x81EA, 0x68C0) -join '')         # zi jian    (self-check)
)
$suspicious = @()
$ctxHits = 0
foreach ($f in @(Get-ChildItem $root -Recurse -File -Include *.md -ErrorAction SilentlyContinue)) {
    $arr = @([System.IO.File]::ReadAllLines($f.FullName, [System.Text.Encoding]::UTF8))
    for ($k = 0; $k -lt $arr.Count; $k++) {
        $line = $arr[$k]
        if (-not ($line.Contains($cProjectWins) -or $line.Contains($cGlobalFirst))) { continue }
        # context window = this line +/- 2 lines: the forbid/check wording often sits on a neighbour
        # (a quoted history sentence, or a grep pattern followed by "judge each hit" on the next line)
        $lo = [Math]::Max(0, $k - 2); $hi = [Math]::Min($arr.Count - 1, $k + 2)
        $win = ($arr[$lo..$hi] -join ' ')
        $isCtx = $false
        foreach ($w in $ctxWords) { if ($win.Contains($w)) { $isCtx = $true; break } }
        # quoting is not asserting: a hit wrapped in 「」 / "" / backticks is a quotation
        # (e.g. quoting a wrong historical wording) or a grep pattern, not a rule relaxation
        if (-not $isCtx) {
            $idx = $line.IndexOf($cProjectWins); if ($idx -lt 0) { $idx = $line.IndexOf($cGlobalFirst) }
            if ($idx -ge 0) {
                $before = $line.Substring(0, $idx)
                $bt = ([regex]::Matches($before, [string][char]0x60)).Count
                $oq = ([char]0x300C); $cq = ([char]0x300D)     # 「 」
                $du = ([char]0x201C); $dc = ([char]0x201D)     # “ ”
                $o = [Math]::Max($before.LastIndexOf($oq), $before.LastIndexOf($du))
                $c = [Math]::Max($before.LastIndexOf($cq), $before.LastIndexOf($dc))
                if ((($bt % 2) -eq 1) -or ($o -gt $c)) { $isCtx = $true }
            }
        }
        if ($isCtx) { $ctxHits++ } else { $suspicious += ($f.Name + ':' + ($k + 1) + '  ' + $line.Trim()) }
    }
}
if ($suspicious.Count -eq 0) { Say 'PASS' 'no-relaxed-phrasing' ('' + $ctxHits + ' keyword hit(s), all inside forbid/check wording') }
else {
    $fail++; Say 'FAIL' 'no-relaxed-phrasing' ('' + $suspicious.Count + ' line(s) actually relax the rule layer')
    $suspicious | Select-Object -First 5 | ForEach-Object { Write-Output ('            ' + $_) }
}

# -- 4) the two copies must be identical ----------------------------------------------
if ($RepoRoot -ne '' -and (Test-Path $RepoRoot)) {
    $a = @(Get-ChildItem $root -Recurse -File | ForEach-Object { $_.FullName.Substring($root.Length + 1) })
    $b = @(Get-ChildItem $RepoRoot -Recurse -File | ForEach-Object { $_.FullName.Substring($RepoRoot.Length + 1) })
    $d = @()
    foreach ($f in $a) {
        if ($b -notcontains $f) { $d += ('ONLY-HOST ' + $f) }
        elseif ((Get-Item (Join-Path $root $f)).Length -ne (Get-Item (Join-Path $RepoRoot $f)).Length) { $d += ('SIZE ' + $f) }
    }
    foreach ($f in $b) { if ($a -notcontains $f) { $d += ('ONLY-REPO ' + $f) } }
    if ($d.Count -eq 0) { Say 'PASS' 'copies-synced' ('host == repo (' + $a.Count + ' files)') }
    else { $fail++; Say 'FAIL' 'copies-synced' ($d -join ' | ') }
} else {
    Say 'SKIP' 'copies-synced' 'pass -RepoRoot <repo-copy> to compare the two copies'
}

# -- 5) the normative FULL rules must exist and be linked from the entry file ---------
#    Layering is the point: the entry file is a SHORT summary, the full version keeps every
#    sentence. Losing the full version = losing semantics (that is a FAIL, not a warning).
$full = Join-Path $root 'reference\rules-full.md'
if (-not (Test-Path $full)) {
    $fail++; Say 'FAIL' 'full-rules' 'reference/rules-full.md missing -- keep a normative full version; the entry file is the summary only'
} elseif (-not ([System.IO.File]::ReadAllText($skill, [System.Text.Encoding]::UTF8)).Contains('reference/rules-full.md')) {
    $fail++; Say 'FAIL' 'full-rules' 'reference/rules-full.md exists but SKILL.md does not link it (dangling knowledge)'
} else {
    $fLines = @([System.IO.File]::ReadAllLines($full, [System.Text.Encoding]::UTF8)).Count
    Say 'PASS' 'full-rules' ('full version kept: ' + $fLines + ' lines, linked from SKILL.md')
}

Write-Output ''
Write-Output ("===== summary: FAIL=" + $fail + " =====")
exit $(if ($fail -gt 0) { 1 } else { 0 })
