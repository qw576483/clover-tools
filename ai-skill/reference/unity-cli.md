# Unity 6 + Unity CLI 规范（AI 自动化操作的强制前置条件）

> **适用范围（务必看清）**：这是**对 AI 的要求，不是对项目的要求**。
> 项目本身用哪个 Unity 版本由项目自己决定；但**只要 AI 要以自动化方式操作/落地客户端代码**
> （改完代码要编译验证、跑测试、构建、改场景/资源、读 Console、截图等），
> AI **必须**通过 **Unity 6（6000.x）+ Unity CLI** 来做，否则不许动手。

典型触发场景：AI 生成/修改了 `Assets/**/*.cs` 或场景资源后，需要验证是否能编译通过、单测是否绿、能否出包。

## 1. 硬性约束（针对 AI 的自动化行为）

### 1.1 只支持 Unity 6（6000.x）—— 硬性要求

- 客户端的一切自动化操作（创建工程 / 编译 / 测试 / 构建 / 改场景与资源）**一律 Unity 6（6000.x）+ Unity CLI**；
  **其他版本不支持**：老工程先 `unity projects upgrade` 或重建，**不要做降级适配**。
- 自检只认**真装了的**：`unity editors list --format json` 里**必须有 `location` 字段**才算装了；
  没有 `location` 的条目只是"已知版本"。**实测踩过**：拿这种条目 `--editor-version` 建工程直接失败  
  （"未找到版本 X 的 Unity Editor"）。
- 模板 id 现场查：`unity templates list --editor 6000.0.x --format json`（不要猜 id）。

### 1.2 需要编辑器时：**默认请用户打开**；用户不开就**停下**（★★★ 硬闸门）

> **默认动作是「请用户打开」，不是「自己打开」。**
> 自己起编辑器（`unity open` / `unity run` / `-batchmode`）**一律禁止，没有例外**。

**标准节奏**（刚创建完工程时**必须**走这条）：

1. **AI 先请用户打开**，**并给出可直接照做的步骤**（不许只说"请打开工程"）：
   > 请把这个工程加到 Unity Hub 并打开（首次导入素材会慢几分钟，属正常）：
   > 1. 打开 **Unity Hub** → 左侧「**项目 / Projects**」页
   > 2. 点右上角「**添加 / Add**」→「**从磁盘添加项目… / Add project from disk**」
   > 3. 文件夹选择框里定位到 `<项目根绝对路径>`，**选中里面的 `client` 文件夹本身**
   >    ——⚠️ **选 `client`，不是它的上一级**（`client` 才是 Unity 工程根：含 `Assets/`、`Packages/`、`ProjectSettings/`）
   > 4. 点「**选择文件夹 / Select Folder**」
   > 5. 点列表里的 `client` 打开 → 等导入 + 编译完
   > 6. **打开后回我一声**，我继续（我会编译 + 生成预制体/场景 + 进 Play 实测截图）
   >
   > 路径（可直接复制）：`<项目根绝对路径>/client`
   >
   > 第 2~4 步也可用命令行代劳：`unity projects add "<项目根>/client"` + `unity projects pin "<项目根>/client"`。
2. **用户打开后** → 按 **§8.1** 驱动它：`unity status`（确认 `state=ready`）→ `eval_file` /
   `editor_play` / `capture_game_view` / `recompile_status`。
3. **用户没开 / 说不想开** → **停下，不继续做**。不许"我自己批处理一下也行"。

**为什么不许自己跑**（实测代价，别重复踩）：

- 首次导入几百上千张素材 + 全量脚本编译，会把**一次往返拖到十几分钟**；
- 授权 / 杀软任一环节出问题就是**静默卡死**（见 §9），时间全烧在等它返回上；
- **批处理下 `-executeMethod` 执行完立刻退出，进不了播放模式** ——
  所以"跑一遍看画面"在批处理下**根本做不了**，自己跑也得不到你要的结果。

> **禁止**因为"用户没开编辑器"就跳过实测直接交付 —— 那是半成品的主要来源；
> **禁止**编辑器开着时硬跑 `unity run`（会报「项目已在运行中的编辑器中打开」）。
> **没有例外。** 即使用户说「你就自己跑，别烦我」，也**不做** ——
> 把原因讲清楚：批处理下 `-executeMethod` 执行完立刻退出、**进不了播放模式**，
> 自己跑既慢又验不了画面。**用户不打开编辑器 = 停在这里。**

| 项 | 要求 |
|----|------|
| 自动化用的编辑器 | **Unity 6 / 6000.x**；若工程当前不是 6000.x，AI 需先确认能否切到 6000.x 自动化，不能则应明确告知用户并给出方案，而不是硬着头皮用旧版跑 |
| 交互方式 | **Unity CLI**（`unity ...`），禁止把 `Unity.exe -batchmode -quit -executeMethod Xxx` 当首选手段 |
| 输出格式 | 给 AI 解析时一律加 `--format json`（或 `--format tsv`），不要解析人类可读表格 |
| 平台 | Windows 用 PowerShell 执行；macOS/Linux 用同一套 `unity` 子命令 |

