# ============================================================================
# capture-editor-window.ps1 —— **采「用户真正看到的那个画面」**：窗口级截屏。
#
# 为什么必须有它：
#   Unity 的 `ScreenCapture` / `unity command capture_game_view` 采的是**游戏自己渲染出来
#   的后缓冲**（`source=screen` 也只多含 Overlay 画布）。Unity 编辑器在 Game view 上
#   **叠加绘制**的东西（组件图标 = AudioSource 的喇叭 / Light 的太阳、DrawGizmos 线框、
#   选中高亮）**不在那个后缓冲里** ⇒ 既有取证路径全部采不到它们。
#   本脚本走 OS 层（PrintWindow / 桌面合成）采**窗口像素**，才和用户的眼睛同源。
#
# ────────────────────────────────────────────────────────────────────────────
# 与引擎 `Runtime/Core/Screenshot.cs` 的 `Screenshot.CaptureToFile` 的区别（各自适用场景）：
#   · Screenshot.CaptureToFile —— **帧末取像素**（Texture2D.ReadPixels 读屏幕后缓冲），
#     采的是**游戏渲染出来的画面**。适用：Play 模式帧末那一拍、需要"渲染结果本身"的
#     像素 diff / 基线对照；Editor 里没有协程可挂，直接调用即可（见该文件顶部取舍）。
#     ⛔ 采不到编辑器在 Game view 上的叠加层（组件图标 / Gizmos / 选中高亮）。
#   · 本脚本 —— **窗口级像素**（OS 层 PrintWindow / 桌面合成），采的是**用户真正看到的
#     画面**，含编辑器叠加层。适用：`表现类` 取证、复现"用户说看得见、截图却抓不到"。
#     ⛔ 代价：必须有**可见**窗口（最小化 / 屏外采不到）、受多显示器 / 遮挡影响、
#        拿不到比屏幕更高的分辨率。两者互补，⛔ 不要拿一个替代另一个。
# ────────────────────────────────────────────────────────────────────────────
#
# 用法：
#   powershell -NoProfile -ExecutionPolicy Bypass -File capture-editor-window.ps1 `
#       -Out <abs.png> -TitleMatch "Unity" [-TimeoutSec 15] [-Focus] [-TopMost] [-Desktop]
#   powershell -NoProfile -ExecutionPolicy Bypass -File capture-editor-window.ps1 `
#       -Out <abs.png> -EditorPid 11364 [-X 301 -Y 177 -W 1362 -H 553]
#
# 窗口选择：
#   -TitleMatch 窗口标题（**正则**，忽略大小写；写普通串即子串匹配）——按面积取最大者。
#   -EditorPid  只在该进程的可见顶层窗口里选（`unity status --format json` 的 data.instances[].pid）。
#   ⛔ 两者至少要给一个；都缺 → 用法错（不许"随便抓一个窗口"）。
#   ⚠️ Unity 的 Process.MainWindowHandle 会指向一个 160x28 的隐藏辅助窗口（实测 -32000,-32000）
#      ⇒ 本脚本自己枚举 pid 名下的可见顶层窗口，取面积最大的那个（= 编辑器主窗口）。
#
# 失败一律**明确报错 + exit 非 0**（⛔ 不许静默出一张黑图）：
#   2 = 用法错（参数非法 / 裁剪框越界 / -Out 不是 .png）
#   3 = 窗口找不到（含等待超时）/ 最小化且未 -Focus
#   4 = 采集退化：全黑或纯色（诊断图写到 <Out>.rejected.png，<Out> 不产出）
#   5 = 检测到多显示器且未 -AllowMultiMonitor（坐标口径会含糊）
#   6 = PrintWindow 自报失败（走 -Desktop 重试）
#
# ASCII 输出 + UTF-8 with BOM 文件（BOM 见文件头；PS 5.1 读无 BOM 文本按 ANSI 解析，
# 而本文件含中文注释 ⇒ 必须带 BOM）。
# ============================================================================
param(
  [Parameter(Mandatory=$true)][string]$Out,
  [string]$TitleMatch = '',
  [int]$EditorPid = 0,
  [int]$TimeoutSec = 15,
  [int]$X = -1,
  [int]$Y = -1,
  [int]$W = 0,
  [int]$H = 0,
  [switch]$Focus,
  [switch]$TopMost,
  [switch]$Desktop,
  [switch]$AllowMultiMonitor,
  [double]$MinMeanRGB = 2.0
)

$ErrorActionPreference = 'Stop'

