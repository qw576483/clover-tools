# visual-verify

**可视化验收的判定工具**：把"看起来像不像"换成**脚本算出来的数字 + 一份待办清单**。

配套规则：`clover-ai-skill/reference/visual-loop.md`（视觉闭环）、`reference/deterministic-gates.md`（判定交给脚本）、
`reference/verify-template.md`（`tools/verify.ps1` 模板）。

> **为什么是脚本而不是"看一遍"**：截图是**诊断工具**，不是每格的常规证据；判定必须可复算、可复查。
> AI 只读**汇总（联络图）**并做分诊，最终裁决仍然是人。

## 0. 装依赖

```bash
python -m pip install -r visual-verify/requirements.txt     # numpy / Pillow / fonttools
```

这里的脚本都是**纯命令行、无状态、可重复跑**：同样的输入永远得到同样的输出。

## 1. `visual-diff.py` —— 确定性像素 diff

```bash
python visual-verify/visual-diff.py <baseline.png> <ours.png> <out-dir> \
    --threshold 0.1 --max-diff-ratio 0.001 --min-ssim 0.99
```

写进 `<out-dir>`：

| 产物 | 用途 |
|---|---|
| `side-by-side.png` | 同一高度归一化后的**基线 \| 我们的**并排图（给人 / 多模态看） |
| `diff-overlay.png` | 把**差异像素涂红**的叠加图（哪一块不对，一眼定位） |
| `delta.txt` | **判定数字 + 待办清单**：差异比 + 分段带状分解（差异落在画面的哪一横条） |

**判据（三个数字同时成立才算 PASS）**：

| 检查 | 默认 | 说明 |
|---|---|---|
| `diff-ratio` | `<= 0.001` | 差异像素占全图比例（模板容差 `maxDiffRatio ≤ 0.1%`） |
| `diff-pixels` | `-1`（关） | 额外的**绝对像素数上限**；给了就多一道闸（`--max-diff-pixels 0` = 要求零差异） |
| `ssim-mean` | `>= 0.99` | 结构相似度（模板容差 `minSsim ≥ 0.99`）；`--no-ssim` 时打印 `SKIPPED` |

**算法**：逐像素 **YIQ 距离**（pixelmatch 的公式：`0.5053*dy² + 0.299*di² + 0.1957*dq²`），
超阈判据 `delta > 35215 * threshold²`（`--threshold 0.1` ⇒ 352.15，与 pixelmatch 一致）。
SSIM = 灰度（`0.299R+0.587G+0.114B`）上的 **8×8 均值滑窗**（积分图，逐像素精确），
`C1=(0.01*255)²`、`C2=(0.03*255)²`；**判定用 SSIM 均值**，最小块值一并打印供分诊。

**抗锯齿口径 `--aa-policy`**：

- `count`（默认）——**每个像素都算**。诚实默认：抗锯齿差异也是看得见的差异。**引用判定时用这个。**
- `suppress` —— 丢弃"看起来是抗锯齿"的差异像素（该像素在**我们**这一侧处在颜色边界上，且 8 邻域里至少有一个像素**两图完全相同**）。
  这是**有意的启发式**（对标 pixelmatch `includeAA:false` 的精神），**不是它的逐位复刻**，只用于分诊。

**退出码**：`0` PASS / `1` FAIL（超容差）/ `2` 用法错 / `3` 依赖缺失。

## 2. `shot-stats.py` —— 截图的确定性统计

```bash
python visual-verify/shot-stats.py --shots .ai-tmp/screenshots \
    --color "accent=#1c8414:60:0.15:0.80" --out .ai-tmp/test/shot-stats.txt
```

逐张输出：**唯一色数 / 主色 / 主色占比 / 暗像素占比**；`--color` 再逐张量**命名色的像素数与列区间游程**。

| 信号 | 抓什么 |
|---|---|
| `unique_colors` ≈ 1、`modal_share` ≈ 1（标 `<-- DEGENERATE FRAME`） | **退化帧**：渲染没出来 / 画面是纯色填充 |
| `dark_share`（`R+G+B < --dark-threshold`） | **黑遮罩**：该画上去的面板（暂停 / 结算）其实没画 |
| `--color NAME=#RRGGBB[:TOL[:Y0:Y1]]` 的 `pixels` / `span` / `runs` | 某个**必须可见的东西**在不在、在画面的哪一列区间（`Y0..Y1` 把 HUD 条排除在外） |