**禁止**：
- 直接拼 `Unity.exe` 绝对路径执行批处理任务（Hub 路径随机器变化，不可移植）
- 用 Unity Hub GUI 手工点，然后让 AI 盲猜结果
- 在 Editor 打开且未保存时直接改磁盘上的 `Assets/` / `ProjectSettings/`（先 `unity` 保存或关闭 Editor）
- 把工程、`-logFile`、内容目录放在**点开头的目录**下（`.codebuddy/`、`.cache/` …）：
  Unity 会报 `xxx is not a valid directory name`，**并且会把机器级 UDS 共享状态弄坏，连累之后所有正常路径下的工程**（详见 §9）
- 在导入/编译进行中**强杀 Editor 进程**（超时、取消、`Stop-Process`）：会写坏 `<工程>/Library/DataStore`，
  之后每次启动都崩在 `UDSInterface::UDSInterface`，表现为「换工程、删了重建都一样」（详见 §9）

**例外**：只有当 Unity CLI 确实没有对应能力时（如 Terrain、Addressables、ParticleSystem 模块、Shader Graph），
才可退回 `unity command ...` 的 `eval` 逃生口跑 C#，或在 CLI 能力外再考虑 Editor 脚本 + `-executeMethod`，
并在回复里说明为什么 CLI 做不到。

## 2. 前置检查：没有 CLI 就自动安装

AI 在第一次需要操作 Unity 之前，**必须**先自检；检测不到就**直接帮用户装上**，不要停下来问。

```powershell
# 1) 检测（PowerShell）
if (Get-Command unity -ErrorAction SilentlyContinue) { unity --version } else { "UNITY_CLI_NOT_FOUND" }
```

```bash
# 1) 检测（macOS / Linux）
command -v unity && unity --version || echo "UNITY_CLI_NOT_FOUND"
```

输出 `UNITY_CLI_NOT_FOUND`（或命令不存在）时，执行安装：

```powershell
# 2) 安装（Windows / PowerShell，官方脚本）
$env:UNITY_CLI_CHANNEL='beta'; irm https://public-cdn.cloud.unity3d.com/hub/prod/cli/install.ps1 | iex
```

```bash
# 2) 安装（macOS / Linux）
export UNITY_CLI_CHANNEL=beta
curl -fsSL https://public-cdn.cloud.unity3d.com/hub/prod/cli/install.sh | sh
```

安装后：
- 脚本会提示 `Note: Restart your terminal to use unity.`
- **当前 shell 可能还没有 `unity` 命令**，需要新开终端，或用完整路径 / `refreshenv` 后再自检一次
- 再次运行检测命令确认 `unity --version` 有输出，才继续后续步骤

可执行文件名统一是 **`unity`**（不是 `unity-cli`、`unityhub`）。

## 3. 让 AI 客户端自己学会用 Unity CLI（强制）

> **这是硬性要求，不是建议**：只要 AI 要敲任何 `unity` 子命令，就必须先完成本节安装。
> 没装 unity-cli skill 就凭记忆拼命令 = 违规。

Unity CLI 自带一份 **agent skill**（离线内置在 CLI 里），可以直接装进 AI 客户端，避免 AI 凭记忆乱猜命令：

```bash
unity skill show                 # 只打印 skill 内容（只读，不写盘，可当权威命令手册读）
unity skill show --list          # 列出 skill 由哪些文件组成（SKILL.md + references/*）
unity skill install --list       # 查看支持的 AI 客户端及安装状态
unity skill install <client>     # 安装到指定 AI 客户端，如 claude-code / cursor / codex / vscode
unity skill refresh              # CLI 升级后，用新版本重新渲染已安装的 skill
```

- 安装位置示例：`claude-code → ~/.claude/skills/unity-cli/`、`codex → ~/.agents/skills/unity-cli/`
- 命令记不准时，先 `unity skill show` 取权威用法，再动手。
- CLI 升级后跑 `unity skill refresh` 重新渲染已安装的 skill。

### 认出自己，装到自己身上（AI 必做）

**AI 要"识别"的不是用户用什么工具，而是自己是什么。**

AI 的身份（模型名 + 编排工具）在会话开始时就由运行时注入，**AI 一定知道自己是谁**
（例：Hy4 + CodeBuddy、Claude + Claude Code）。所以：

- **禁止**问用户"你用的什么 AI 工具"；
- **禁止**靠 `~/.claude`、`~/.cursor` 这类目录探测来反推"我现在跑在哪个工具上"——
  机器上装过什么 ≠ 当前会话跑在什么上面，用目录猜自己的身份是**错的**；
- 正确做法：**认出自己 → 找到自己的 skill 目录 → 自己把 unity-cli skill 放进去 → 当场用它**。

**步骤（自己想，别指望现成脚本）：**

