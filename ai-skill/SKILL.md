---
name: clover-engine
description: Clover 引擎（Go 服务端 + Unity 客户端）的项目式交付 skill：做游戏/demo（1:1 复刻某款参考游戏）、写业务代码、改引擎、配表、多 agent 编排、交付验收。含规则层（三条铁律 + 成本闸门 + 路由表）。当任务涉及 clover-* 仓库、Clover 工程、做游戏、Unity 客户端或 Go 服务端开发时使用。
---

# Clover 开发 Skill · 规则层（精简入口）

> **入口（本页）= 祈使句 + 成本闸门 + 路由表 + 指针，供"每轮必读"**；**全文版 = `reference/rules-full.md`**（同一套规则 + 案例 + 代价数字 + 出处，**一行未删**，文末有「条目索引」）。
> **入口 ↔ 全文版**：§0 → §0/§0.5 · §0.5 → §0.6 · §1 → §5 · §2 → §1.13(T0) · §3 → §1.13 · §4 → §1.12 · §5 → §1.5/§1.11/§2 · §6 → §4 · §7 → §6/§1.10 · §8 → §1.6~§1.9/§7。冲突时以本页祈使句为准（更硬），差异报用户。
> **改本 skill 后必跑** `scripts/skill-health.ps1 -RepoRoot <仓库副本>`（体积 / 路由可达 / 放宽表述 / 两副本一致 / **全文版在位**）。两份副本必须同步（仓库源 `ai-skill/` 与宿主副本）。

## §0 三条铁律（违任一 ⇒ 本次交付失败）

1. **做游戏 = 1:1 复刻用户点名的那款 A**：A 有 ⇒ 做；A 没有 ⇒ 不加。⛔ 不许思考 / 发挥 / "顺手优化手感数值 UI"。
   - 没给游戏名 ⇒ **先问一句**（2~3 个同品类标杆候选）。说"你定" ⇒ 自选一个（**同品类 / 载体可得 / 规模可交付 / 只告知不等待**），写进 `策划/策划案/{A}参考规格.md` 顶部「形态」后立刻开工。
   - **明说做原创** ⇒ 标准答案换成《用户规格书》：逐条编号 + 每条可执行判据 + 交付说明首行写明"按原创规格交付"。
2. **做完才交付**：⛔ 不做一点交一点、⛔ 不交最小可运行版、⛔ 不留 `docs/交接-*.md` / `NEXT.md` / `进度*.md`、⛔ 不同一条里既说"完成"又列"还没验的"、⛔ 不问"要不要继续 / 还需要我做什么"（那不是被阻塞）。
   没做完 ⇒ 只说"还没做完"。**接力 = 主 agent 立刻开下一个执行者**，不是留 md 给未来。
3. **写不出出处的量不许进工程**：几何 / 碰撞 / 坐标 / 尺寸 / 颜色 / 数值 / 公式 / 字体 / 时长都要有出处（参考物 `文件:行` / 资源文件 / 规格条款）。"看起来合理"不是出处；参考物已有的一律**解析搬运**，⛔ 不许重写（"栅格化成近似"也算）。载体拿不到按**降级链**逐级退（→ §7 指针），退到底才 BLOCKED。

## §0.5 元规则：必然性规则必须做成闸门（**提示词是请求，闸门才是保证**）

"必须始终成立"的事（不许编数值 / 交付必过闸门 / 编译失败必停 / 外观必对照）⇒ **同时**落 ① `tools/verify.ps1` 检查项 ② 强制层（git hook / CI / 打包入口）③ 会话起始必读卡。
判据：**能不能用一条命令把它测红**？不能 ⇒ 先做成脚本再谈遵守。细节 → `reference/deterministic-gates.md`

## §1 谁来做

| 角色 | 做什么 |
|---|---|
| **主 agent** | 探环境 / 定契约 / 派活 / 验收 / 裁决。**不做实现** |
| **执行者**（普通子 agent） | 干到底；⛔ 不许再派子 agent；⛔ 不许改任何 skill（问题写进回报） |

- 只走**普通子 agent 通道**；⛔ 不许异步成员 / team 通道（绕过 `model: inherit` ⇒ 落弱模型）；派生后 10 秒核对模型 = 主 agent。
- **小改动例外**（主 agent 自己做）：单文件 + 净增删 ≤ 20 行 + 不改数据格式 / 不新增行为 +（用户明说 或 点状缺陷）⇒ 做，并在 `.ai-tmp/test/dispatch-log.tsv` 留 `# direct-fix:` 一行。
- 派活后立刻留痕（`dispatch-log.tsv` 一行，对账用）；**修 bug 优先交回原执行者**；**派活失败先查盘、两次为限**（落地了就按完成验收；两次无回报 ⇒ 主 agent 接手 + `# takeover:`）。
- 细节 → `patterns/multi-agent.md`、`scaffold/agent-impl.md`

