# 素材来源与自动获取（**能下就下，别卡在许可上**）

> **目标：素材自己找、自己下、自己接，不劳烦用户。**
>
> **本项目非商业用途**（练手 / 能力验证 / 内部演示）⇒ **不设许可门槛**：
> 只要你能下到、能导入、效果合适，**就下、就用** —— 不必逐条核对许可条款，不必为"是不是可商用"停下来问用户，
> 更不必因为"来源不明/论坛转载/社区提取"而放弃一个合适的素材。
>
> **唯一真正的门槛是"下不动"**：需要登录 / 需要会员 / 需要付费 / 需要点按钮的站点，才走 §5 让用户下
> （不是法律原因，是 **AI 没法登录、没法付款**）。
>将来若真要商用：再按 §6 的"替换清单"统一换素材/补授权即可 —— 所以现在**只要记一行来源**就够了，
> 不要为了"以后可能商用"提前牺牲效果和进度。
>⚠️ **URL 纪律**：本文件里的地址是**实测可用**的入口；**具体文件路径一律现场取**
> （GitHub 走 git/raw、Poly Haven 走官方 API），**不许凭记忆编造链接**。

---

## 1. 下载方式速查（AI 能自动做的）

| 方式 | 适用 | 命令范式 |
|---|---|---|
| **git clone（首选）** | GitHub/Gitee 上的资源仓库（FBX/GLB/PNG 齐全） | `git clone --depth 1 <repo-url> <dir>`（`--depth 1` 省时间省流量） |
| **raw 直链** | 单文件（`.glb`/`.png`/`.gltf`） | `Invoke-WebRequest <raw.githubusercontent.com/...> -OutFile <path>` |
| **官方 API** | Poly Haven 等有 API 的站点（**能拿到真实文件 URL，不用猜**） | `Invoke-RestMethod https://api.polyhaven.com/files/<id>` → 取 `blend/gltf/jpg` 里的 URL |
| **页面找直链** | 素材站的资源页 | 抓页面 → 找 `.zip/.fbx/.glb/.png` 直链或 CDN 链接 → 直接下（**能下就下**） |
| **让用户下（兜底）** | **需要登录 / 会员 / 付费 / 点按钮**的站点 | 给清单：包名 + 页面链接 + 要下的文件 + 放到哪个目录（见 §5 模板） |

**网络（国内）**：GitHub 直连慢或断就**换源**（换镜像 / 换 Gitee 同名仓库 / 换另一个素材站），
**不要在一个下不动的链接上耗时间** —— 换一个能下的等价素材，比等下载快得多。

**导入范式**（示例：KayKit 全套 → Unity 工程）：

```powershell
# ① clone 到工作区外（不要直接 clone 进 Assets，避免把 .git/源文件带进工程）
$tmp='<工作区>\_assets_tmp'; New-Item -ItemType Directory -Force -Path $tmp | Out-Null
git clone --depth 1 --quiet https://github.com/KayKit-Game-Assets/KayKit-City-Builder-Bits-1.0.git "$tmp\CityBits"

# ② 只把引擎要用的格式拷进工程（FBX 原生、GLB 自带内嵌动画、贴图 PNG），排除 .git / 冗余格式
robocopy "$tmp\CityBits" "<client>\Assets\ThirdParty\KayKit\CityBits" /S /IF *.fbx *.png *.glb /XD .git /NJH /NJS /NP /NDL

# ③ 运行时要用到的（角色模型 / 动画 controller / HUD）必须放 Resources 下
#    Resources/Models/   角色 FBX（Game.Res.LoadAsset<GameObject>("Models/Knight")）
#    Resources/Anim/     AnimatorController（Editor 生成）
#    Resources/UI/       面板预制体（Game.UI.Open<T>() 按类名查这里）
```

---

## 2. 实测可用的来源（按优先级）

### 2.1 「目标游戏」自己的素材（用户点名了游戏 A 时，**先找它**）