1. `unity skill install --list` 看官方支持哪些客户端。
2. **里面有自己的**（下表前 8 个）→ `unity skill install <自己>`。
3. **里面没有自己**（CodeBuddy、自研 Agent 等）→ 自己想办法搬进去：
   - `unity skill show` 打印权威正文 → 自己写进自己的 skill 目录；或
   - 从任一已安装客户端的 `skills/unity-cli/` 复制过来。
   目录约定与 Claude 系一致（`<你的 skill 根目录>/unity-cli/SKILL.md`），**具体路径你自己确认**——   
   你跑在什么上，就知道自己从哪儿读 skill。
4. **验收并当场使用**：确认 `unity-cli/SKILL.md` 已在自己的 skill 目录里，然后**立刻读进上下文**再动手。

| 若自己是… | client 名 |
|---|---|
| Claude Code | `claude-code` |
| Claude Desktop | `claude-desktop` |
| Cursor | `cursor` |
| Windsurf | `windsurf` |
| VS Code + GitHub Copilot | `vscode` |
| Cline | `cline` |
| OpenAI Codex CLI | `codex` |
| Grok Build | `grok` |
| CodeBuddy / 其他 | 官方列表不含 → 按第 3 步自己动手 |

**时效（别误以为都要重启）：**

- skill 索引一般下一条消息即可见；skill 正文**当场读/加载即用，无需重启会话**。
- 只有 `unity` 二进制的 **PATH** 需要新开终端（刚装完常见）；在那之前用安装器给出的完整路径调用即可。
- CLI 升级后 `unity skill refresh`；不在官方列表里的客户端（如 CodeBuddy）要自己再搬一次。



## 4. 常用命令速查

```bash
unity --help              # 查看全部命令
unity upgrade             # 升级 CLI 自身
unity editors list        # 列出已安装/可安装的编辑器（--format json / tsv）
unity install             # 安装 Unity 6 编辑器（缺编辑器时用它，别手工下载 Hub）
unity install --version 6000.0.58f2 --module android ios webgl   # 指定版本 + 模块
unity pipeline install    # 装 Unity Pipeline 包（AI 操控 Editor 的前置依赖）
unity command             # 连接本机已打开的 Editor，列出可执行的 142+ 条 Editor 命令
unity build run --target android --output build/app.apk          # 异步构建 + BuildReport
unity test <工程路径> --mode EditMode|PlayMode --output <报告路径>   # 跑单元测试（§4.1）
unity console tail --format json                                  # 增量读 Console 日志（cursor 追踪）
unity screenshot --source scene --output shot.png                 # 场景/Game 视图截图
unity project audit --format csv                                  # Project Auditor 静态分析
```

`unity editors list --format json` 返回结构（字段稳定，可安全解析）：

```json
{ "success": true, "command": "editors", "data": [ { "version": "6000.0.58f2", "alias": "6.0.58f2", "architecture": "x86_64", "default": false } ] }
```

### 4.1 跑引擎自带测试（EditMode / PlayMode）

**`clover-client-unity-engine/` 是 UPM 包，不是 Unity 工程**（目录里没有 `ProjectSettings/ProjectVersion.txt`、没有 `Assets/`）。
直接 `unity test clover-client-unity-engine` 会报 `不是 Unity 项目`；**必须指向一个承载它的工程**。

承载工程要满足两条（详见 `scaffold/new-project.md` §2.2）：

1. `Packages/manifest.json` 里列了 `com.clover.unity-engine`；
2. `Packages/manifest.json` 的 **`testables` 里也列了 `com.clover.unity-engine`** ——
   不加的症状**是静默的**：`unity test` 正常退出、报告 `testcasecount="0"`，一条用例都不跑。

```bash
# 全部 PlayMode / EditMode 用例，各落一份 NUnit XML
unity test <承载工程路径> --mode PlayMode --output <报告路径>/playmode-results.xml
unity test <承载工程路径> --mode EditMode --output <报告路径>/editmode-results.xml
```

**读结果**：解析报告里的 `<test-run ...>` 属性，别去翻 Editor 日志 —— `passed / failed / skipped / testcasecount` 都在这一行：

```powershell
[xml]$x = [IO.File]::ReadAllText('<报告路径>', [Text.Encoding]::UTF8)
$x.SelectNodes('//test-run') | Select-Object -First 1 -ExpandProperty OuterXml
```

**写 PlayMode 用例的三个坑**（都踩过，而且**都会伪装成「实现有 bug」**）：