`--shots` 接受目录、文件或 glob（逗号/分号分隔，可重复）。**读到就打 `READ FAILED` 并最终 `exit 1`**，不静默跳过。

## 3. `font-metrics.py` —— 从字体文件反推字形量法

```bash
python visual-verify/font-metrics.py --font "candidate=C:/path/font.ttf" \
    --text "by clover-engine" --size 16
```

逐字符给出字形的 `yMin / yMax / height`（px，相对基线），并算：

- **短游程数**：`height <= hMax - --min-run-deficit` 的字形个数 —— `>= --min-short-runs` ⇒ **这个字体真有"小写"**；
- **基线下沉**：`yMin < 0` 的字形（真正的降部）；
- **基线众数**：大多数字形落在哪条 `yMin` 上。

**它是屏幕上那条判据的负控**：只有大写的像素字体没有短游程 ⇒ 判据必须为 False；
若换成"看起来有大小写"的字符串它还是 True，说明这条判据在测空气。
（出处同 `clover-ai-skill` 铁律 3：**写不出出处的量不许进工程** —— 字体这一层要量字体文件本身，不是量源码字符串。）

## 4. `capture-editor-window.ps1` —— 窗口级截屏（用户真正看到的画面）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File capture-editor-window.ps1 `
    -Out <abs.png> -TitleMatch "Unity" -TimeoutSec 15 [-Focus] [-TopMost] [-Desktop] `
    [-X 301 -Y 177 -W 1362 -H 553]
```

OS 层（`PrintWindow` / 桌面合成）采**窗口像素** ⇒ 含编辑器在 Game view 上**叠加绘制**的
组件图标 / Gizmos / 选中高亮 —— 那些**不在**游戏后缓冲里，所以 `Screenshot.CaptureToFile`
（帧末 `ReadPixels`）采不到。两者互补，⛔ 不互相替代（区别写在两个文件头注释里）。
编辑器若以**管理员**身份启动（UIPI 挡住外部置顶），改用引擎
`Editor/capture-editor-screen.cs` 的 `run_script --entry CloverEngine.Editor.EditorScreenCapture.Shot`。

| 参数 | 默认 | 说明 |
|---|---|---|
| `-Out` | 必填 | 目标 `.png`（非 `.png` 直接拒绝） |
| `-TitleMatch` | — | 窗口标题**正则**（忽略大小写），按面积取最大匹配 |
| `-EditorPid` | 0 | 限定进程；与 `-TitleMatch` **至少给一个** |
| `-TimeoutSec` | 15 | 等窗口出现（0 = 单次尝试） |
| `-X/-Y/-W/-H` | — | 窗口内相对矩形裁剪（四个一起给） |
| `-Focus` / `-TopMost` / `-Desktop` | off | 还原 / 置顶 / 改走桌面合成 |
| `-AllowMultiMonitor` | off | 多显示器**默认拒绝**（坐标口径含糊） |
| `-MinMeanRGB` | 2.0 | 退化阈值 |

**判据（失败一律 exit 非 0，⛔ 不许静默出一张黑图）**：窗口找不到 / 超时 / 最小化未 `-Focus` = `3`；
多显示器 = `5`；`PrintWindow` 自报失败 = `6`；**全黑或纯色**（`uniqueColors<=1` 或
`meanRGB < -MinMeanRGB`）= `4`，此时 `<Out>` **不产出**（诊断副本写 `<Out>.rejected.png`）；
参数 / 裁剪框越界 = `2`。成功时打印 `hwnd / rect / uniqueColors / meanRGB / bytes / sha256`。

## 5. `compose-contact-sheet.py` —— 把 N 个证据点压进 1 张联络图

```bash
python compose-contact-sheet.py --shots ".ai-tmp/screenshots/a.png,b.png" \
    --labels "01=菜单|表现类|标题 y=64;02=创角|表现类|3 个输入框" \
    --cols 2 --title "build=abc123" --out .ai-tmp/test/sheet.png \
    --index .ai-tmp/test/sheet.index.tsv
```

每格左上角烧**格号**，格下烧**状态名 + 类别 + 关键数值**；同时输出 `index.tsv`：
`格号 / 截图来源 / 状态 / 类别 / 关键数值`（AI 只读这张汇总图，格号可查回具体取证文件）。