| 途径 | 说明 |
|---|---|
| **同名/同源素材包** | 搜 `<A 游戏名> assets pack / game assets / sprite pack / model pack`（GitHub / itch.io / 素材站） |
| **官方 SDK / Mod 工具资源** | 如 `<A> SDK assets`、`<A> mod tools models`（很多游戏官方放出建模/贴图供 mod） |
| **复刻/开源实现** | 搜 `<A> clone / remake / open source`，常带复刻素材 |
| **社区提取资源** | 搜 `<A> ripped assets / extracted sprites / sounds` —— **本项目非商用 ⇒ 直接用**（记一行来源即可） |

### ⛔ 硬闸门：级 ①「A 自己的素材」**穷尽之前，禁止碰 §2.2 的通用兜底素材**

**"穷尽"的量化定义（达不到就不算找过）**：**≥ 4 轮**不同关键词（中英各半）×
**≥ 3 类站点** × 换过**格式 / 打包方式**（sprite sheet / 单帧 PNG / 模型 FBX·GLB / 贴图 / 音效包 / MPQ·CASC 解包产物）。

**每一轮都要落盘 `策划/素材调研.md`**（关键词 / 站点 / 拿到什么 / 为什么不可用）。
**没有这份文件 = 素材闸门未通过，不许开始接资源。**

⛔ **不许用"先做起来 / 别停工 / 能用就用，否则会停工"当跳级理由** ——
用户对这条的原话是：**「skill 不是让你先找相关的素材吗？你怎么默认使用通用素材了！」**

**A 的素材需要"用户本机游戏文件 / 登录 / 会员 / 付费 / 点按钮"时 → 走 §5 让用户提供并等用户，不许自己跳过 ①。**
典型就是 **Blizzard 系（`.mpq` / CASC）**：解包工具链（MPQEditor / CascView / D2RMM / bin2txt）是公开的、到处能下，
**但游戏本体文件只有用户有** ⇒ 必须让用户提供本体或已解包的素材（清单模板见 **§5.1**）。

**只有 ①② 都真的做不到时**，才允许用 §2.2 兜底素材，且必须同时做三件事：
① `策划/素材调研.md` 写清"①② 为什么都拿不到"；② **交付说明首行**高亮「⚠️ 未使用 A 的素材（原因：…）」；
③ `client/资源欠缺清单.md` 登记"待换成 A 的素材"。

> 判定顺序：**A 的素材（穷尽）→ 同类游戏成套素材 → 我们的兜底素材（§2.2）→ 让用户下**。
> 找不到就**多找几轮**（换关键词、换站点、换语言、换镜像、换格式），不要一次没搜到就跳到"用别的游戏素材凑合"。

### 2.2 我们的兜底素材（实测可 `git clone`，**直接自动下载使用**）

**KayKit（Kay Lousberg）· 官方 GitHub · 免登录直连**
组织：`https://github.com/KayKit-Game-Assets`（共 10 个仓库，已确认存在 6 个）：

| 仓库名 | 内容 |
|---|---|
| `KayKit-City-Builder-Bits-1.0` | 城市 40 模型（地砖/道路/建筑/车辆/街道设施） |
| `KayKit-Character-Pack-Adventures-1.0` | 5 个**带动画**角色（Knight/Mage/Rogue/RogueHooded/Barbarian）+ 22 武器，每角色 76 clip |
| `KayKit-Character-Pack-Skeletons-1.0` | 4 个骷髅角色（每角色 95 clip） |
| `KayKit-Dungeon-Remastered-1.0` | 地牢 200+ 模型 |
| `KayKit-Medieval-Hexagon-Pack-1.0` | 中世纪六边形地块 200+ |
| `KayKit-Prototype-Bits-1.0` | 原型白盒块 |

> 角色自带动画名（实测）：`Idle` / `Running_A` / `Walking_A` / `1H_Melee_Attack_Slice_Diagonal` /
> `Hit_A` / `Death_A`（接入方式见 `reference/unity-cli.md` §8.1 与项目 `AnimSetup.cs`）

**其他可直链源**（**「能否自动下」比「许可」更重要**，故按获取难度列）：