| 现象 | 真因 | 正确做法 |
|---|---|---|
| WS 用例恒红：客户端 `State=Disconnected`、就是连不上 | Unity/Mono 的 `HttpListener` **不支持 WebSocket 升级**（`IsWebSocketRequest` 恒 false、`AcceptWebSocketAsync` 不可用），假服务端会对着合法的升级请求回 400 | 别用 `HttpListener`：改用裸 `TcpListener` 自做 RFC 6455 握手（范例 `clover-client-unity-engine/Tests/PlayMode/MiniWsServer.cs`） |
| 推送 / 回包用例超时，但链路明明连上了 | 收包队列只在 `NetworkManager.Tick()` 里被排空（`Tick → TryTakePacket → DrainFrame`），测试里只 `yield return null` 的话帧会**一直躺在队列里** | 等待循环中**每帧泵一次** `nm.Tick()`（范例 `TransportLineLoopbackTests.WaitUntilPumping`） |
| 等待类用例报"超时"，但**日志显示数据早收到了** | 等待助手把**有副作用的谓词求值了两次**：`while(!cond()) ...; Assert.IsTrue(cond())` —— 循环里那次成功后（如 `TryTakePacket` **已出队**）断言里再求值就成了 false | 助手内**只求值一次、用它断言**：`var ok = cond(); while(!ok && 未超时) { yield return null; ok = cond(); } Assert.IsTrue(ok, ...)`（范例 `Tests/PlayMode/QuicLoopbackTests.cs` 的 `WaitFor`） |

> 同理：跑 `TestCase` 数对不上的时候，先确认**承载工程的 `testables`**，再怀疑测试框架 —— 见 `scaffold/new-project.md` §2.2。

## 5. 标准操作流程

1. **自检 CLI**（`unity --version`）→ 没有就按第 2 节装
2. **确认编辑器**：`unity editors list --format json` → AI 自动化要用的 Unity 6（6000.x）不存在就
   `unity install --version 6000.0.x`（只是为自动化补一个编辑器，不改项目本身的版本要求）
3. **确认工程**：在项目根目录（含 `Assets/`、`ProjectSettings/`）执行，或显式 `--project <path>`
4. **执行任务**：构建 / 测试 / 编译 / 场景与资源编辑 / 截图
5. **读结果**：`--format json` 解析；失败时 `unity console tail --format json` 取日志再定位

## 6. 新建 Clover 客户端工程（强制用 CLI，禁止 mkdir）

**从头建游戏时，`client/` 一律用 `unity projects create` 生成，禁止手工 `mkdir`。**
完整 SOP 见 `patterns/client/new-project.md`，此处只放要点。

```bash
# 1. 前置：CLI + Unity 6（硬门槛，没有 6000.x 先 unity install 再继续）
unity --version
unity editors list --format json
unity templates list --editor 6000.0.x --format json     # 列真实模板 id，不要猜

# 2. 创建（工程名=client，--path=项目根 ⇒ clover-{项目名}/client/ 即工程根）
unity projects create "client" --path "clover-{项目名}" \
  --editor-version 6000.0.x --template com.unity.template.2d
```

> 命令是 `unity projects create`（`projects` 复数），**不是** `unity create project`。

**验收五条，缺一即失败、删掉重来：**
1. `client/ProjectSettings/ProjectVersion.txt` 的 `m_EditorVersion` 是 **Unity 6（6000.x）**
2. `client/Packages/manifest.json` 含 `"com.clover.unity-engine"`
3. `client/Assets/Scripts/{项目名}.asmdef` 的 `references` 含 5 个 `CloverEngine.*`
4. `client/*.sln*` 存在（Unity 6 生成的是 **`client.slnx`**，不是 `.sln`；不要因为没看到 `.sln` 就判定失败）
5. **Hub 注册表里有它、且已置顶**（见 §6.1）

### 6.1 建完必须让用户「看得到」工程（硬约束）

工程建在 `clover-{项目名}/client/`，而 **Hub 用最后一级目录名当标题** ⇒ 列表里它叫 **`client`**，
用户机器上往往还有别的 `client`，一排同名条目里根本认不出哪个是新建的。
**已真实踩过：用户反馈"我怎么没看到你创建的项目？"**

建完立刻做三步，缺一即交付失败：

```bash
# ① 登记进 Unity Hub 注册表（create 不保证已登记）
unity projects add "<项目根>/client"

# ② 置顶（Hub 顶部 Favorites 区，一眼可见）；参数是**路径或标题 glob**
unity projects pin "<项目根>/client"

# ③ 复核（确认 isFavorite=true 且 path 正确）
unity projects list --format json
```

④ **交付说明里必须写清两行**，让用户照着点就行：
   - 「Hub 里显示为：**client**（标题=目录名）」
   - 「路径：`<绝对路径>/client`」
   并附一条可直接复制的 `unity open "<绝对路径>/client"`。

> 只把工程"建在磁盘上"不算交付 —— 用户看不见 = 没交付。

## 7. 与 Clover 客户端工程的配合

- 客户端工程目录：`clover-{项目名}/client/`（或 `clover-client-unity-engine/`）
- CLI 只做**工程级操作**（编译、测试、构建、资源/场景编辑、日志、截图）；
  **业务代码**仍按 `patterns/client/*` 与 `reference/client-conventions.md` 编写，不要靠 CLI 生成业务脚本
- 场景/资源改动完成后，提醒用户在 Unity 6 Editor 里确认（CLI 无 Undo 集成）
- 构建产物不要提交；`Library/`、`Logs/`、`Temp/`、`obj/` 必须已在 `.gitignore` 中