| 参数 | 默认 | 说明 |
|---|---|---|
| `--shots` | 必填 | 目录 / glob / 单个文件；逗号或分号分隔，可重复（内部 `sorted()` 定序） |
| `--cols` | 4 | 列数 |
| `--labels` | — | 分号分隔的 `[格号=]状态名[\|类别\|关键数值]` |
| `--out` | 必填 | 联络图 PNG |
| `--index` | `<out 去扩展名>.index.tsv` | 索引表 |
| `--title` | — | 顶部标题（**采集时间 / 构建标识由调用方传入**，工具不自动烧时间戳） |
| `--cell-size` | 480x320 | 每格图像框 |

**判据**：缺任一张图 = `exit 4` 且**一个字节都不写**（⛔ 不半发布）；标签数量不匹配 = `2`；
依赖缺失 = `3`。坐标 / 尺寸 / 居中全由参数算出、缩略图固定 `LANCZOS` ⇒
**同输入两次跑，图与索引的 SHA256 相同**（脚本打印这两个 SHA，便于当场复核）。

**两条实测坑（已在本脚本内修掉，写同类脚本请照抄）**：① `--shots <目录>` 与 `--out` 同目录时，
**输出文件不算输入**（否则第二遍把第一遍的图当输入 ⇒ 格数变、SHA 变）；② Python 的 `stdout`
在**重定向**下编码是 GBK ⇒ 路径里的**非 GBK 字符**会让 `print` 在**文件已写好之后**抛
`UnicodeEncodeError` 并 `exit 1`（假红），中文则成乱码；本脚本显式
`sys.stdout.reconfigure(encoding="utf-8", errors="backslashreplace")`。

## 6. `editor-overlay-probe.py` —— 编辑器叠加层量法（对齐线 / 网格 / 包围盒 / Gizmo）

```bash
# ① 采像素：编辑器在 Game view 上叠加绘制的对齐线 / Gizmos / 选中高亮**不在**后缓冲里
powershell -NoProfile -ExecutionPolicy Bypass -File visual-verify/capture-editor-window.ps1 `
    -Out .ai-tmp/screenshots/editor-window.png -TitleMatch "Unity" -TimeoutSec 15
# ② 把那一层变成数字 + 掩模 + 标注图
python visual-verify/editor-overlay-probe.py .ai-tmp/screenshots/editor-window.png `
    --out .ai-tmp/test/overlay.txt --mask .ai-tmp/test/overlay.mask.png `
    --annotate .ai-tmp/test/overlay.annotated.png --color "guide=#3cdb6a:20" --min-primitives 3
```

**分工**：`capture-editor-window.ps1` 负责采**用户真正看到的窗口像素**（含编辑器叠加层），本件负责把那一层
变成**可比的数字** —— 同一条对齐线两次跑必须给同一个 `x=`，⛔ 不是"看着对"。
与 `shot-stats.py` 的区别：后者回答"某个颜色在不在 / 在哪几列"，本件回答"**直线原始图元**在哪、多厚、
算 `line` 还是 `band`"。与 `visual-diff.py` 的区别：本件不做基线对照，它是**量法**（量完再喂给别的判据）。

写进：`--mask`（白 = 叠加层，可与上次的掩模直接 diff）/ `--annotate`（原图压暗 + 横线红 / 竖线蓝 / 颜色探针框黄）。
"叠加层" = 与**众数背景色**差超过 `--bg-tolerance` 的像素；也可用 `--color NAME=#RRGGBB[:TOL]` 只挑一个已知叠加色。

| 参数 | 默认 | 说明 |
|---|---|---|
| `shot`（位置参） | 必填 | 窗口截图 `.png`（一般来自 `capture-editor-window.ps1`） |
| `--out` | stdout | 报告落盘位置 |
| `--mask` / `--annotate` | — | 掩模图 / 标注图（都可省） |
| `--color SPEC` | — | `NAME=#RRGGBB[:TOL]`，与 `shot-stats.py --color` 同语法（可重复） |
| `--bg` / `--bg-tolerance` | 众数色 / `30` | 背景口径（`sum|dR|+|dG|+|dB|`） |
| `--min-line-span` | `24` | 连续游程多长才算一个图元 |
| `--max-line-thickness` | `6` | 比这厚就不算 `line` 而报 `band`（⛔ 大色块不会被当成对齐线） |
| `--max-lines` | `20` | 每个方向最多打印多少条 |
| `--min-primitives` | `0` | 判据要"必须看得见 N 条线"时给 N（不够 ⇒ `exit 1`） |