function Fail([int]$code, [string]$msg) {
  # ⛔ 不用 Write-Error：$ErrorActionPreference='Stop' 下它是**终止错误**，后面那句 exit 永不执行，
  #    退出码会退化成 1（调用方就分不清"窗口没找到"和"用法错"）。这里直接写 stderr 再 exit。
  [Console]::Error.WriteLine("capture-editor-window: " + $msg)
  exit $code
}

function Write-Rejected([System.Drawing.Bitmap]$bmp, [string]$path, [string]$why) {
  # 退化帧的**诊断副本**：不是产物（名字带 .rejected），仅用于人看一眼"到底采成了什么"。
  try {
    $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    [Console]::Error.WriteLine("capture-editor-window: rejected frame kept for diagnosis -> " + $path)
  } catch { }
  Fail 4 ("capture failed: " + $why)
}

if ([string]::IsNullOrWhiteSpace($Out)) { Fail 2 '-Out is required' }
if ($Out -notmatch '(?i)\.png$') { Fail 2 ("-Out must end with .png (got: " + $Out + ")") }
if ([string]::IsNullOrWhiteSpace($TitleMatch) -and $EditorPid -le 0) {
  Fail 2 'need -TitleMatch and/or -EditorPid to select the window (refusing to grab "any" window)'
}
if ($TimeoutSec -lt 0) { Fail 2 '-TimeoutSec must be >= 0 (0 = single attempt)' }
$crop = ($X -ge 0 -and $Y -ge 0 -and $W -gt 0 -and $H -gt 0)
if (-not $crop -and ($X -ge 0 -or $Y -ge 0 -or $W -gt 0 -or $H -gt 0)) {
  Fail 2 '-X/-Y/-W/-H must be given together (all four) or not at all'
}

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
# 同会话里重复 dot-source 时 Add-Type 会因"类型已存在"报错 ⇒ 先探测再注册（幂等）。
if (-not ('CloverCap.Win32' -as [type])) {
Add-Type -Namespace CloverCap -Name Win32 -MemberDefinition @'
[DllImport("user32.dll")] public static extern bool PrintWindow(System.IntPtr hWnd, System.IntPtr hdcBlt, uint nFlags);
[DllImport("user32.dll")] public static extern bool GetWindowRect(System.IntPtr hWnd, out RECT lpRect);
[DllImport("user32.dll")] public static extern bool SetForegroundWindow(System.IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool IsWindowVisible(System.IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool ShowWindow(System.IntPtr hWnd, int nCmdShow);
[DllImport("user32.dll")] public static extern bool IsIconic(System.IntPtr hWnd);
[DllImport("user32.dll")] public static extern bool SetWindowPos(System.IntPtr hWnd, System.IntPtr after, int x, int y, int cx, int cy, uint flags);
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(System.IntPtr hWnd, out uint pid);
[DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc cb, System.IntPtr lp);
[DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowTextW(System.IntPtr hWnd, System.Text.StringBuilder s, int n);
public delegate bool EnumProc(System.IntPtr hWnd, System.IntPtr lp);
[StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left; public int Top; public int Right; public int Bottom; }
'@
}

# --- 多显示器：坐标口径会含糊（PrintWindow 尚可、CopyFromScreen / 裁剪框会歧义）⇒ 默认拒绝，
#     确认过口径的调用方再显式 -AllowMultiMonitor。
$screens = @([System.Windows.Forms.Screen]::AllScreens)
if ($screens.Count -gt 1 -and -not $AllowMultiMonitor) {
  $desc = ($screens | ForEach-Object { $_.DeviceName + '=' + $_.Bounds.ToString() }) -join ' | '
  Fail 5 ("multi-monitor detected (" + $screens.Count + "): " + $desc +
          " -- pass -AllowMultiMonitor only after confirming the capture coordinate口径")
}

# --- 选窗口：按 pid / 标题过滤的可见顶层窗口里取面积最大者。---
$script:bestH = [IntPtr]::Zero; $script:bestArea = [long]0; $script:bestRect = $null; $script:bestTitle = ''
function Get-BestWindow([int]$wantPid, [string]$titlePattern) {
  $script:bestH = [IntPtr]::Zero; $script:bestArea = [long]0; $script:bestRect = $null; $script:bestTitle = ''
  $rgx = $null
  if (-not [string]::IsNullOrWhiteSpace($titlePattern)) {
    try { $rgx = New-Object System.Text.RegularExpressions.Regex($titlePattern, 'IgnoreCase') }
    catch { Fail 2 ("-TitleMatch is not a valid regex: " + $titlePattern) }
  }
  $cb = [CloverCap.Win32+EnumProc]{
    param($h, $l)
    $wpid = 0
    [void][CloverCap.Win32]::GetWindowThreadProcessId($h, [ref]$wpid)
    if ($wantPid -gt 0 -and $wpid -ne $wantPid) { return $true }
    if (-not [CloverCap.Win32]::IsWindowVisible($h)) { return $true }
    $sbT = New-Object System.Text.StringBuilder 512
    [void][CloverCap.Win32]::GetWindowTextW($h, $sbT, 512)
    $title = $sbT.ToString()
    if ($title.Length -eq 0) { return $true }
    if ($script:rgxTitle -ne $null -and -not $script:rgxTitle.IsMatch($title)) { return $true }
    $r = New-Object CloverCap.Win32+RECT
    if ([CloverCap.Win32]::GetWindowRect($h, [ref]$r)) {
      $area = [long]($r.Right - $r.Left) * [long]($r.Bottom - $r.Top)
      if ($area -gt $script:bestArea) {
        $script:bestH = $h; $script:bestArea = $area; $script:bestRect = $r; $script:bestTitle = $title
      }
    }
    return $true
  }
  $script:rgxTitle = $rgx
  [void][CloverCap.Win32]::EnumWindows($cb, [IntPtr]::Zero)
  $script:rgxTitle = $null
  return @{ hwnd = $script:bestH; rect = $script:bestRect; title = $script:bestTitle }
}

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$found = $null
do {
  $found = Get-BestWindow $EditorPid $TitleMatch
  if ($found.hwnd -ne [IntPtr]::Zero) { break }
  if ((Get-Date) -ge $deadline) { break }
  Start-Sleep -Milliseconds 500
} while ($true)

if ($found.hwnd -eq [IntPtr]::Zero) {
  $sel = @()
  if ($EditorPid -gt 0) { $sel += ("pid=" + $EditorPid) }
  if (-not [string]::IsNullOrWhiteSpace($TitleMatch)) { $sel += ("title~/" + $TitleMatch + "/") }
  Fail 3 ("no visible top-level window matched (" + ($sel -join ', ') + ") within " + $TimeoutSec + "s")
}
$hwnd = $found.hwnd
Write-Host ("window hwnd=" + $hwnd + " title=" + $found.title)

if (-not $Focus) {
  # 最小化窗口的矩形恒为 -32000,-32000 160x28 ⇒ 采到的是屏外那片黑。这里**先报错**，
  # 而不是让调用方拿到一张黑图（要恢复就给 -Focus，或调用方自己先还原窗口）。
  if ([CloverCap.Win32]::IsIconic($hwnd)) {
    Fail 3 'matched window is minimized (IsIconic=true); pass -Focus to restore it, or restore it yourself'
  }
}
if ($Focus) {
  [void][CloverCap.Win32]::ShowWindow($hwnd, 9)   # SW_RESTORE
  [void][CloverCap.Win32]::SetForegroundWindow($hwnd)
  Start-Sleep -Milliseconds 600
}
# 实测：AI 宿主的 IDE 窗口常年 topmost，SetForegroundWindow 抢不过它 ⇒ 桌面合成会采到 IDE。
# SetWindowPos(HWND_TOPMOST) 不受前台锁限制，是唯一能把编辑器真正放到最上面的办法；
# 采完立刻恢复 NOTOPMOST（幂等、无残留）。
$hwndTM = -1
$hwndNTM = -2
if ($TopMost) {
  [void][CloverCap.Win32]::SetWindowPos($hwnd, [IntPtr]$hwndTM, 0, 0, 0, 0, 0x0043)  # NOMOVE|NOSIZE|SHOWWINDOW
  Start-Sleep -Milliseconds 900
}

$rect = New-Object CloverCap.Win32+RECT
if (-not [CloverCap.Win32]::GetWindowRect($hwnd, [ref]$rect)) { Fail 3 'GetWindowRect failed' }
$ww = $rect.Right - $rect.Left
$wh = $rect.Bottom - $rect.Top
if ($ww -le 0 -or $wh -le 0) { Fail 3 ("window rect is degenerate: " + $ww + "x" + $wh) }
Write-Host ("window rect=" + $rect.Left + "," + $rect.Top + " " + $ww + "x" + $wh)
if ($crop -and (($X + $W) -gt $ww -or ($Y + $H) -gt $wh)) {
  Fail 2 ("crop " + $X + "," + $Y + " " + $W + "x" + $H + " exceeds window " + $ww + "x" + $wh)
}

# --- 采集 ---
$bmp = New-Object System.Drawing.Bitmap($ww, $wh)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$mode = 'PrintWindow'
if ($Desktop) {
  # 桌面合成：拿到的就是合成器上真正显示的像素（含其它窗口遮挡）。
  $mode = 'Desktop(CopyFromScreen)'
  try { $g.CopyFromScreen($rect.Left, $rect.Top, 0, 0, (New-Object System.Drawing.Size($ww, $wh))) }
  catch { $g.Dispose(); $bmp.Dispose(); Fail 6 ("CopyFromScreen failed: " + $_.Exception.Message) }
  $g.Dispose()
} else {
  $hdc = $g.GetHdc()
  # 2 = PW_RENDERFULLCONTENT (DWM/GPU 渲染窗口也能采到内容；1 = PW_CLIENTONLY 会丢内容)
  $ok = [CloverCap.Win32]::PrintWindow($hwnd, $hdc, 2)
  $g.ReleaseHdc($hdc)
  $g.Dispose()
  Write-Host ("PrintWindow ok=" + $ok)
  if (-not $ok) {
    if ($TopMost) { [void][CloverCap.Win32]::SetWindowPos($hwnd, [IntPtr]$hwndNTM, 0, 0, 0, 0, 0x0043) }
    $bmp.Dispose()
    Fail 6 'PrintWindow returned false (retry with -Desktop for compositor pixels)'
  }
}

# --- 裁剪（窗口内相对坐标）---
if ($crop) {
  $cropBmp = New-Object System.Drawing.Bitmap($W, $H)
  $gc = [System.Drawing.Graphics]::FromImage($cropBmp)
  $gc.DrawImage($bmp, (New-Object System.Drawing.Rectangle(0, 0, $W, $H)),
                (New-Object System.Drawing.Rectangle($X, $Y, $W, $H)), [System.Drawing.GraphicsUnit]::Pixel)
  $gc.Dispose()
  $bmp.Dispose()
  $bmp = $cropBmp
}

# --- 退化自检（抽样，不是逐像素；全黑 / 纯色都算失败）---
$step = [Math]::Max(1, [int][Math]::Floor([Math]::Sqrt(($bmp.Width * $bmp.Height) / 20000.0)))
$seen = New-Object 'System.Collections.Generic.HashSet[int]'
[long]$sum = 0; [long]$n = 0
for ($yy = 0; $yy -lt $bmp.Height; $yy += $step) {
  for ($xx = 0; $xx -lt $bmp.Width; $xx += $step) {
    $c = $bmp.GetPixel($xx, $yy)
    [void]$seen.Add((([int]$c.R) -shl 16) -bor (([int]$c.G) -shl 8) -bor ([int]$c.B))
    $sum += [long]$c.R + [long]$c.G + [long]$c.B
    $n++
  }
}
$unique = $seen.Count
$mean = if ($n -gt 0) { [double]$sum / ([double]$n * 3.0) } else { 0.0 }
$stats = ("samples=" + $n + " uniqueColors=" + $unique + " meanRGB=" + $mean.ToString('F2') + " mode=" + $mode)
Write-Host ("stats " + $stats)

$dest = $Out
if (-not [System.IO.Path]::IsPathRooted($dest)) { $dest = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $dest)) }
$rej = $dest + '.rejected.png'
if ($unique -le 1) { Write-Rejected $bmp $rej ('degenerate frame: uniqueColors=' + $unique + ' (capture produced a solid image)') }
if ($mean -lt $MinMeanRGB) {
  Write-Rejected $bmp $rej ('degenerate frame: meanRGB=' + $mean.ToString('F2') + ' < -MinMeanRGB ' + $MinMeanRGB +
                            ' -- try -Desktop, or -Focus/-TopMost if the window is covered/minimized')
}

if ($TopMost) {
  [void][CloverCap.Win32]::SetWindowPos($hwnd, [IntPtr]$hwndNTM, 0, 0, 0, 0, 0x0043)
}

$dir = Split-Path -Parent $dest
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
try { $bmp.Save($dest, [System.Drawing.Imaging.ImageFormat]::Png) }
catch { $bmp.Dispose(); Fail 4 ("failed to write " + $dest + ": " + $_.Exception.Message) }
$bmp.Dispose()

$fi = Get-Item $dest
$sha = (Get-FileHash -Algorithm SHA256 -LiteralPath $dest).Hash.ToLower()
Write-Host ("saved " + $fi.FullName + " bytes=" + $fi.Length + " " + $stats)
Write-Host ("sha256 " + $sha)
exit 0