| 源 | 入口 | 能自动下吗 | 内容 |
|---|---|---|---|
| Poly Haven | `https://api.polyhaven.com/files/<id>` | ✅ 有 API，可取真实 URL | HDRI / PBR 贴图 / 少量模型 |
| Khronos glTF-Sample-Assets | `https://github.com/KhronosGroup/glTF-Sample-Assets` | ✅ git clone / raw | 完整场景（Sponza/Bistro…）、测试模型 |
| ambientCG | `https://ambientcg.com` | ⚠️ 本机 TLS 受限时让用户下 | PBR 贴图 |
| Kenney | `https://kenney.nl/assets/<slug>` | ⚠️ 下载链接带随机哈希 ⇒ 抓页面找直链，抓不到就让用户下 | 2D/3D/音频成套（40+ 套） |
| Quaternius | `https://quaternius.com` | ⚠️ 需点按钮 ⇒ 页面上找直链 | 低模成套 |
| OpenGameArt | `https://opengameart.org` | ⚠️ 逐件，多为直链 | 2D/音效/音乐 |
| Freesound | `https://freesound.org` | ⚠️ 需账号/API key | 音效 |
| `game-icons.net` | — | ✅ 多为直链 | 图标 |
| Google Fonts | — | ✅ 直链 | UI 字体 |
| Unity Asset Store | 编辑器内 | ❌ 需在 Editor 里点（**免费的也让用户点一下**） | Synty POLYGON Starter（免费）等 |

**国内素材站**（爱给网、CG 模型网、微元素、Unity 中国资源商店等）：**先搜、先试着直连**，
能下就下；要登录/会员的就进 §5 清单 —— **不要因为"要会员"就跳过搜索**。

### 2.3 「多模态自产」只在前两级都失败时用（见 `SKILL.md` 多模态闸门）

---

## 3. 自动下载 + 自动接入的完整范式

```text
① 找：web_search 目标游戏素材 → 命中 GitHub / Gitee / raw 直链 / 官方 API / 页面直链
      （一轮没找到就换关键词、换站点、换语言、换镜像，多找几轮）
② 下：git clone --depth 1 / Invoke-WebRequest（能自动就自动，下不动才进 §5）
③ 验：确认拿到的是真模型（有 .fbx/.glb + 贴图），不是空目录/假包
④ 接：
   - 场景/道具 → Assets/ThirdParty/<来源>/…，由 Editor 脚本铺场景
   - 运行时角色 → Assets/Resources/Models/（FBX）
   - 动画 → Editor 生成 AnimatorController → Assets/Resources/Anim/
   - 贴图 → 派生材质 → Assets/Materials/
⑤ 量测后使用：**不许假定单位/枢轴**——实例化后实测 Renderer 包围盒反算缩放并贴地
   （各包单位不一致是常态，实测踩过：不量测就会 100 倍大或半埋地里）
⑥ 记一行：包名 / 来源 URL / 落地路径 → 《资源欠缺清单》（**够用了，不必展开许可分析**）
```

**贴图/模型规模控制**：只拷"用得上的格式"（FBX 原生 / GLB 自带动画 / PNG 贴图），
排除 `.obj/.mtl/.blend/.git`，避免工程体积与导入时间爆炸。

## 4. 许可：本项目**不设门槛**

| 情形 | 动作 |
|---|---|
| 免费可下（任何许可：CC0 / CC-BY / NC / 仅个人 / 来源不明） | **直接用**，顺手记一行来源 |
| 社区提取 / 游戏内资源 / 论坛转载 | **直接用**（非商业项目），记一行来源即可 |
| 需要登录 / 会员 / 付费 | 走 §5 让用户下（AI 不能登录/付款，与许可无关） |

> **不要**为了许可去问用户、不要写 `CREDITS.md` 当交付门槛、不要因为"来源不明"放弃合适的素材。
> **以后真要商用**：照《资源欠缺清单》里记的来源，统一替换或补授权即可（那时再谈合规）。
>
> ⛔ **"许可不设门槛"绝不等于"可以不找 A 的素材"**：取消许可门槛是为了让你**更大胆地用 A 的 rip /
> 社区提取 / 论坛转存素材**，**不是**让你跳到通用素材包。**这两件事完全无关，不许混为一谈。**

## 5. 让用户下载时的清单模板（**只在"登录/会员/付费/必须点按钮"时用**）