## 8. 场景创建 + 脚本挂载：AI 自己做，不许丢给用户

**交付铁律**： Unity 客户端的交付标准是「**打开工程 → 打开场景 → 直接 Play 就能跑**」。
下面这些事**全是 AI 的活**，不许在交付说明里写"请用户在场景中创建空物体并挂载 Xxx 脚本"：

- 创建 / 保存场景（如 `Assets/Scenes/Main.unity`）；
- 场景加进 **Build Settings** 的 Scenes In Build（否则打包后跑不起来）；
- 创建挂载点 GameObject（如 `Bootstrap`、`GameRoot`）；
- 把业务脚本 `AddComponent` 挂上去；
- 必要引用（如 UI 根节点、prefab 引用）能自动 `Find`/`GetComponent` 就不要留手工拖拽的依赖；
  确实需要序列化引用时，也要在代码里做空判断 + 运行时兜底创建，保证无手工配置也能跑；
- 保存场景 + 刷新资产数据库（`AssetDatabase.SaveAssets` / `Refresh`）。

**做法**：优先用 `unity` 的场景/资源相关子命令；记不准就先 `unity skill show` 取权威用法
（或 `unity command` 看 Editor 侧可用命令）。CLI 没有对应能力时，走 §1 的 **eval 逃生口**跑 C#：

```csharp
// 通过 unity command eval 执行（Editor 环境）
var scene = UnityEditor.SceneManagement.EditorSceneManager.NewScene(
    UnityEditor.SceneManagement.NewSceneSetup.DefaultGameObjects);
var root = new GameObject("Bootstrap");
root.AddComponent<Bootstrap>();           // 业务入口脚本
UnityEditor.SceneManagement.EditorSceneManager.SaveScene(scene, "Assets/Scenes/Main.unity");
UnityEditor.AssetDatabase.SaveAssets();
UnityEditor.AssetDatabase.Refresh();
```

```bash
# 保存后确认场景已入 Build Settings，并做编译验证
unity console tail --format json      # 有编译错误就先修完再交付
```

**验收**：交付前必须确认——场景文件存在、脚本已挂在场景里的 GameObject 上、
`unity console tail` 无编译错误、用户在 Editor 里点 Play 即可运行（不需要任何手工挂脚本操作）。

### 8.1 驱动**正在运行**的编辑器：AI 交付前自证的首选路径（★ 最高效）

**用户很可能已经把工程开在编辑器里**。此时**不要**再用 `unity run`（会直接报
「项目已在运行中的编辑器中打开」），也不要去手改 `.unity`/`.prefab` 文本 ——
**直接驱动那个活编辑器**（unity-cli 的设计用途）：

```bash
unity status --format json          # 看 instances[].state 是否 "ready"（及 pid/port）
unity pipeline list                 # 安全模式自检：Safe Mode=true 说明有编译错误，先修代码
unity command                       # 列出该编辑器开放的命令（能力随包版本不同，先看再调）
```

常用命令（0.7.0-exp.1 实测可用）：

| 目的 | 命令 |
|---|---|
| 触发/查询编译 | `unity command recompile` → 轮询 `unity command recompile_status --format json`（解析 `data.result` 字符串里的 `failed` / `errors`） |
| **跑一段 C# 装配/自证** | `unity command eval --code '<C#>'`；**代码里有空格/引号时必须用文件**：`unity command eval_file --file <绝对路径>.cs`（PowerShell 会把带空格的参数拆散，`--code` 会报"接受 1 个位置值，但收到 N 个"） |
| 进/出 Play 实测 | `unity command editor_play` / `editor_stop`（`editor_status` 看状态） |
| 读运行日志 | `unity command console --tail 300`（**落盘后再解析**：输出是 `命令<TAB>成功<TAB>{JSON}<TAB>参数`，直接用 `[IO.File]::ReadAllText` + 正则取 `"message":"..."` 最稳） |
| 清日志（先清再测） | `unity command clear_console` |
| **截图取证** | `unity command capture_game_view --source screen --save_path Screenshots/x.png`（`source=screen` 才能截到 **Screen Space-Overlay 的 HUD**；**仅 Play 模式**；`save_path` 相对**作者根 `Assets/`**，且**必须在工程目录内**，否则 400） |
| 场景内物体/找资产 | `find_gameobjects` / `find_assets` / `create_gameobject` / `add_component` / `attach_script` |
| Animator 资源操作 | `add_animator_parameter|state|transition|layer`、`get_animator_controller`、`get_animation_clip` |

**`eval` 里的 C# 注意**：没有 `using`（写全限定名，如 `UnityEngine.Vector3.Distance`）；
返回值放在文件末尾 `return "ok";`；长耗时操作给 `--timeout`（秒）。

**交付前的"自证四连"**（不许口说"已经好了"）：