**判据**：`primitives`（= `lines_h + lines_v`）、`bands_h / bands_v`、`ink_pixels`、`ink_bbox`，
每个颜色探针再给 `pixels / bbox / primitives`；`--json` 出机读版。
**退出码**：`0` 报告完毕（且 `primitives >= --min-primitives`）/ `1` 图元不够 / `2` 用法错 / `3` 依赖缺失。
⚠️ 只吃**可见窗口**的像素：最小化 / 屏外 / 被遮挡时先 `-Focus` 或改走引擎 `Editor` 截屏 —— 与
`capture-editor-window.ps1` 的限制同源（本件不改那个脚本，只做它的判定端）。

## 7. `ab-region-diff.py` —— A/B 差异：**点名区域**的 diff

```bash
python visual-verify/ab-region-diff.py --a .ai-tmp/screenshots/before.png `
    --b .ai-tmp/screenshots/after.png --out .ai-tmp/test/ab --grid 4x6 `
    --max-region-ratio 0.005 --json .ai-tmp/test/ab.json
```

`visual-diff.py` 回答"**我们**这张和**基线**像不像（整帧 PASS/FAIL）"；本件回答两边**都是我们自己的图**时
（改 UI 前后 / 动画第 N 与 N+1 帧 / 1080p 与 720p 两次采集）"**哪一块**变了、变了多少" ——
把两图切成 `R{行}C{列}` 命名的网格，逐格给判定，出的是**可复现的判定输出**，不是只出一张图。

写进 `<out>`：`ab-diff.png`（差异像素涂红 + 网格线）/ `ab-regions.png`（压暗底图 + **超标格涂红**）/
`ab-report.txt`（判定 + 逐格明细）/ `ab-regions.tsv`（逐格机读表：`region,x0,y0,x1,y1,diff_pixels,region_pixels,ratio,verdict`）。

| 参数 | 默认 | 说明 |
|---|---|---|
| `--a` / `--b` | 必填 | A / B 两张图；B 是标注画布 |
| `--out` | 必填 | 产物目录 |
| `--grid RxC` | `4x6` | 网格（格号 `R{行}C{列}`，从 1 起） |
| `--threshold` | `0.1` | 逐像素 YIQ 阈值，**与 `visual-diff.py` 同尺度**（cut = `35215*threshold²`） |
| `--max-total-ratio` | `0.001` | 整帧差异像素占比上限 |
| `--max-region-ratio` | `0.005` | **单格**差异占比上限（超了就点名） |
| `--max-diff-pixels` | `-1`（关） | 绝对像素上限 |
| `--align resize\|none` | `resize` | 尺寸不同时：`resize` 把 A 缩到 B（LANCZOS，报告里打印 `scaleX/scaleY`）；`none` 报用法错 |
| `--aa-edge-band N` | `0` | 丢掉 B 中**边缘 N px 内**的差异像素（抗锯齿容差；启发式，口径同 `visual-diff.py --aa-policy suppress`） |
| `--edge-threshold` | `60` | 什么算"边缘"（邻像素 RGB 距离差之和） |

格区间一律是**半开**写法 `x=[100..200)`（= x0 含、x1 不含；TSV 的 `x0/x1` 同口径），
`bbox=` 才是**闭区间**的 `x0,y0-x1,y1`（差异像素的实际外框）—— 两者别混读。

**口径必须随报告走**：每次运行都把 `threshold / grid / 尺寸与缩放比 / aa_edge_band` 打在报告头几行，⛔ 不靠记忆、
不靠默认值；写进验收表的判定一律引用 `--aa-edge-band 0` 的那一次（抑制那次只用于分诊）。
**退出码**：`0` PASS / `1` FAIL（有格超标或整帧超标；报告里点名格号）/ `2` 用法错 / `3` 依赖缺失。
**确定性**：同输入两次跑，`ab-report.txt` / `ab-regions.tsv` / `ab-diff.png` 的 SHA256 相同。

## 8. `ui-bbox.py` —— 量控件 bbox（半通用量法）

```bash
# 颜色探针：这个按钮实际占多大，并与"应该多大"对判
python visual-verify/ui-bbox.py .ai-tmp/screenshots/menu.png --mode color `
    --target "ok=#1c8414:10" --expect "ok=240,120,160,48:2" --boxed .ai-tmp/test/ok.boxed.png