```markdown
## 需要你下载的素材（我这边下不了：需要登录/会员/付费/点按钮）

| # | 包名 | 页面链接 | 要下的文件 | 放到哪 |
|---|---|---|---|---|
| 1 | Synty POLYGON - Starter Pack | https://assetstore.unity.com/packages/... | 整套（FBX + 贴图） | Unity → Package Manager 导入 |
| 2 | Kenney City Kit | https://kenney.nl/assets/city-kit-suburban | `kenney_city-kit-suburban.zip` | 解压后把 `Models/*.fbx` 放 `<client>/Assets/ThirdParty/Kenney/CityKit/` |

**放好后告诉我一声，我继续接场景/动画/UI**（或你直接说"下好了"，我会重新扫目录自动接入）。
```

### 5.1 A 类模板：**参考游戏本体 / 已解包素材**（Blizzard 系等"解包工具公开、本体只有用户有"的游戏）

**这一级存在的意义**：让用户把 A 的本体文件给出来（或他自己解包后丢进目录），
AI 负责**解包工具链的获取与运行**、素材的整理与接入 —— **不许因为"AI 拿不到本体"就降级去用通用素材**。

```markdown
## 需要你提供的素材（A 自己的素材；我这边拿不到 A 的本体文件）

我需要的**是你电脑上那款游戏的原始文件**（我不可能拿到它），拿到后解包/整理/接入全归我做。

| # | 要什么 | 为什么需要 | 从哪来 | 放到哪 |
|---|---|---|---|---|
| 1 | A 的游戏本体目录（或其中的数据包文件，如 `*.mpq` / `Data/data/*`） | 里面有 A 的全部贴图 / 模型 / 音效 / 图集 | 你自己的正版安装目录 | `<项目根>/原版资源/安装包/`（原样拷进去即可，不用改名） |
| 2 | （可选）已解包出的素材目录 | 若你已用 MPQEditor / CascView 等解包过，直接给我解包结果更快 | 你的解包输出目录 | `<项目根>/原版资源/解包原始/` |

**只放这一个目录就行，其余全归我做**：我会自己去找解包/转换工具、按格式解析、切图、建材质、接进工程。
**解包工具不需要你装**（我自己取）。
```

> ### ⛔ 素材原则（**已修正**）：级 ① 没穷尽之前，**不许**"先用手上最好的素材做起来"
>
> 那句话正是被用户点名的偷懒路径。**拿不到 A 的本体 → 等用户（§5.1），不要自己跳过 ①。**
> **只有当 ①② 都真的做不到**（`策划/素材调研.md` 有据可查）时，才允许用 §2.2 兜底料，
> 并在**交付说明首行高亮**「⚠️ 未使用 A 的素材（原因：…）」。
> 无论用哪一级，**资源引用必须收敛在少数几处** —— 将来换素材 = 换文件、不动逻辑。

---

## 6. 从别人的 Unity 工程里**精确解出**素材（不要肉眼认图）

要还原某个游戏的关卡 / 动画 / 预制体用图时，**权威来源是工程文件本身**：

| 要什么 | 从哪里解 |
|---|---|
| 某个动作对应第几帧 | `*.anim` 的 `m_PPtrCurves` |
| 某个预制体用哪张图 | `*.prefab` 的 `m_Sprite` |
| **每一格**用哪张图、实体摆在哪 | `*.unity` 的 `m_Tiles` + `m_TileSpriteArray` |

> **钥匙**：Unity meta 里的 `internalID` **就是**引用里的 `fileID` —— 不掌握这一条就只能肉眼认图，
> 而认图**不可复查**（见 `fast-compile-loop.md` §3）。

### 6.1 素材文件本身的坑（切图/解码时撞上）

| 坑 | 症状 | 规矩 |
|---|---|---|
| **PNG 在 `IEND` 之后带垃圾数据** | 源素材的精灵表在 `IEND` 块后多出几百字节；严格解码器直接拒收（`unrecognised content at end of stream`），而 System.Drawing 却能容错打开 | **截断到 `IEND` + 12 字节**。**不要重新编码** —— 重编码会丢掉**调色板**（这些常是 4-bit 调色板 PNG），颜色会偏 |
| **Unity 的 `rect.y` 从底边往上算** | 切图时直接用会得到一堆上下错位的图，而且"看起来有点像对的"，很容易蒙混过去 | 必须翻转：`top = sheetHeight - (rect.y + rect.height)` |
| **精灵是紧裁剪的，尺寸不统一** | 同是"小马里奥"，帧高在 16~18 像素之间浮动；用**中心轴心**会让跑步时上下抖一像素 | **站地上的（角色/敌人/道具）统一用底部居中轴心，角色坐标 = 脚底位置** |

## 7. 将来要商用时的"替换清单"（现在不用管，留个口子）

1. 打开《资源欠缺清单》，把"来源 URL"列当替换清单；
2. 逐项找**可商用版本**（CC0 / 商用免费 / 已购）替换同路径文件 —— 因为**资源引用收敛**，
   换完不用改逻辑（这也是为什么 §3 ④ 要把路径收敛）；
3. 需要署名的补 `CREDITS.md`；付费包由用户购买。

---

## 8. 「原版资源」目录：所有下载 / 解包素材的**唯一来源**（硬约束）

> 用户原话：「如果要**下载的资源**，创建一个新文件夹，就叫 **原版资源**。
> 以后你游戏里使用的资源**从这个文件夹里复制走**！！！！！！！！！」
> 规则原文在 `SKILL.md::「原版资源」目录`；本节是落地细则。

**硬规则**：

1. **所有"下载 / 解包 / 提取"来的原版素材**（安装包、`*.mpq`、解包产物、rip 的 PNG / 模型 / 音效 / 文本 / 字体 / 图标）
   **只放一个地方**：**`<项目根>/原版资源/`** —— **名字就叫这四个中文字**，⛔ 不许改名、不许改成英文。
   内部按用途分层（例：`原版资源/安装包/`、`原版资源/mpq/`、`原版资源/解包原始/`、`原版资源/导出的png/`），
   ⛔ **不许散到工作区各处**（实测违规形态：解包产物在 `_assets_src/`、工作区根、项目里各来一份）。
2. **工程里用的素材必须"复制"过去**：`原版资源/**` → `client/Assets/Resources/**`。
   ⛔ **不许**让 Unity 工程直接引用 / 依赖 `原版资源/`（它不是资源目录）；⛔ **不许**边解包边原地引用。
   ⛔ **更不许"整包 / 整表全量搬运"**：只复制**被业务代码 / 关卡数据真正引用**的那几个文件。
   **实测代价（`clover-project-super-mario`，2026-09-20）**：三张原版素材表被整片切片进
   `Assets/Resources/Sprites/**` ⇒ `Resources` 里 953 个资源文件 **795 个无任何引用**
   （Items 95% / Enemies 88% / Tiles 80% / Scenery 72%）；`smb1_misc_sprites_0..548` 全在，
   而 `Core/ResPaths.cs` 只引用其中 **8 个**；`FlagpoleSprites` 10 张只用 `_0/_1/_6`。
   体积只有 1.26 MB，但**文件数** 1925 → 335 才裁回来，且"哪个切片是哪块砖"只能靠人猜。
3. **复制是唯一搬运方式**：将来换素材 = **改 `原版资源/` 里的源文件 → 重新复制一遍**，**不动业务代码**。
   ⇒ 所以工程内的资源路径必须**收敛**（客户端一律走 `Core/ResPaths.cs` 常量，不许散落字符串）。
4. **`原版资源/` 默认不进 git、不进交付包**（体积大 + 原版版权物）：写进 `.gitignore`。
   **唯一例外（用户本轮明说才生效）**：用户要求"原版素材随交付一起给" ⇒ 见 §8.2（zip + git-lfs）。
   但**必须留一份清单** **`原版资源/清单.md`**（来源 URL / 包名 / 解包命令 / 导出了什么 / 复制到工程哪些路径），
   **换台机器照着清单能完整复现**。
5. **旧位置清理**：历史遗留的 `_assets_src/`、工作区根散落的解包产物 ⇒ **一并迁进 `原版资源/` 并删掉旧目录**。

### 8.1 按需取用（pull）——**做成闸门，不做成提醒**

| 项 | 判据 |
|---|---|
| 进工程的每个资源 | 必须能指到"**谁引用它**"—— 指法**不止一种**，见下「引用形态清单」 |
| 闸门 | `tools/check-assets.ps1`（骨架 = `reference/check-assets-template.md`）：`client/Assets/Resources/**` 里**未被任何引用形态覆盖**的文件 ⇒ 记一行；**默认只报不删** |
| 依据 | `SKILL.md` §0.5「必然性规则必须做成闸门」—— 只写提醒的规则，实测都会烂回去 |

#### 引用形态清单（⛔ 漏一种就会把**活资源**判成未引用）

只做"文件名在源码里 `-notmatch`"是**错的** —— 它只覆盖第 ① 种：

| # | 形态 | 判据（任一命中 = 已引用） |
|---|---|---|
| ① | **精确文件名** | `"…/xxx_7.png"` 这类字面量出现在 `.cs` / 关卡数据 / `.prefab` / `.asset` / manifest 里 |
| ② | **目录 / 前缀常量** | 文件名所在目录出现在 `Core/ResPaths.cs` 的常量里（如 `D2Monsters = "D2/Monsters/{name}/"`）⇒ **整个目录视为已引用** |
| ③ | **帧序列（运行时拼名）** | `Frame(prefix, i)` 类助手 + `FrameCount*` 常量（如 `FrameCountSkillTreeBack = 16`）⇒ `skltree_s_back_0..15` **全部**已引用 |
| ④ | **目录级加载** | `LoadAll<T>(dir)` / `Resources.LoadAll` ⇒ 该目录已引用 |
| ⑤ | **生成物登记** | 由导出器生成、带"本文件是生成物，改它=重跑导出器"头的 `.cs` / `manifest.json` 里登记过的 |

#### 实测代价（2026-09-20，`clover-project-diablo2`）

按老口径（只做 ①）跑：`resource-files=11957 unreferenced=10529（88%）unreferenced-MB=29.5`，
听上去"整包全量搬运、该裁 88%"。**但逐组核对后全是假阳性**：

```
Clover\D2\Monsters 5129 / Chars 3431 / UI 1621(Quest 565 + SkillIcon 419 + SkillTree 79) / Items 342
这些目录 100% 被 ResPaths 常量（②）覆盖；Chars/Monsters 的帧名由 Frame(prefix,i)（③）拼出
ResPaths.cs 实测：FrameCount* 常量 11 个、Frame( 调用 17 次、还写明「也可以一次取全部 Resources.LoadAll<Sprite>(path)」（④）
```

⇒ **真该裁的接近 0**。若照老口径裁剪，`Quest 565 + SkillIcon 419 + SkillTree 79 + Chars 3431 + Monsters 5129` 张全部会被删掉、游戏大面积缺图。
**结论：闸门必须按上表五种形态判"覆盖"，且默认"保留优先"（fail-safe）；存量清理只报数（`-Warn`），删不删由人裁决。**

### 8.2 用户要求"原版素材随交付一起给"时：zip + git-lfs

```powershell
# 1) 打包（脚本化、可复现；⛔ 不要手工右键压缩 —— 每次内容都不一样，sha256 对不上）
powershell -NoProfile -File tools/pack-original-assets.ps1     # → <项目根>/原版资源.zip，并打印 sha256
# 2) LFS 声明（仓库根 .gitattributes）
git lfs install
'*.zip filter=lfs diff=lfs merge=lfs -text' | Add-Content .gitattributes
# 3) 提交
git add .gitattributes 原版资源.zip; git commit -m "refs: 原版素材打包（lfs）"
```

- ⛔ **zip 只放仓库根**，**不许**放进 `client/Assets/**`（Unity 会当资产扫；且违反 §8 第 1 条"唯一来源"）。
- zip 的 **sha256 + 打包脚本版本**记进 `原版资源/清单.md` ⇒ 收到的人能核对"解出来是不是同一份"。
- `原版资源/`（**目录本身**）**仍然**留在 `.gitignore` 里 —— 提交的是 zip，不是散开的原版目录。
- **能省则省**：打包前先看 `原版资源/` 里哪些子目录**不是**规格来源（与本作无关的参考工程 / 缓存 / `.git`）⇒ 不纳入。

> ⚠️ 与上文 §5.1「让用户提供本体」的关系：用户把游戏本体给过来之后，
> **本体落 `原版资源/`，解包产物也落 `原版资源/`** —— 不要一收到就直接解到工程目录里。