```bash
# ① 编译干净
unity command recompile_status --format json         # failed=false
# ② 进 Play，跑一会儿，读关键日志（自己的 Tag 前缀）
unity command clear_console; unity command editor_play; sleep 40
unity command console --tail 400                     # 找 [Boot]/[Motor]… 等关键日志，且**无异常**
# ③ 运行时自证：把关键对象状态打印出来（比"看着像"可靠）
unity command eval_file --file clover-{项目名}/client/verify-runtime.cs
# 例：motor=True camHasTarget=True camDist=6.01 hud=True model=True clip=Idle ctrl=Knight
# ④ 截图（含 HUD 的合成画面）
unity command capture_game_view --source screen --width 1600 --height 900 --save_path Screenshots/play.png
# ⑤ 退 Play，别把编辑器留在 Play 模式
unity command editor_stop
```

### 8.2 在**活编辑器里**跑 PlayMode 用例（不用抢工程，★ 实测可用）

用户开着工程时，**不要**跑 `unity test <路径>`（会报"项目已在运行中的编辑器中打开"）；
活编辑器自己提供了测试命令（`unity command` 里能看到 `list_tests` / `run_tests` / `test_status` / `cancel_tests`）：

```bash
unity command list_tests --mode PlayMode                                   # 先确认用例被注册
unity command run_tests --mode playmode --filter <类名或命名空间> --async_tests --timeout 300
# 然后轮询（同步调用会失败：进 Play 触发域重载，HTTP 请求会被打断）
unity command test_status                                                  # status: running → completed
```

`test_status` 返回 `summary{total,passed,failed,skipped}` + `results[]{FullName,Status,Message,StackTrace}`，
直接读 `Message` 就能拿到断言失败原因（不用去翻 Console）。

**三条纪律（都在真实项目里踩过，不遵守就会得到"假失败/假成功"）**：

| # | 纪律 | 反例 |
|---|---|---|
| 1 | **必须 `--async_tests` + 轮询** | 同步调用报 `PlayMode tests cannot run synchronously over HTTP: entering play mode triggers a domain reload that drops the request` |
| 2 | **等真实时间**（`yield return new WaitForSeconds(0.9f)`），不要只 `yield return null` 数帧 | 相机/插值类是**平滑**的：PlayMode 测试里 30 帧 ≈ **0.1 秒**，根本没收敛 ⇒ 断言读到"半路值"（实测相机距离 3.37m，看着像避障失效） |
| 3 | 清理用 **`DestroyImmediate`**；**"是否重复投递"以服务端日志为准** | `Destroy` 延迟到帧末 ⇒ 上一用例的墙被下一用例的探针打到（实测读数 2.90m 正好是上一用例那堵 3m 墙）。 活编辑器里跑用例时，**测试与场景里的业务入口（Bootstrap）会抢同一个 `Game`/连接**，用例里的事件计数会虚高（实测 `enter=5347`）——那是**环境产物**，不是产品缺陷 |

> 顺带一条：**别在 `eval_file` 里 `Thread.Sleep` 驱动游戏**。`eval_file` 跑在编辑器主线程上，
> 睡眠会把"发送帧"一起卡住（你以为连打了 4 下，实际只出去 1 下）。
> 要连打就连开多次 `eval_file`，由外壳（PowerShell/bash）控制间隔 —— 主线程始终是自由的。
>
> **`eval_file` 有 5 秒主线程上限**（超时报 `Main thread operation timed out after 5000ms`）：
> 凡是需要"等资源导入 / 编译 / 大批量 `AssetDatabase.ImportAsset` / 大文件加载"的脚本，一律**拆小或分多次调用**。
> 踩过的例子：一次 `eval_file` 里强制重导 9 个 20MB 的 FBX ⇒ 必然超时；而导入其实**已经在编辑器后台跑了**，
> 正确做法是"先提交一小批 / 先等一会儿，再用一个**只读**脚本查结果"（`LoadAllAssetsAtPath` 查材质贴图关联就是只读的）。
>
> **截图要自己看，但按`类别`看**：`capture_game_view --source screen`（仅 Play 模式；编辑模式用 `--source camera`，
> 否则报 `requires Play Mode`）。`表现类` 的项**采一次联络图、AI 只读那张汇总图**（格式见 `reference/visual-loop.md` 第八节）；
> `数值类` **不必截图**（`SKILL.md` §2 硬性判定第 3 条）。
> 数字全对但画面不对的静默失效（血条永不变化、角色纯白无贴图、血条细到看不见）只有看图才发现 ——
> 这正是"`表现类` 必须进图"的理由。详见 `reference/design-review.md` §3.4。

> `verify-runtime.cs` 放工程根（**`Assets/` 之外**，Unity 就不会把它当业务脚本编译），
> 内容是一段 top-level 语句：用 `FindFirstObjectByType<T>()` 找到自己的组件，
> 打印"是否挂上 / 目标是否绑定 / 模型是否加载 / Animator 当前 clip 与 normalizedTime"。

## 9. 故障排查：创建/打开工程「静默卡死」