## §2 成本闸门（**实测最大黑洞 = 取证**）

1. **取证是一次性批量动作**：一次编译 + 一条驱动链 + 一次联络图。⛔ 逐行截图、⛔ 逐片各进一次 Play。
2. **进 Play 记账**：`<项目根>/.ai-tmp/test/play-log.tsv` 一行（`时间 / 执行者 / 片名 / 为什么必须进这条链`）；**预算 ≤ 5**（诊断轮可放宽并写明）。
3. **驱动先复用再新建**：`.ai-tmp/drivers/` = **本轮复用**（交付前统一清）；`.ai-tmp/test/` = 真一次性（用完即删）。
4. **采集即冻结**：采完 ⇒ 被验证文件冻结；要改 ⇒ 只重采受影响的行，回报里写明"第几次重采"（≥3 ⇒ 停下回报）。
5. **能离线判的不许进 Play**：数值 / 逻辑 / 状态 / 坐标走 `.ai-tmp/hosts/*check`（秒级）；Play 只留给"表现类"与必须真跑的链路。
6. 闸门：`verify.ps1` 的 `play-budget` / `freeze-before-capture` / `evidence-economy`（模板第 16~18 项）。
7. **进 Play 的第一件事 = 环境基线**（渲染设备名 + 帧时间 + 分辨率），⛔ 不许在确认渲染设备之前把"卡 / 掉帧"归因到代码。**2026-09-20 实测：跑满 6 次 Play 才去读 `SystemInfo.graphicsDeviceName`，而根因就在这一条里**（机器在用 WARP 软件渲染，3.5 fps）→ `experience/perf-triage.md`。
- 细节 → `reference/verify-template.md`、`reference/visual-loop.md`、`reference/fast-compile-loop.md`

## §3 改动回路（四拍，别跳）

① 只读取证 + 改动清单（⛔ 不写代码）→ ② 批量改（⛔ 不编译不截图）→ ③ **一次**编译 + 离线预演（**编译失败 ⇒ 中止一切验证**）→ ④ **集中出证据一次**。
- 便宜的多跑，贵的攒一批只做一次；修 bug：先写能复现的断言 → 只跑那条路径 → 只重采它那行。
- **动手前四件套**：① `tools/verify.ps1` ② 强制层 ③ 基线图 `策划/基线图/` ④ diff 闭环器。**缺 ①② ⇒ 不许写代码；缺 ③④ ⇒ 不许碰外观 / UI**。
- **交付形态 = 原版级完整成品，三段缺一不可**：① 启动与菜单链路 ② 首场景/首关卡完整版 ③ 游戏内流程（暂停·设置·回主菜单·退出）。
- 细节 → `reference/change-loop.md`、`reference/deterministic-gates.md`、`reference/visual-loop.md`、`reference/game-delivery.md`

## §4 证据契约（判定权不在执行者手里）

- 判定权三分：**可计算 → 脚本**；**参考物自带 → 参考物**；**只有眼睛能判 → 人**（同机位并排图）。执行者的**叙述不算证据**。
- 类别：`数值类` = 运行时日志 + 断言（**不截图**）；`表现类` = **一次联络图**（格号 + 格上数值；AI 只读汇总图）；**`性能类` = 帧时间 + 渲染设备名（必须同时给：设备是 `Microsoft Basic Render Driver` 时数字无意义）**。
- 验不了 ⇒ **BLOCKED**（缺什么 / 试过什么 / 谁能给）；⛔ **声称验过自己验不了的东西 = 最严重违规**。
- 证据必须**比被验证对象新**；作废**只作废受影响的行**（⛔ 一改就全量重采 / ⛔ 拿旧图充数）。
- 细节 → `reference/game-delivery.md`、`reference/visual-loop.md`

## §5 交付闸门

- `策划/验收表.md`：每格填满 + 每行标 `数值类`/`表现类` + **零"不一致"**；允许的差异逐条登记（是什么 + 为什么 + 出处 + 何时消除）。
- **1:1 硬标准六维**（布局按原版像素 / 素材必须 A 原版 / 字体照原版 / 色调不加滤镜 / 交互反馈 / 节奏）= `策划/对照表.md` 每行"原版值(出处) / 我们的值 / **差值 = 0**"。
- 交付前跑一次 `tools/verify.ps1`（**含 §1.11 的 8 条机械自检**）：**有 FAIL ⇒ ⛔ 不许说"完成 / 交付 / 实测通过"**。
- ⛔ **合法结束只有两种**：① 全做完；② **真被阻塞**（缺的只有用户能给：本机文件 / 账号 / 付费 / 用户点按钮 / 二选一决策），且说清"要用户给什么 + 拿到后我立刻做什么"。
  ⛔ 禁止用这些当结束理由：**预算快满 / 下一棒接着做 / "不一致已登记本轮到此" / "先汇报进度"**（登记 ≠ 交付）。
