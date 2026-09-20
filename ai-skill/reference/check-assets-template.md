# 资源引用闸门模板（`tools/check-assets.ps1`）

> **为什么有这个文件**：`SKILL.md` §8 / `asset-sources.md` §8.1 / `game-delivery.md` §9.5 第 2 条 /
> `rules-full.md` §1.9-7 **四处点名**"闸门 `tools/check-assets.ps1`"，但此前**一处都没给骨架** ——
> 结果是每个项目都得自己现编，而现编出来的那一版（只做"文件名在源码里 `-notmatch`"）
> **必然把帧序列资源判成未引用**（实测代价见 `asset-sources.md` §8.1）。
> 本文件补上骨架，并把判据升级成"**五种引用形态覆盖**"。

## 怎么用

1. 复制下面骨架到 **`<项目根>/tools/check-assets.ps1`**（**编码 UTF-8 无 BOM，正文一律 ASCII**）。
2. 存量工程先跑 **`-Warn`**（只报数、退出码 0）：`powershell -NoProfile -ExecutionPolicy Bypass -File tools/check-assets.ps1 -Warn`
3. 新项目 / 新加素材：**不带 `-Warn`** ⇒ 有未覆盖文件即退出码 1（闸门形态）。
4. ⛔ **`-Warn` 的报告不是删除清单**。必须逐组人工核过（尤其大目录），确认不是形态 ②③④ 漏配，才谈裁剪。

## 判据（五种引用形态，任一覆盖即"已引用"）

| # | 形态 | 在脚本里的做法 |
|---|---|---|
| ① | 精确文件名 | 全文字面量 `-match` |
| ② | 目录 / 前缀常量 | 取资源**所在目录名**在所有 `.cs` 里 `-match`（如 `D2/Monsters/`、`D2/UI/Quest/`）；命中 ⇒ 整目录放行 |
| ③ | 帧序列拼名 | **按帧号与权威帧数对账**：导出器 manifest 里 `actions.<动作>.frames` 是权威值，文件名是 `<动作>_<方向>_<帧号>.png` ⇒ **帧号 < 该动作 frames 才算覆盖**。⛔ 不要用"父目录被提到就放行"（那是全局放行 ⇒ 报 0，等于永不可裁） |
| ④ | 目录级加载 | `LoadAll<` / `Resources.LoadAll` 出现 **且该文件所在目录本身在源码里被具名** ⇒ 放行（单独一个全局标志不许放行整棵树） |
| ⑤ | 生成物登记 | 目录里有 `manifest.json` **且该文件不是帧序列文件**（`_<数字>` 结尾）⇒ 放行；帧序列文件一律回 ③ 对账 |

**默认是"保留"**：任何一步解析不出来（读不到文件、编码异常）⇒ 该文件记为 **UNKNOWN**（不计入未覆盖）并打印出来。
⛔ 不许把"解析失败"当成"未引用" —— 会误删活资源（`SKILL.md` §1.11 第 2 条：会误报的检查比没有更糟）。

## 骨架