**症状**：`unity projects create` / `unity run` / `unity test` 长时间不返回、CLI 一个字都不输出；
`ProjectSettings/ProjectVersion.txt` 始终不生成；主观感受是「跑了一小时没动静」。
**这类问题几乎都不是 Unity 慢，而是 Editor 起不来在死等。** 按下面顺序查。

> **★ 但在排查之前，先回到"预防"——这一步能省掉 90% 的此类事故：**
> 1. 工程还没创建 → **先问用户 360 关了吗**（没关就别建，见 `scaffold/new-project.md` §2.0）；
> 2. 工程已创建 → **请用户自己在 Hub 里打开**，**不要自己跑 `unity run` / `-batchmode`**
>    （见本文 §1.2）。
>
> **本节列的是"已经卡住了怎么救"，不是"可以放心自己去跑"。** 顺序反了就是拿几十分钟换一个本可避免的卡死。

### 9.1 先分清「慢」和「阻塞」

```powershell
# CPU 时间在涨 = 慢（正在导入/编译）；几乎不动 = 阻塞（在等 I/O 或某个服务）
Get-Process -Name Unity -ErrorAction SilentlyContinue |
  Select-Object Id, @{n='CPU秒';e={[math]::Round($_.CPU,0)}}, @{n='内存MB';e={[math]::Round($_.WorkingSet64/1MB,0)}}
```

### 9.2 三个已确认的成因

**(1) 杀软拦截 Unity 的内存映射数据库（最常见）**

360 / 火绒 这类国产安全套件的「主动防御 / HIPS」会拦 Unity DataStore 的 mmap 与文件锁。
证据在 `<工程>/Logs/` 下：

```
Logs/UDS-Service.log : [Error] Failed to open database: 5 : Input/output error
                       [Error] Failed to initialise database
Logs/Editor-UDS.log  : Service connected: false
Editor 日志          : Launching Unity Data Store - .../Unity.DataStore.exe
                       Fatal Error! Failed to initialise UDS client in the Editor
```

**决定性特征**：日志里出现 `Launching Unity Data Store`，但进程表里**从来没有** `Unity.DataStore.exe` 
（服务起不来）→ Editor 无限等它，CPU 接近 0。

```powershell
# 确认机器上是否装了这类套件
Get-Process | Where-Object { $_.ProcessName -match '360|ZhuDongFangYu|HuoRong|QQPCRTP|HipsTray' } |
  Select-Object ProcessName, Id
```

**处理（按代价从低到高）**：
1. 请用户把这几处加入安全软件的**信任区/白名单**（改完不必重启系统）：
   编辑器安装目录（`C:\Program Files\Unity\Hub\Editor\<版本>\Editor`）、工程所在目录、   
   `%LOCALAPPDATA%\Unity`、`%LOCALAPPDATA%\Temp\Unity`；
2. 或请用户**临时关闭杀软的主动防御**后重试；
3. 或请用户用 Unity Hub / Editor **手工打开一次工程**（GUI 启动路径对 UDS 没这么敏感），
   导入完成后 AI 再回到 CLI 跑测试/构建。

**(4) Unity Licensing 客户端卡死**（★ 会同时卡死"编辑器启动"与"batch 出包"，本轮实测踩了近 40 分钟）

症状（`<工程>/Logs/build-*.log` 或 `Editor.log`）：
```
Connection to channel LicenseClient-Dongs refused
Failed to acquire global mutex Unity-LicenseClient-Dongs.
Another instance of Unity.Licensing.Client is already running.
[Licensing::Module] Timed-out after 60.00s, waiting for channel: "LicenseClient-Dongs"
```
表现：batch 构建/编辑器启动**一直卡在 early init**（进程 100~600MB、CPU 几乎不动、日志只在刷上面那几行 60s 一循环）；注意 `unity license status` 可能仍显示「已激活/已登录」——**许可证有效 ≠ 客户端进程健康**。

处理（顺序照做）：
1. 关掉**所有** Unity 编辑器与 **Unity Hub**（Hub 会反复拉起 licensing 客户端，互相抢全局互斥）；
2. 强杀残留 licensing 客户端：`Stop-Process` 常报 **Access denied**（该进程提权），改用 WMI 终止：
   ```powershell
   Get-CimInstance Win32_Process -Filter "Name='Unity.Licensing.Client.exe'" |
     ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate }
   ```
   （返回 `ReturnValue=0` 即成功；Hub 随后会拉一个新的、能正常服务。）
3. 再跑 batch 构建：日志里应出现 `Licensing is initialized (took NN.Ns)` 且**不再**刷 mutex 报错。

> 排查时**别看 `Editor.log` 判断"卡在哪"**：这三种情况（UDS / licensing / 导入）在日志尾部长得都很像，认准关键字：UDS 看 `UDSInterface::UDSInterface`、licensing 看 `Failed to acquire global mutex`、导入看 `[Bee]`/`Import` 步骤在动。

**(2) 工程 / 日志放在点开头的目录**