# 墨迹模式：控件是渐变 / 图标 / 文字（没有单一色）—— 量裁剪区里的"非背景"
python visual-verify/ui-bbox.py .ai-tmp/screenshots/menu.png --mode ink `
    --crop 380,280,160,100 --expect "ink=400,300,120,60:0"
```

这是"按钮 160×48 @ (240,120)"这类**声明**后面的量法：给 `pixels / bbox / w / h / center / fill`，
再用 `--expect NAME=x,y,w,h[:TOL]` 与布局该产生的框**对判**（超容差 ⇒ `MISMATCH`；一个像素都没匹配 ⇒ `MISSING`）。
三件半通用量法的分工：`font-metrics.py` 量**字体文件**、`shot-stats.py` 量**颜色在不在**、本件量**控件的框**。

| 参数 | 默认 | 说明 |
|---|---|---|
| `shot`（位置参） | 必填 | 截图 |
| `--mode color\|ink` | `color` | 颜色探针 / 裁剪区墨迹 |
| `--target SPEC` | — | color 模式：`NAME=#RRGGBB[:TOL[:Y0:Y1]]`（与 `shot-stats.py --color` 同语法，可重复） |
| `--crop x,y,w,h` | 全图 | ink 模式的测量区 |
| `--bg` / `--bg-tolerance` | 裁剪区众数色 / `30` | ink 模式的背景口径 |
| `--name` | `ink` | ink 模式的目标名（`--expect` 要用同名） |
| `--expect SPEC` | — | `NAME=x,y,w,h[:TOL]`；`TOL` 缺省取 `--expect-tolerance` |
| `--expect-tolerance` | `1` | 默认容差（px） |
| `--boxed` | — | 把量到的框画在图上（绿 = 通过 / 红 = 不符 / 黄 = 期望框） |

**判据**：`OK` / `MISMATCH` / `MISSING` 三种判定 + `delta(dx,dy,dw,dh)`；`--json` 出机读版。
**退出码**：`0` 全部 `--expect` 命中（或没给 `--expect`，只报告）/ `1` 有 `MISSING` 或 `MISMATCH` /
`2` 用法错 / `3` 依赖缺失。

## 9. 一条典型链路

```bash
# ① 渲染 + 截图（由项目自己的驱动脚本完成，落到 .ai-tmp/screenshots/）
# ①b 表现类 / 编辑器叠加层（组件图标 / Gizmos）：窗口级采一次
powershell -NoProfile -ExecutionPolicy Bypass -File visual-verify/capture-editor-window.ps1 `
    -Out .ai-tmp/screenshots/editor-window.png -TitleMatch "Unity" -TimeoutSec 15
# ② 先算数字，再决定要不要看图
python visual-verify/shot-stats.py --shots .ai-tmp/screenshots --out .ai-tmp/test/shot-stats.txt
python visual-verify/visual-diff.py 策划/基线图/title.png .ai-tmp/screenshots/title.png .ai-tmp/test/diff-title
# ③ 只有 FAIL 的格子才去看那一格的原始大图 / 叠加图
# ③b 改 UI / 换分辨率之后：两组我们自己的图比差异，让它点名区域
python visual-verify/ab-region-diff.py --a .ai-tmp/screenshots/before.png `
    --b .ai-tmp/screenshots/after.png --out .ai-tmp/test/ab
# ③c 量一个控件的框，并与布局该产生的框对判
python visual-verify/ui-bbox.py .ai-tmp/screenshots/after.png --mode color `
    --target "ok=#1c8414:10" --expect "ok=240,120,160,48:1"
# ④ 交付前把各格拼成一张联络图 + 索引表（AI 只读这一张）
python visual-verify/compose-contact-sheet.py --shots .ai-tmp/screenshots --cols 4 `
    --labels "01=菜单|表现类|标题 y=64" --out .ai-tmp/test/sheet.png
```

⛔ **不要把 diff / 拼版塞进"每改一处就跑一遍"的循环**：渲染 + diff 是分钟级动作
（见 `visual-loop.md` 第五节）—— **批量改完再跑一次**，拿 `delta.txt` 批量修，再跑下一轮。

## 10. 相关

- 判定口径与容差：`clover-ai-skill/reference/visual-loop.md` 第三节
- 联络图（N 个证据点压进 1 张图）：同文件第八节
- 交付前的机械自检：`clover-ai-skill/reference/verify-template.md`