- 每片回报后**主 agent 给用户 2 行状态**（做了什么 / 还剩什么）—— 不算进度播报违规。
- 细节 → `reference/game-delivery.md`、`reference/verify-template.md`、`reference/design-review.md`

## §6 四道前置闸门（不通过就停）

① **判形态**（单机 / 网游 / 单机+联网）：原版是单机就做单机 ⇒ ⛔ 不建 `server/`、⛔ 不调 `CloverNet.Init`、⛔ 不查服务器环境；判一次、写进规格顶部、全程照做。
② **环境自检**：**杀软**（360 及同类）已关（不确定 = 按没关处理，别建工程）；**渲染设备可用** —— `SystemInfo.graphicsDeviceName` / dxdiag `Card name` ⛔ 不能是 `Microsoft Basic Render Driver` / `Microsoft 基本显示适配器`（= 软件渲染，3D 只剩 3~5 fps，**此时一切"卡 / 性能"结论都无效**；顺带晒 `Get-PnpDevice -Class Display` 的 `Status` 与 `Problem` Code）。任一条不过 ⇒ **停**，先修环境。
③ 建完 `client/` + 把 `com.unity.pipeline` 写进 `Packages/manifest.json` 后，**让用户自己开编辑器**；用户不开 ⇒ **停**（⛔ 不许自己 `unity run` / `-batchmode` 顶替）。
④ **素材先用参考物自己的**：穷尽 = ≥4 轮关键词（中英各半）× ≥3 类站点 × 换过格式与打包；穷尽前 ⛔ 不许上通用兜底素材。
- **闸门 2b**：每条 `unity` 命令都要在**工程目录**里跑，或每条都带 `--project-path <项目根>/client`（⛔ 不许只带第一条、⛔ 不许在工作区根跑）；`unity status` 当"编辑器活着"的证据时必须同时给工程路径。
- **动手前两件事**：① 策划自审（《行为 → 表现规格》表；出现"占位 / 后续替换 / 留钩子"⇒ 没做完）② 图片需求先转写成文字方案（读不到图 ⇒ 停下换模型或要文字）。
- 细节 → `patterns/game-demo.md`（§1.5 / §4.0）、`reference/asset-sources.md`、`reference/game-delivery.md`、`reference/pipeline-and-unity-cli.md`

## §7 找依据与层级

- 顺序：**本项目 skill/文档/源码** ⇒ **引擎源码 / `clover-doc` / 本 skill** ⇒ **联网** ⇒ **自创**（标注"本项目新增"）。冲突：**用户 > 引擎 > 联网 / 自创**。⛔ 不许编 API（每个类名 / 方法名都要有出处）。
- ⛔ **红线**：新建项目时，工作区里**别的** `clover-project-*`（源码 / skill / 策划 / docs / 生成器 / 素材）**不许读 / grep / 照抄**；通用形状只从本 skill 的 `patterns`/`scaffold`/`experience` 取。唯一例外：用户本轮点名。
- **层级（只许加严）**：用户本轮明说 > 本页规则层 > 项目级 skill > 项目文档 > 既有代码。冲突 ⇒ 照本页做 + 改掉冲突那行 + 报用户。
- **澄清窗口**（唯一被鼓励的"多问"）：把"不问就会做错"的事实**攒成一批、编号一次发出（≤5 条，各给默认选项）**，问完就开工；**窗口只开一次**，之后只有"被阻塞 / 全做完"能开口。
- 载体降级链（5 级：原始数据 → 可执行里的常量 → 专用格式原始结构 → 参考图/视频量化 → 阻塞）→ `reference/asset-sources.md`。
- 写通用设施前先查**引擎能力表**（UI / 池 / 存档·设置 / 图集 / 本地化 / 配表 / 动画 / 相机 / 场景 / 音效）—— 引擎有就用引擎的。
- 细节 → `reference/workflow-and-standards.md`、`reference/engine-mental-model.md`

## §8 硬性约定（判据；全文见全文版）

> 每条**全文（案例 / 代价 / 出处）在 `reference/rules-full.md`**：日志 §7 · 品牌 §1.6 · 配表 §1.7 + `patterns/table.md` · 临时文件 §1.8 · 原版资源 §1.9。