```
Error: .codebuddy is not a valid directory name. Please make sure there are no unallowed characters in the name.
```

Unity 不接受以 `.` 开头的目录名。**危害不只是「这次失败」**：它会让 UDS 的机器级共享状态进入坏状态，
**之后放在正常路径下的工程也会一起失败**——所以踩过一次之后，把工程挪到正常路径并不能立刻恢复。

**(3) 中途强杀 Editor**

导入被强杀（超时、取消、`Stop-Process`）会写坏 `<工程>/Library/DataStore`。此后每次启动都崩在：

```
UDSInterface::UDSInterface
InitializeAssetDatabaseV2
AssetDatabase::InitializeAssetDatabase
```

**修法**：把 `Library` 整体**挪走**（改名即可，不必删除）让它重建：

```powershell
Move-Item "<工程>/Library" "<工程>/Library-dead-$(Get-Date -Format HHmmss)"
```

**预防**：不要给 Editor 设很短的超时。首次导入（含包解析）几分钟属正常；
需要等待就用**后台进程 + 轮询**，不要用会缓冲输出的前台命令干等。

### 9.3 正确的调用姿势（避免「看起来卡死」）

```powershell
# ❌ 长命令后面不要接 Select-Object -Last N：它会把输出全部缓冲到进程结束，
#    期间一个字都不显示，人和 AI 都会以为程序死了。
# ✅ 后台启动，自己轮询：
$p = Start-Process -FilePath 'unity' `
     -ArgumentList @('run','<工程>','--editor-version','6000.0.x','--timeout','1500','--non-interactive','--no-banner',
                     '--','-nographics','-logFile','<工程>/Logs/editor.log') `
     -RedirectStandardOutput '<工程>/Logs/cli.log' -RedirectStandardError '<工程>/Logs/cli.err' `
     -WindowStyle Hidden -PassThru
"PID=$($p.Id)"
```

随后每隔 1~2 分钟用**短命令**检查四项：进程是否存活 / CPU 是否在涨 / `<工程>/Library` 体积是否在变大 / 日志尾部。

**判断导入真的在推进**：`Library` 体积持续增长（空 3D 模板首次导入通常几分钟）。
体积长时间不变 **且** CPU 不动 = 阻塞，回到 9.2 排查。

> 另注：`unity run` 的 `-quit` / `-batchmode` 是 CLI 自己管理的保留参数，
> 放在 `--` 之后会报 `conflicts with a reserved Unity flag`，去掉即可。

## 10. Unity **编辑器 API** 的坑（写生成器 / 导入器 / 批处理时会撞上）

| # | 坑 | 症状 | 修法 |
|---|---|---|---|
| 1 | `TextureImporter.spriteAlignment` **不在** `TextureImporter` 上 | 直接赋值编译不过 | 走**往返**：`var s = new TextureImporterSettings(); importer.ReadTextureSettings(s); s.spriteAlignment = (int)SpriteAlignment.Custom; s.spritePivot = new Vector2(0.5f, 0f); importer.SetTextureSettings(s);` |
| 2 | `AssetDatabase` **未同步**时 `SaveAsPrefabAsset` 会写出**空壳** | 预制体文件在、`m_Name` 也对，但 `m_Script: {fileID: 0}` ⇒ 运行时 `Component X not found on prefab` ⇒ **面板永远打不开**。成因：同一次批处理里脚本刚编译完，`MonoScript` 还没被 AssetDatabase 导入 | 存之前用 `MonoScript.FromMonoBehaviour` **自检**，解析不到就**报错跳过**（而不是安静地生成一个废预制体）。⚠️ `Refresh(ForceSynchronousImport)` 能修但会**重新导入整个工程**（几百~上千张贴图）⇒ **直接跑到超时**；用轻量 `Refresh()` + 自检 |
| 3 | `ScreenCapture` 属于 `com.unity.modules.screencapture` | manifest 没引这个模块就用不了 | 改用 `Texture2D.ReadPixels` + `EncodeToPNG`（在 Core / ImageConversion 里，通常已引） |
| 4 | 场景里**没有 `AudioListener`** | ① 每秒刷一条 `There are no audio listeners in the scene`；② **所有音效静默** —— 而那条刷屏会把"音效没做"这个**真问题淹掉**（第②条才致命） | 在 `Bootstrap` 里**运行时保证有一个**（比依赖"生成场景时记得挂"可靠） |
| 5 | 批处理模式下 `-executeMethod` **执行完立刻退出** | **进不了播放模式** ⇒ 想驱动 Play 模式只能走 **PlayMode 测试**（`unity test`）或驱动**已打开**的编辑器（见 §8.1/§8.2） | — |

> 写生成器/导入器脚本时，**每个非预期分支都要打日志**：这类失败（空壳预制体、模块缺失、静默音效）
> 的共同点是**编译期全对、运行期静默**。见 `reference/pipeline-and-unity-cli.md` §五（驱动编辑器实测的坑）。