```powershell
# check-assets.ps1 -- Resources reference gate (SKILL 8 / asset-sources 8.1)
# ASCII-only on purpose; non-ASCII path segments are built from code points.
param([switch]$Warn)

$ErrorActionPreference = 'Continue'
$root    = Split-Path $PSScriptRoot -Parent
$resRoot = Join-Path $root 'client\Assets\Resources'
$srcDirs = @('client\Assets\Scripts', 'client\Assets\Editor', 'client\Assets\Configs')

# reference-source full text (basis for forms 1/3/4)
$sb = New-Object System.Text.StringBuilder
foreach ($d in $srcDirs) {
  $p = Join-Path $root $d
  if (-not (Test-Path $p)) { continue }
  foreach ($f in @(Get-ChildItem $p -Recurse -File -Include *.cs,*.json,*.txt,*.prefab,*.asset -ErrorAction SilentlyContinue)) {
    [void]$sb.AppendLine([System.IO.File]::ReadAllText($f.FullName, [Text.Encoding]::UTF8))
  }
}
# level data / data tables (add if the project has them, otherwise the glob just matches nothing)
foreach ($g in @('client\Assets\Resources\Levels\*.txt', 'client\Assets\StreamingAssets\*.txt')) {
  foreach ($f in @(Get-ChildItem (Join-Path $root $g) -File -ErrorAction SilentlyContinue)) {
    [void]$sb.AppendLine([System.IO.File]::ReadAllText($f.FullName, [Text.Encoding]::UTF8))
  }
}
$src = $sb.ToString()

# form 4: directory-level loading (only counts together with a mentioned directory, see (4))
$hasLoadAll     = ($src -match 'LoadAll<') -or ($src -match 'Resources\.LoadAll')
# form 3 sanity check: a project that composes frame names at runtime must show the helper somewhere.
# If neither the helper nor LoadAll is found, the scan scope is probably wrong -- say so out loud,
# because in that case every frame-indexed file falls through to "uncovered" and the count is junk.
$hasFrameHelper = ($src -match 'FrameCount\w+\s*=') -or ($src -match '\bFrame\s*\(')
if (-not $hasFrameHelper -and -not $hasLoadAll) {
  Write-Output 'WARN no frame-series helper / LoadAll found in the scanned sources -- check the scan scope before trusting the counts'
}

$files   = @(Get-ChildItem $resRoot -Recurse -File -ErrorAction SilentlyContinue | Where-Object { $_.Extension -ne '.meta' })
$covered = 0; $unknown = 0; $unref = @()
foreach ($f in $files) {
  $name = [System.IO.Path]::GetFileNameWithoutExtension($f.Name)
  $dir  = $f.DirectoryName
  $relD = $dir.Substring($resRoot.Length).TrimStart('\','/').Replace('\','/') + '/'   # e.g. D2/Monsters/zombie/

  # (1) exact file name
  if ($src -match [regex]::Escape($name)) { $covered++; continue }
  # (2) directory / prefix constant mentioned anywhere
  if ($src -match [regex]::Escape($relD.TrimEnd('/'))) { $covered++; continue }
  # (2b) leaf directory name mentioned (covers D2UiQuest = D2Ui + "Quest/")
  $leaf = ($relD.TrimEnd('/') -split '/')[-1]
  if ($leaf.Length -ge 3 -and ($src -match [regex]::Escape($leaf))) { $covered++; continue }
  # (3) frame series: the frame index must be < the authoritative count for that action.
  #     Two WRONG ways, both seen in practice:
  #       a) name-only matching        -> 88% false positives (frame names are composed at runtime);
  #       b) a GLOBAL flag ("the project contains Frame(" -> let every _<n> file through)
  #          -> reports 0 uncovered, i.e. "nothing can ever be trimmed" (the opposite failure).
  #     The authoritative number lives in the exporter manifest next to the frames:
  #       manifest.json : actions.<action>.frames   (per direction; dirs = direction count)
  #     Naming is <action>_<direction>_<frame>.png  -> per-prefix bound check:
  if ($name -match '_\d+$') {
    $man = Join-Path $dir 'manifest.json'
    $bound = -1
    $manState = 'unreadable'
    if (Test-Path $man) {
      try {
        $j = Get-Content $man -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($j.actions) { $manState = 'readable' }
        $pre = $name -replace '_\d+$',''
        $parts = $pre -split '_'
        if ($parts.Count -ge 2) {
          $act = ($parts[0..($parts.Count-2)] -join '_')
          $ap = $j.actions.PSObject.Properties | Where-Object { $_.Name -ieq $act } | Select-Object -First 1
          if ($ap -ne $null) { $bound = [int]$ap.Value.frames }
          # NOTE on manifest `actions.<a>.skipped`: it lists SKIPPED **LAYERS** as strings, e.g.
          #   "layer#4(S1,wc=1ht): not drawn for bare hands (original behaviour); file exists"
          # -> NOT frame indices. Never use it as a frame filter: an [int] cast throws, and those
          #    files are still REQUIRED (the same unit draws them under other weapon classes).
          #    Verified on clover-project-diablo2 (2026-09-20) after a wrong first guess.
        }
      } catch { $manState = 'unreadable' }
    }
    # Two "no bound" cases, and they are NOT the same thing:
    #   (1) manifest missing / unreadable  -> UNKNOWN: keep (fail-safe is the whole point)
    #   (2) manifest readable but this ACTION is not declared in it -> frame set genuinely extra
    if ($manState -eq 'unreadable') { $unknown++; $covered++; continue }
    if ($bound -lt 0) { $unref += $f; continue }
    $idx = [int]([regex]::Match($name, '_(\d+)$').Groups[1].Value)
    if ($idx -lt $bound) { $covered++; continue }
    # idx >= declared frames -> genuinely extra frame (still: report only, never delete automatically)
  }
  # (4) directory-level loading: accept it ONLY when this directory is itself named in the sources
  #     (a bare "somewhere the project calls LoadAll" must not whitelist the whole tree).
  if ($hasLoadAll -and ($src -match [regex]::Escape($leaf) -or $src -match [regex]::Escape($relD.TrimEnd('/')))) { $covered++; continue }
  # (5) generated directory: only files that are NOT frame-indexed are covered by registration alone
  #     (frame-indexed files must pass the bound check in (3) -- otherwise every unit dir that has a
  #      manifest.json would whitelist all of its frames, which is failure (b) again).
  if (($name -notmatch '_\d+$') -and (Test-Path (Join-Path $dir 'manifest.json'))) { $covered++; continue }

  if ([string]::IsNullOrWhiteSpace($relD)) { $unknown++; Write-Output ('UNKNOWN   ' + $f.FullName); continue }
  $unref += $f
}

$mb = 0.0
if ($unref.Count -gt 0) { $mb = (($unref | Measure-Object Length -Sum).Sum / 1MB) }
Write-Output ('resources=' + $files.Count + ' covered=' + $covered + ' uncovered=' + $unref.Count + ' unknown=' + $unknown + ' uncovered-MB=' + [math]::Round($mb,2))
if ($unref.Count -gt 0) {
  Write-Output 'uncovered by directory (top 10):'
  $unref | ForEach-Object { $_.DirectoryName.Substring($resRoot.Length) } | Group-Object |
    Sort-Object Count -Descending | Select-Object -First 10 |
    ForEach-Object { Write-Output ('  ' + $_.Name + '  ' + $_.Count) }
  $unref | ForEach-Object { Write-Output ('UNREF  ' + $_.FullName.Substring($resRoot.Length)) }
}
if ($Warn) { Write-Output 'RESULT: WARN (report only)'; exit 0 }
if ($unref.Count -gt 0) { Write-Output 'RESULT: FAIL (uncovered resources exist)'; exit 1 }
Write-Output 'RESULT: PASS'
exit 0
```

## 报告怎么读（⛔ 别直接当删除清单）

- `uncovered` 大、且集中在**大目录**（Monsters / Chars / UI）⇒ 十有八九是形态 ②③④ 没被识别：
  **先去 `Core/ResPaths.cs` 找目录常量与 `FrameCount*`**，而不是删文件。
- `uncovered-MB` 才是"删了能省多少"的真实上限 —— 文件数看着吓人、体积常常很小
  （实测：10529 个 ≈ 29.5 MB，因为都是几十 KB 的帧图）。
- `unknown > 0` ⇒ 先把解析异常修掉再看结论（解析失败不算未引用）。
