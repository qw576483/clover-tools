# asset-audit —— 未引用素材盘点（STOCK：只报数，绝不删）

**规则**：进入工程的素材必须能从「游戏自己读得到的文本」（代码 / 配置 / 表 / 关卡文本）**到达**。
本工具把这件事做成一条命令，**不改任何文件**。

- 脚本：`Invoke-AssetAudit.ps1`（ASCII-only；含非 ASCII 才能上 BOM，本文件没有 ⇒ 保持无 BOM 即可）
- 出处：由 `clover-project-diablo2/tools/check-assets.ps1` 上浮并参数化（源版只做「文件名在源码里字面出现」一种形态，
  与 skill `reference/check-assets-template.md` 点名的失败模式相同 ⇒ 这里按该模板的**五形态**实现，并把所有路径/白名单变成参数）

## 用法

```powershell
# 只报数（盘点模式，退出码恒为 0）—— 存量工程先跑这条
powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-AssetAudit.ps1 -ProjectRoot <项目根> -Warn

# 闸门模式：有未覆盖素材即退出码 1
powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-AssetAudit.ps1 -ProjectRoot <项目根>

# 覆盖自动探测失败时（非常规工程布局）
powershell ... -ProjectRoot <根> -AssetsRoot <Unity 工程目录> -ResourceRoot <素材根> `
           -SourceRoots <代码目录1>,<代码目录2> -ExtraTextGlobs 'Resources/Levels/*.txt'
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `-ProjectRoot` | 必填 | 项目根（`client/` 或 `Assets/` 所在的那一层之上） |
| `-AssetsRoot` | 自动 | 依次探测 `<root>/client/Assets`、`<root>/Assets` |
| `-ResourceRoot` | `<AssetsRoot>/Resources` | 被盘点的素材根 |
| `-SourceRoots` | 自动 | `<AssetsRoot>` 下已存在的 `Scripts` / `Editor` / `Configs`（**只读不写**） |
| `-TextGlobs` | `*.cs,*.json,*.txt,*.prefab,*.asset` | 参考文本的文件名过滤 |
| `-ExtraTextGlobs` | `Resources/Levels/*.txt`、`StreamingAssets/*.txt` | 额外参考文本（相对 `-AssetsRoot`） |
| `-ManifestName` | `manifest.json` | 帧序列的权威帧数来源（`actions.<动作>.frames`） |
| `-FrameIndexRegex` | `_\d+$` | 帧序列文件名的帧号后缀 |
| `-FrameDigitsRegex` | `(\d+)$` | **帧号本身**的正则（必须带一个捕获组）；与上面分开，是因为「是不是帧序列」和「帧号是多少」是两件事（实测：从 `-FrameIndexRegex` 的捕获组取号，在该正则没写括号时会静默得到 0 ⇒ 整条帧序列被放行） |
| `-MinLeafLen` | `3` | 叶子目录名短于此值不参与「形态 2b」 |
| `-SkipNamePrefix` | `_` | 文件名以此前缀开头则跳过（Unity 约定：下划线开头不进游戏） |
| `-HighlightDirs` | 空 | 额外单独报数的顶层目录名（可重复），用来把某一族素材的未覆盖数拆出来看 |
| `-TopN` | `10` | 「未覆盖按目录」清单的条数 |
| `-OutFile` | 空 | 把整份报告落成文件（UTF-8 无 BOM） |
| `-Warn` | 关 | 盘点模式（退出码 0）；不传即闸门模式 |

退出码：`0` = 通过（或 `-Warn`）、`1` = 闸门模式且存在未覆盖素材、`2` = 参数/路径错误。

## 判据形态（一条命令 + 关键原始输出）

```text
===== asset-audit (STOCK: report only, never deletes) =====
project-root  = <...>
assets-root   = <...>
resource-root = <...>
mode          = warn (report only, exit 0)
ref-source files = 1432 ; text chars = 9123456
resource files = 10529 (size = 29.5 MB)
skipped (name starts with _) = 0
resources=10529 covered=9871 uncovered=658 unknown=0 uncovered-MB=2.1 total-MB=29.5
uncovered by directory (top 10):
  <目录>  <条数>
uncovered list (one line per file):
UNREF  <相对路径>
RESULT: resources=10529 covered=9871 uncovered=658 unknown=0 uncovered-MB=2.1 -> WARN (exit 0)
```

判定读法（硬性）：

- `uncovered=0` 且 `unknown=0` ⇒ 闸门模式退出码 0。
- `uncovered>0` 且集中在**大目录**（角色/怪物/UI）⇒ 先怀疑形态 ②③④ 没被识别（目录常量、帧序列拼名、目录级加载）
  ⇒ **去源码里找目录常量与帧数常量**，⛔ 不要照着这份清单删文件。
- `unknown>0` ⇒ 先把解析异常修掉再看结论：**解析失败不算未引用**，这些文件已按「保留」处理
  （当前只有一种情况会进 UNKNOWN：文件名匹配帧序列形态、但**读不出帧号**）。
- **直接躺在素材根目录下的文件**（相对目录为空）算正常布局，按未覆盖处理并打印文件名 —— 不是解析失败。
- ⛔ 本报告**不是删除清单**。要裁素材必须逐组人工核过形态 ②③④。

## 五形态（任一命中即「已引用」）

| # | 形态 | 判法 |
|---|---|---|
| ① | 精确文件名 | 去扩展名的文件名在参考文本里字面出现 |
| ② | 目录 / 前缀常量 | 文件所在目录（相对素材根）在参考文本里被具名 |
| ②b | 叶子目录名 | 叶子目录名（长度 ≥ `-MinLeafLen`）在参考文本里被具名 |
| ③ | 帧序列拼名 | `<前缀>_<帧号>`：帧号 < 同级 `-ManifestName` 里该动作的权威帧数 |
| ④ | 目录级加载 | 源码出现 `LoadAll<` / `Resources.LoadAll` **且该目录本身被具名**（单独一个全局标志不许放行整棵树） |
| ⑤ | 生成物登记 | 同级存在 `-ManifestName` 且该文件**不是**帧序列文件 |

## 与既有件的关系（避免重复上浮）

- `clover-ai-skill/reference/check-assets-template.md` 是**骨架/文档**（复制粘贴用），本目录是**可直接跑的参数化件**。
- `clover-ai-skill/scripts/resource-key-crosscheck.ps1` 的「B 方向：落在盘上却没人引用」覆盖**同类问题**，
  但它要求工程存在**资源 key 注册表**；本工具不需要注册表，判据是「从可读文本可达」。
  两者并存：有注册表就两边都跑（互补），没注册表只有本工具能跑。