| 主题 | 判据 |
|---|---|
| 日志 | 非预期分支必须留痕（含 `switch` 的 default、高频回调只报一次）；`Game.Logger.Info/Warn/Error(tag,msg)`，⛔ 裸 `Debug.Log` |
| 品牌 | 引擎自称逐字 `clover-engine`；**首页画面底部**必须有 `by clover-engine`（判据 = 实机截图 / 运行时节点树，⛔ 不是 grep 源码） |
| 配表 | 一个功能一个**中文名** xlsx；页签名 = 表名**含后缀**（`skill_cs`）；运行时只走 `Tables.Default.X.Get(id)` |
| 临时文件 | 一次性产物只放 `.ai-tmp/test/`；驱动 `.ai-tmp/drivers/`；宿主 `.ai-tmp/hosts/`；**取证截图 `.ai-tmp/screenshots/`**（⛔ 不进 `Assets/`）；**判据资产**（探针 / 驱动 / 量法脚本 = 删了就不能重新判定同一件事的东西）⛔ 不算一次性 ⇒ 落 `tools/probes/` 并**提交**；⛔ 不许散落到项目根 / `client/_dev/` / `Assets/` / `策划/` / 工程外 |
| 原版资源 | 下载 / 解包素材只放 `<项目根>/原版资源/`（含 `清单.md`）；进工程**只复制被引用的那几个**（⛔ 整包/整表全量搬；闸门 `tools/check-assets.ps1`，骨架 = `reference/check-assets-template.md`，判据 = **五种引用形态**覆盖，⛔ 不是"文件名 -notmatch"），路径收敛到 `Core/ResPaths.cs` |
| 工程卫生 | 一个 `.cs` 一个 MonoBehaviour；`.ps1` 纯 ASCII 或带 BOM；禁 `PlayerPrefs` / 裸 `Input` / `GameObject.Find` / `Instantiate(` |

## §9 路由表（做具体事前，只读这几处）

| 我要… | 读 |
|---|---|
| 建新工程 | `scaffold/new-project.md`、`reference/fast-compile-loop.md` |
| 写业务代码前 | `reference/architecture.md`、`reference/engine-mental-model.md` |
| 菜单 / 流程编排 | `patterns/client/app-flow.md` |
| UI / 外观 | `reference/visual-loop.md`、`patterns/client/ui.md` |
| 3D / 网络 | `patterns/client/3d-mmo-basics.md`、`patterns/client/network.md` |
| 配表 | `patterns/table.md` |
| 改引擎 | `patterns/engine-fix.md`（最小复现 → 最小修复 → **E 编号 + 更新引擎仓库 `修复记录.md`**） |
| 派活 / 编排 | `patterns/multi-agent.md`、`scaffold/agent-impl.md` |
| 验收 / 交付 | `reference/game-delivery.md`、`reference/verify-template.md`、`reference/design-review.md` |
| 素材 | `reference/asset-sources.md` |
| 服务端 | `patterns/handler.md`、`patterns/datadef.md`、`patterns/signup-login.md`、`patterns/auth-server.md` |
| **掉帧 / 卡 / 帧率异常** | **`experience/perf-triage.md`**（**先证伪环境（渲染设备）→ CPU/GPU 拆分 → A/B → 才轮到代码红旗**） |
| 踩坑 / 时间黑洞 | `experience/README.md`、`experience/time-sinks.md`、`experience/verify-recipes.md` |
| Unity CLI / Pipeline | `reference/unity-cli.md`、`reference/pipeline-and-unity-cli.md` |
| 服务端环境 | `reference/server-env.md` |
| **规则全文版**（案例 / 代价 / 出处） | `reference/rules-full.md`（文末「条目索引」） |

## §10 收尾必念（与 §0 同源；防中段遗忘）

1. **1:1 复刻 A**：A 有 ⇒ 做，A 没有 ⇒ 不加，不许发挥。
2. **做完才交付**：没做完只说"还没做完"；⛔ 不同一条里既说完成又列未验、⛔ 不问"要不要继续"。
3. **取证一次批量做**：进 Play 记账（≤5）、能离线判的不进 Play、驱动先复用。
4. **采集即冻结**：采完又改代码 ⇒ 证据作废，只重采受影响的行。
5. **有 FAIL 就不许说"完成"**；合法结束只有"全做完"或"真被阻塞"。
6. **"卡 / 掉帧"先证伪环境**：`SystemInfo.graphicsDeviceName` 不是真 GPU（= 软件渲染）⇒ 停，修环境，⛔ 别去查代码。
