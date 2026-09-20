# Pipeline 与 Unity CLI —— 踩坑全记录（用到时再查）

> 本文是**能力/踩坑参考**，不是规则。规则原文在 `SKILL.md::§4 ★★★ 做游戏的四道硬前置闸门`（很短，只讲"必须做什么"）。
> 本文讲"为什么、怎么排查、报错长什么样"。**只有真的遇到问题时才需要读它。**

---

## 一、闸门 1：360（及同类国产杀软）必须关闭

**为什么**：360 会拦截 Unity 的安装目录与辅助进程。实测症状是
`Unity.Licensing.Client` **进程在、但通道永远拒绝连接**（其内部在崩），编辑器于是无限重连：

```
[Licensing::IpcConnector] Connection to channel LicenseClient-XXX refused
[Licensing::Module] Timed-out after 60.00s, waiting for channel
Another instance of Unity.Licensing.Client is already running.
```

**每一次重连烧 60 秒且会一直循环**，表现是"创建工程静默卡死"、"跑一小时没动静"。
**这不是慢，是根本起不来。**

**做法**：
- 开工前先问用户一句「**360 关了吗？**」。
- 用户说没关 → **别创建工程**（也创建不了），让他先关，关完再来。
- **不确定 = 按没关处理**，先问，别赌。
- 已经卡住了才发现 → 别硬等，回到这条闸门去问。

---

## 二、闸门 2 的正确节奏（顺序不可换）

```
① AI：创建工程目录 + 写好全部代码 + 写好工程生成器
        ↓
② AI：★【在让用户「添加项目到 Hub、打开编辑器」之前】
        把 com.unity.pipeline 写进 Packages/manifest.json
        （或跑一次 `unity pipeline install`）—— 见下方「P-0」
        ↓
③ AI：才让用户去「添加项目到 Hub + 打开编辑器」，并给出「可直接照做的 Hub 步骤」（见下）
        ↓
④ 用户：打开（这一步**只能用户做**）—— 首次启动即带 Pipeline 服务，无需再重启
        ↓
⑤ AI：驱动那个**活着的编辑器**（unity status / unity command / editor_play）
       —— 编译、生成预制体与场景、进 Play、截图自审
```

---

## 三、★ P-0 铁律：Pipeline 包必须在**让用户添加项目 / 打开编辑器之前**就位

> **踩过的坑（代价：用户被迫重启多次）**：AI 建完工程就直接交给用户打开，
> 等用户开好了才发现要用 Pipeline 驱动，**这时才把包写进 manifest** ——
> 但 Pipeline 的 HTTP 服务**只在编辑器启动 / 域重载时拉起**，
> 对一个**已经跑着的**编辑器**毫无作用**（`unity pipeline list --json` 的 `instances` 恒为 `[]`，
> `unity status` 永远是空表）。于是用户必须**再重启一次** Unity 才连得上。
> 用户重启三次 = 三次都是这个原因。
>
> ⚠️ **但「空表」不只这一个原因** —— 装包时机正确、服务也**确实在跑**，**依然可能看到空表**：
> 那就是下面 **P-1 工作目录**的坑。**先按 P-1 排除，再怀疑装包时机**；
> 否则会把"我目录不对"误判成"包装晚了"，白白让用户重启一次。

**硬性做法（二选一，都必须在"让用户添加项目 / 打开编辑器"之前做完）**：

| 方式 | 做法 | 说明 |
|---|---|---|
| ① 手写 manifest（推荐） | 在 `client/Packages/manifest.json` 的 `dependencies` 里加 `"com.unity.pipeline": "0.7.0-exp.1"`，并补 `"com.unity.modules.screencapture": "1.0.0"` | 与写引擎包同一批做，**零额外步骤** |
| ② CLI 安装 | `unity pipeline install --project-path "<项目根>/client"` | 建完工程立刻跑，同样要在交付前 |

**判定标准**：在**让用户去添加项目 / 打开编辑器之前**，**先自己确认 manifest 里已经有 `com.unity.pipeline`**。
没有命中就别催用户开 —— 开了也白开，他还要再重启一次。

> 前提是本机 `unity` CLI 已装（`Get-Command unity`）。没装就**先装**（见 `reference/unity-cli.md`），
> 再建工程 —— **不要建完才想起来**。

### Hub 添加步骤话术（照抄改路径即可，不许只说"请打开工程"）

> 请把这个工程加到 Unity Hub 并打开（首次导入素材会慢几分钟，属正常）：
> 1. 打开 **Unity Hub** → 左侧「**项目 / Projects**」页
> 2. 点右上角「**添加 / Add**」→「**从磁盘添加项目… / Add project from disk**」
> 3. 在弹出的文件夹选择框里，定位到 **`<项目根绝对路径>`**，然后**选中里面的 `client` 文件夹本身**
>    ——⚠️ **选的是 `client`，不是它的上一级目录**（`client` 才是 Unity 工程根：它有 `Assets/`、`Packages/`、`ProjectSettings/`）
> 4. 点「**选择文件夹 / Select Folder**」，Hub 列表里会出现 `client`
> 5. 点它打开 → 等它导入 + 编译完
> 6. **打开后回我一声**，我继续
>
> 路径（可直接复制）：`<项目根绝对路径>/client`

> 也可以用命令行代劳第 2~4 步（省得用户在文件框里找）：
> `unity projects add "<项目根绝对路径>/client"` + `unity projects pin "<项目根绝对路径>/client"`。
> 但**第 5 步（真的打开）只能用户做**，且打开后要等用户确认。

### 硬性禁止

- ❌ **创建完工程后自己跑 `unity run` / `unity test` / `Unity.exe -batchmode` 去"顺便"编译验证。**
  首次导入几百上千张素材 + 全量脚本编译会把**一次往返拖到十几分钟**；且授权/杀软任一环节出问题
  就是那种**静默卡死**，你会把时间全烧在等它返回上。
- ❌ 用户没打开编辑器时"先跳过实测，把代码交了" —— 那是半成品。
- ❌ **用户不开编辑器，还继续往下做**。正确动作是**停下**，明确说清"我需要你打开 `client/` 文件夹"，然后**不继续**。
- ❌ 用"我自己批处理一下也行"绕过 ③ —— 那正是所有坑的来源。

**为什么必须这样**：批处理模式还有个硬限制 —— `-executeMethod` **执行完立刻退出，进不了播放模式**，
所以"跑一遍看画面"这件事本来就做不了；而用户开着编辑器时 `unity run` 还会直接报
「项目已在运行中的编辑器中打开」。**活编辑器是唯一能实测的路径，且只能由用户开启。**

> **没有例外。** 即使用户说「你就自己跑，别烦我」，也**不做** ——
> 正确动作是把原因讲给他：批处理模式下 `-executeMethod` **执行完立刻退出、进不了播放模式**，
> 自己跑既慢、又验不了画面，只会把时间烧光。
> **用户不打开编辑器 = 这个任务就停在这里，硬气一点。**

---

## 四、★ P-1 铁律：`unity status` 空表 —— **第一嫌疑是工作目录，不是编辑器**

> **踩过的坑（代价：AI 自己白绕半小时）**：闸门 2 完全遵守（包在用户开编辑器**之前**就在 manifest 里），
> 用户也确认"编辑器已打开"（`Temp/UnityLockfile` 存在、`Unity` 进程在跑），
> 但 `unity status` **依然是空表**、`unity pipeline list --json` 的
> `totalInstances / runningInstances / instancesWithPipeline / reachableServers` **全是 0**，
> 并抛出误导性话术：
> `No Unity Editor instances found with reachable Pipeline servers.`
> `Make sure: • Unity Editor is running with a project open • The Pipeline package is installed`
> `• The Pipeline HTTP server is running`
> —— **这三条全是假警报**：编辑器开着、包装着、HTTP 服务也活着。
> 真因只是：**AI 的 shell cwd 不在工程里**（在工程根的上一级、或工作区别的目录跑的命令）。

**根因**：`unity` CLI 的编辑器发现是**相对 cwd** 的 —— 它只去
`<当前目录>/Library/Pipeline/.unity-pipeline-port` 找那个描述文件。
cwd 不在 Unity 工程内 ⇒ **一个都发现不了**（它不会告诉你"目录不对"，只会说"没找到编辑器"）。

**⛔ 硬规定（先做，不是"排查手段"）：每一条 `unity` 命令都必须在「Unity 工程目录」里执行**

> 用户原话（2026-09-19）：「Pipeline 工作目录，**要在你项目里**啊！你在我根目录试鸡毛！」
> —— 起因：AI 在**工作区根**（所有 `clover-*` 项目并列的那一层）跑 `unity`，
> 被 CLI 的「相对 cwd 发现」当成**另一个工程**去找描述文件 ⇒ 行为不可预期。

**唯一合法形态（二选一，没有第三种）**：

```powershell
# ① 切目录（推荐：驱动脚本一律先 Set-Location，写绝对路径）
Set-Location "<项目根>/client"     # 有 Assets/ 与 ProjectSettings/ 的那一层
unity command <name> ...

# ② 不切目录时，**每条**命令显式带 --project-path（不能只带第一条）
unity command <name> --project-path "<项目根>/client"
```

**证据形态（拿 `unity status` 当"编辑器活着"的证据时，必须是这个）**：
- `<项目根>/client/Library/Pipeline/.unity-pipeline-port` 存在且端口可达；
- `unity status --project-path "<项目根>/client"` 返回的**工程路径 = `<项目根>/client`**
  （**不是**工作区根、**不是**别的 `clover-*` 工程）。

⛔ **禁止**：
- 在**工作区根** / **上一级** / **别的项目目录**里跑 `unity`（哪怕只是"看一眼"）；
- 用 `cd ..` 之类的相对跳转后再跑 —— **一律写绝对路径**；
- 把"空表"直接判成"包没装 / 包装晚了" ⇒ **先按本条自查目录，再谈重启编辑器**（P-0 与 P-1 的先后关系见 §三）。

**怎么区分「目录错」和「服务真没起」——不要靠猜**：

| 判据 | 结论 |
|---|---|
| 编辑器菜单 `Window → Pipeline`：**Start Server 是灰的、只有 Stop Server 可点** | 服务**在跑** ⇒ 问题在**发现**（多半是 cwd），不在服务 |
| 同上，但 **Start 可点** | 服务确实没起 ⇒ 点它（`Window/Pipeline/Start Server`），Console 会打 `Pipeline Server started on port xxxxx` |
| 工程 `Library/Pipeline/.unity-pipeline-port` 文件存在 | 服务写过描述文件 ⇒ 基本可判定它在跑 |

**要铁证就直接问服务**（绕过 CLI 的发现逻辑）：

```powershell
$d = Get-Content "<项目根>/client/Library/Pipeline/.unity-pipeline-port" -Raw | ConvertFrom-Json
Invoke-WebRequest -Uri "http://127.0.0.1:$($d.port)/api/status" `
  -Headers @{ Authorization = "Bearer $($d.evalToken)" } -UseBasicParsing
# → {"status":"ready","lastHeartbeat":"…","capabilities":[…]}   服务一直是健康的
```

> ⚠️ 描述文件里的 `lastHeartbeat` **可能是陈旧的**（停在启动那一刻）——
> **它不刷新 ≠ 服务死了**，别拿它当死亡判据。

**该端口上的端点**（都在 `127.0.0.1`，全都要 `Authorization: Bearer <evalToken>`）：
`/api/status`、`/api/editor_status`、`/api/commands`（列出全部命令，通常上百条）、
`/api/exec`（**POST**，真正执行命令）、`/api/dialog`、`/api/progress`、`/api/job`。
—— CLI 走的就是这些；**CLI 发现不了时，直接打 HTTP 一样能驱动编辑器**。

> **排查顺序（照这个来，别跳）**：`cd` 进工程 → `unity status` → 还空表？
> 看 `Window → Pipeline` 菜单灰不灰 → 还不行？读 `.unity-pipeline-port` 打 `/api/status`。
> **明确讲给用户听**："这不是包装晚了，不用重启"。

---

## 五、★ P-2 铁律：驱动编辑器做「实测」时必踩的 5 个坑

> 这几条全是**「代码没问题、但你以为有问题」**那一类 —— 共同特征是**没有任何编译错误**，
> 却让你在错误的方向上查很久。按下面的顺序自查，能省掉大部分时间。

### ① 编辑器窗口失焦 → 帧循环【完全停住】（最坑的一条）

- **症状**：启动画面的 1.8 秒定时器永不触发；流程卡在 Boot / Menu 不再推进；
  日志停在某一条之后再无输出；`Time.frameCount` **跑了两分钟只有 2**。
  看起来完全像"状态机坏了""场景加载回调丢了"，**实际游戏代码一行都没错**。
- **判据**：`eval_file` 打印 `UnityEngine.Time.frameCount`，隔 10 秒再打一次 ——
  数字没变 ⇒ 就是它（正常应该是几百上千）。
- **对策（根治，别只在验证时临时开）**：游戏侧在 `Awake` 里设
  `Application.runInBackground = true;`。临时用 eval 打开的那个值**域重载就没了**，
  下一次重编译后又开始"卡死"，会让人误判成回归。

### ② 窗口失焦 → Unity 不会自动扫到外部文件改动 → `recompile` 误报「无需编译」

- **症状**：改了 `.cs`（或新生成文件）后，`unity command recompile` 回
  `{"status":"up_to_date","message":"No scripts needed recompilation."}`，
  但 `Library/ScriptAssemblies/*.dll` 的**时间戳还是旧的**。
- **判据**：**比对 dll 时间戳 vs 源码时间戳**。
  不要信 `recompile_status` —— 它报的是"**上次**编译结果"，不是"有没有待编译改动"，
  在"有改动但没触发编译"时它一样回 `failed:false`，极具误导性。
- **对策**：先刷资产库，**再**编译，顺序不能反：
  ```powershell
  # 1) probe.cs（eval_file 执行，内容是方法体语句）
  #    UnityEditor.AssetDatabase.Refresh(UnityEditor.ImportAssetOptions.ForceUpdate);
  unity command eval_file --file "<工程>/client/probe.cs"
  # 2) 然后才编译
  unity command recompile
  ```

### ③ `recompile` 回 "Network error" —— 十有八九是正常的

- 编译会触发**域重载**，域重载会把 Pipeline 的 HTTP 连接掐断，于是 CLI 报
  `Failed to execute command 'recompile': Network error: An error occurred while sending the request.`。
- **判据**：看 `ScriptAssemblies/*.dll` 时间戳有没有更新。更新了 = 编译成功，那条报错直接忽略。
- 同理：**Play 模式会因为域重载而自动退出**（日志里有 `Loaded scene 'Temp/__Backupscenes/0.backup'`
  就是退出 Play 后的场景还原）。发现 `playMode: stopped` 别慌，重新 `editor_play` 即可。

### ④ `capture_game_view --source screen` 只在 Play 模式可用

- 编辑模式下会回 400：
  `capture_game_view with source="screen" requires Play Mode:
   Screen Space - Overlay UI only composites to the backbuffer at runtime. Use source="camera" in Edit Mode.`
- 所以**拍 UI / 拍游戏画面必须在 Play 里做**；编辑模式想截图就用 `--source camera`。

### ⑤ `clear_console` 清的是【编辑器控制台窗口】，日志文件里的历史行仍在

**别用 `clear_console` + grep 日志文件来判断"这次编译过没过"** —— 文件是追加写的，
上一轮甚至上上轮的错误行都还在，`Select-Object -Last N` 抓到的很可能全是历史行，
于是**明明已经修好、却看起来还在失败**（实测因此白绕一轮）。

**判据只用权威状态**：`unity command recompile_status --format json` 的
`failed` / `errors` / `compilationFailed` 三个字段；再配合 `Library/ScriptAssemblies/*.dll`
的时间戳（**编译失败时 Unity 不会更新 dll**，所以时间戳变了就等于这次编译成功了）。

### ⑥ `eval_file` 的代码是【方法体】，不是完整类

- 只能写**语句**：不能有 `using` / `class` / 方法声明；结尾要给一个 `return` 值。
- 想用 `UnityEngine` 之类就直接写全名（包装器会补 using）。
- **长代码一律走 `eval_file` 读文件**，不要内联塞进 `eval`：内联字符串里的
  空格会让 CLI 把它拆成多个位置参数，报 `eval 的参数无效：...没有对应的位置`，
  而且报错信息看起来像"你的 JSON 写错了"，其实是命令行被拆了。
- 探针文件放在**项目根**（如 `<工程>/client/probe.cs`）即可，**不要放进 `Assets/`** ——
  放进 Assets 会被 Unity 当资产导入并参与编译，一个语法错就让整个工程进 Safe Mode。

---

## 六、★ P-3 铁律：第二次进 Play 就是一片蓝 —— 查「快速进入 Play 模式」

> **症状**：第一次进 Play 一切正常（启动画面 → 菜单 → 关卡）；**停掉再进第二次，
> 整个屏幕只剩相机的纯色背景**（比如一片天蓝），UI 一个字都没有，日志里连报错都没有。
> 用户会直接说"你这菜单根本就是全蓝的"，而你去跑一次又是好的 —— 典型的
> **"只有非首次运行才复现"**。
>
> **真因**：工程开了 **Enter Play Mode Options / 不重载域**
> （`ProjectSettings/EditorSettings.asset` 里 `m_EnterPlayModeOptions: 1`，1 = DisableDomainReload）。
> 域不重载 ⇒ **所有 `static` 字段跨 Play 局残留**。本项目里正好有两道静态闸门：
> ① 自己的 `Bootstrap._launched` 还是 `true` → 新一局的 Bootstrap `Destroy(gameObject)` 自毁，
>    什么都不初始化；
> ② 引擎 `Game.Launch` 里的 `if (IsRunning) { LogWarning("Already running"); return; }`
>    —— `IsRunning` 也是静态的，同样残留，引擎不会重新初始化。
> 两条叠加 = 没有 UI、没有流程、只有相机底色。**而且它不报错**。
>
> **判据**（一条命令定位）：
> ```powershell
> Select-String -Path ProjectSettings\EditorSettings.asset -Pattern 'EnterPlayMode'
> # m_EnterPlayModeOptionsEnabled: 1 且 m_EnterPlayModeOptions: 1  ← 就是它
> ```
> 运行时判据：`eval_file` 打印 `Game.Fsm.Current`，如果第二次进 Play 直接是上一局的终态
> （比如 `GameOver`）、而不是 `Boot`，那就是静态状态残留，不是"菜单没做"。
>
> **对策（二选一，按你能不能改引擎）**：
> - **能改工程设置（推荐）**：关掉快速进入模式。用编辑器 API 改比手改文件可靠
>   （手改 `.asset` 会被 Unity 覆写）：
>   ```csharp
>   // _dev/fix.cs（eval_file）
>   UnityEditor.EditorSettings.enterPlayModeOptionsEnabled = false;
>   UnityEditor.AssetDatabase.SaveAssets();
>   UnityEditor.EditorApplication.ExecuteMenuItem("File/Save Project");
>   ```
>   落盘后 `m_EnterPlayModeOptions` 应变成 `0`。
> - **不能改设置**：每个静态状态都要自己复位 ——
>   ```csharp
>   [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.SubsystemRegistration)]
>   static void ResetStatics() => _launched = false;
>   ```
>   但这只解决你自己那份；**引擎侧的静态闸门你改不了**，所以这条通常救不回来。

> **高危连带坑：输入注入的两个反面**（一个症状是"乱响应"，另一个是"完全不响应"）。
> 每次 `InputSystem.AddDevice<Keyboard>()` 都会留下一个设备；实测累积到 **23 个**，
> 其中残留的按键按下状态会污染引擎读数（表现为"没注入跳却在跳 / 原地反复跳 / 掉穿地面"），
> 极易被误判成游戏 bug。
>
> ⚠️ **但"注入前先移除所有非真实键盘设备"这条旧建议只对了一半，照做会掉进更深的坑**：
> `RemoveDevice()` 之后 `Keyboard.current` **仍然指向那个已销毁的对象**，于是
> ① 往它身上投事件等于投进黑洞（注入**完全没反应**）；
> ② 访问 `kb.name` 会直接抛 `NullReferenceException` —— 这个 NRE 会把人引到完全无关的方向。
>
> → 完整结论见下面 **【P-4】**（三个坑 + 定位手法 + 可靠注入写法）。**写注入脚本前先读它。**

---

## 六之二、★ P-4 铁律：按键注入（**已解决 —— 照抄这套，别再自己发明**）

> **症状**：用 `eval_file` 往 InputSystem 队列里投按键（`QueueStateEvent`），
> **日志一切正常**（Timer 回调执行了、自己的日志打了、`fsm` 也对），**但游戏就是不响应**。
> 实测为此绕了二十多轮，其中**一半时间浪费在一个错误判据上**（见下方 ⛔）。

### ✅ 正确做法（已验证：注入后菜单从 `Menu` 推进到 `CharSelect`）

```csharp
// ① 两个设置都要改（只在探针脚本里改，不要写进工程设置）
var s = UnityEngine.InputSystem.InputSystem.settings;
s.backgroundBehavior = UnityEngine.InputSystem.InputSettings.BackgroundBehavior.IgnoreFocus;
s.editorInputBehaviorInPlayMode =
    UnityEngine.InputSystem.InputSettings.EditorInputBehaviorInPlayMode.AllDeviceInputAlwaysGoesToGameView;

// ② 清掉所有【残留】的自建键盘（不能省，理由见下面 ②/③ 号坑）
foreach (var d in UnityEngine.InputSystem.InputSystem.devices.ToArray())
    if (d is UnityEngine.InputSystem.Keyboard k && k.name != "Keyboard")
        UnityEngine.InputSystem.InputSystem.RemoveDevice(k);

// ③ 新建一条键盘并投状态（新建后它就是 Keyboard.current）
var kb = UnityEngine.InputSystem.InputSystem.AddDevice<UnityEngine.InputSystem.Keyboard>();
UnityEngine.InputSystem.InputSystem.QueueStateEvent(kb,
    new UnityEngine.InputSystem.LowLevel.KeyboardState(UnityEngine.InputSystem.Key.Space));  // 按下
// 松开：同样一句，换成无参的 new KeyboardState()

// ④ 【不要】手动调 InputSystem.Update()（没有带参重载；默认模式下无参调用还会 NRE）。
//    交给播放循环自己那一帧 Dynamic 更新去处理就行。
```

### ⛔ 判定"注入到底有没有生效"的唯一正确方式

**看游戏自己的反应** —— 流程日志（`[Flow] →` 有没有推进）、角色位移、HUD 数值变化。

**绝对不要**在 `eval_file` 里读 `kb.spaceKey.isPressed` 或引擎 `Game.Input.GetKey(...)` 来判定：
`eval_file` 跑在**编辑器更新**上下文里，读的是**编辑器那一套状态缓冲**；游戏跑在**播放循环**里，
读的是 **Dynamic 缓冲**。**这是两套独立的状态**，编辑器里读到 `False` **完全不代表游戏没收到**。

> **代价**：我在这上面绕了二十轮 —— 每次注入后都在 eval 里读 `isPressed` 得到 `False`，
> 于是断定"注入完全无效"，接着去改设置、换设备、硬调 `Update()`……全是白费，
> 而**游戏其实早就收到了**。**判据选错，比不测更糟。**
>
> 同理：`InputState.Change(control, value)` 对按键这类**位域控件**会直接抛
> `Cannot change state of bitfield control` —— 别指望用它绕过去。

### 为什么前面会踩（每条的判据/对策）
> **三个坑（叠在一起，任一个都能让注入失效）**：
>
> **① 窗口失焦 → 设备被禁用 → 状态被直接丢弃**（AI 驱动时**永远**是失焦的）
> `InputSystem.settings.backgroundBehavior` 默认是
> `ResetAndDisableNonBackgroundDevices`：编辑器窗口一失焦，设备就被禁用，
> `QueueStateEvent` 投进去的状态**被丢掉**。
> 判据：日志里带上 `Application.isFocused`（会是 `false`）。
> 对策（**只在探针脚本里设，不要改工程设置**）：
> ```csharp
> UnityEngine.InputSystem.InputSystem.settings.backgroundBehavior =
>     UnityEngine.InputSystem.InputSettings.BackgroundBehavior.IgnoreFocus;
> ```
>
> **② `Keyboard.current` 可能是【已被 RemoveDevice 掉的死对象】**
> 见上一节那个"高危连带坑"。往它身上投事件 = 投进黑洞；读 `.name` 还会 NRE。
> 对策：**先把残留的全 RemoveDevice 掉，再 AddDevice 一条新的**（它就是 `current`）。
>
> **③ 残留设备必须清、不要"复用"**（这条早先写反了，已订正）
> 累积的自建键盘会互相干扰：残留设备可能带着"按下未松开"的状态，
> 也可能让 `Keyboard.current` 指向一个已销毁/已失效的对象。
> **正确做法就是上面 ✅ 的那句：清空所有非 OS 键盘 → 新建一条 → 只往这条投。**
> 不存在"复用更稳"这回事。
>
> **④ ⚠️ 绝对不要动 `InputSystem.settings.updateMode`**（这条是我自己炸的，代价最大）
> 排查时我把它设成 `ProcessEventsManually` 想"手动驱动输入"，结果：
> **`CloverEngine.Game.Input` 直接变成 `null`**，整个游戏收不到任何输入，
> 于是又冒出一个"看起来完全无关"的新故障（菜单、跳跃、暂停全都失灵），
> 而报错为零、日志正常 —— 差点被当成"引擎坏了"。
>
> 它改的是 `InputSystem` 的全局设置 ScriptableObject，**在同一个 Play 会话内一直生效**；
> 本工程没有 `.inputsettings` 资产，所以重进 Play / 域重载会恢复默认
> （判据：`Get-ChildItem -Recurse -Include *.inputsettings*` 为空 ⇒ 我的改动没落盘）。
> **但"没落盘"是靠不住的保险 —— 有的工程就是有这个资产。**
>
> **硬规矩：探针只许改 `backgroundBehavior` 和 `editorInputBehaviorInPlayMode` 这两项
> （且每次注入前都显式设一次），`updateMode` 碰都不许碰。**
> 排查完顺手把这两个也还原成默认值，别留给下一次。
>

> **附带两个同样误导的坑**：
>
> - **`GameObject.Find("XxxPanel")` 查 UI 面板永远查不到** —— 实例名带 `(Clone)`。
>   实测因此误判成"面板没开、所以没人听按键"，白查一轮。
>   要么查 `"XxxPanel(Clone)"`，要么用子串扫描 `FindObjectsByType<GameObject>` 判 `name.Contains(...)`。
> - **只是想让游戏"到达某个状态"来做验证时，不要硬走按键。**
>   直接发工程自己的事件（`Game.Event.Emit(Events.StartNewGame)` /
>   `Emit<int>(Events.CharChosen, 1)`）一步到关卡：稳定、不受上述三个坑影响，
>   也不影响"逻辑正确性"这个结论。**按键注入只用于专门验证输入链路本身。**

---

## 六之三、★ P-5 铁律：Editor 脚本弹**原生模态框** ⇒ 编辑器**静默卡死**、之后所有命令都超时

> **症状（极具误导性；实测为此绕了很多轮）**：AI 用 Pipeline 让编辑器跑一个耗时的 Editor 脚本
> （场景 / 资产生成器）之后，**所有** `mainThreadRequired` 命令都超时（连 `editor_status` 都超时）。
> 与此同时，各项自查**全是绿的**，于是排查被引到完全错误的方向：

| 自查项 | 读数 | 误导出来的结论 |
|---|---|---|
| `/api/status` | `ready`，**心跳还是新鲜的** | "服务活着 ⇒ 肯定是 CLI 没连上" |
| `/api/dialog` | `active:false`、`dialogs:[]` | "没有对话框挡着" |
| `/api/progress` | `active:false` | "没有长任务在跑" |
| `unity status` / `pipeline list` | 空表 | "编辑器没开 / Pipeline 包装晚了" |
| 进程 `Responding` | `True` | "编辑器没死" |
| 进程 `CPU`（两次采样） | **差值 = 0** | 被读成"脚本很慢，再等等" |

> **真因**：Editor 脚本里调用了**会弹原生模态对话框**的 API。
> 最典型的是 `EditorSceneManager.SaveCurrentModifiedScenesIfUserWantsTo()`
> （弹 **"Scene(s) Have Been Modified" → 再弹 "Save Scene"**）；同类还有 `EditorUtility.DisplayDialog*`、
> 模态 `EditorWindow`、资源冲突对话框等。
> 模态框**阻塞编辑器主线程** ⇒ Pipeline 的 Dispatcher 工作项**永远不返回** ⇒ 之后所有命令排队超时。
> 而 **Pipeline 看不见原生框**（`/api/dialog` 只报它自己包装的那类）⇒ 自查全绿、无一条报错。
>
> **为什么"CPU 差值 0"是分水岭**：脚本真在跑会吃 CPU；**CPU 不动却命令不通 = 等待式阻塞**
> （模态框 / 锁），不是在计算。**判据选错，比不测更糟**（会一直在"再等等"里耗）。

**判据（一条命令区分"卡死"与"慢"）**：

```powershell
Get-Process -Id <unityPid> | Select-Object CPU, Responding, MainWindowTitle
Start-Sleep -Seconds 15
Get-Process -Id <unityPid> | Select-Object CPU, Responding, MainWindowTitle
```
**CPU 差值 = 0 且命令不通 ⇒ 按"卡在模态框"处理**，不要再等。

**铁证（不问 CLI、不问 Pipeline，直接问 Windows）**：用 Win32 `EnumWindows` 枚举该进程的顶层窗口：

- 命中一个 **类名 `#32770`**（Windows 标准对话框类）、标题形如 `Scene(s) Have Been Modified` / `Save Scene` 的窗口；
- 与此同时主窗口（Unity 的 `UnityContainerWndClass`）会变成 **`Enabled = False`** ——
  这一条就是"存在模态子窗口"的铁证。

**处置（AI 自己就能点掉，不必麻烦用户）**：

| 目标 | 做法 |
|---|---|
| 点某个按钮 | `EnumChildWindows` 找按钮文本（`Don't Save` / `取消` / …）→ 给按钮发 **`BM_CLICK` (0x00F5)** |
| 整体取消 | 直接给**对话框窗口**发 **`WM_CLOSE` (0x0010)**（等价 Cancel） |

⚠️ **点 "Save" 而当前场景又没有路径 ⇒ 立刻再弹一个"另存为"文件对话框**，里面是
`DUIViewWndClassName` / `DirectUIHWND` 那一套 —— **又卡一次**。要么别点 Save，要么准备好**连点两次**。
点掉后 `editor_status` **立刻**恢复，无需重启编辑器。

**根治（已升格为规则层硬约束，见 `SKILL.md` §4 闸门 2）**：

- ⛔ **Editor 脚本（生成器 / 构建器 / 批量工具）在自动化路径上不许调用任何会弹原生模态框的 API**；
- 需要处理"当前场景可能有未保存修改"时：**静默处理 + 打一条 Warn 日志**，
  不许调 `SaveCurrentModifiedScenesIfUserWantsTo()`。
  `EditorSceneManager.NewScene(..., NewSceneMode.Single)` **本身不弹框**，直接用它覆盖即可。

**同一轮实测顺带钉死的四条（省得下次再绕）**：

1. **`unity status` / `unity pipeline list` 空表 ≠ 连不上**：先 `cd` 进工程，或显式 `--project-path`；
   仍空表就**直接读 `Library/Pipeline/.unity-pipeline-port`、打 HTTP `/api/status` 当铁证**。
   某些 CLI 版本与 Package 版本的组合下**实例枚举坏了，但 `unity command <name> --project-path <p>` 完全可用**
   —— **不要因为 status 空表就判定"连不上"、就重启编辑器或去麻烦用户**。
2. `unity command` 的 **`--timeout` 单位是「秒」**（默认 30），而且**只影响 CLI 侧等待**，
   **不会**放宽服务端命令自身的预算；另有 **`--detach`**（提交为分离作业、立刻返回 job id）。
3. **`eval_file` 的服务端预算只有 5 秒**（`timeout` 默认 5000ms），而且**超时会中断**正在跑的主线程代码
   （不是"继续跑完"）⇒ **长任务不能用 `eval_file`**：走 `menu`（CLI 侧默认 30 秒、可 `--timeout` 放大），
   或把工作**切片 / 改成增量**，让每片都落在预算内。
4. **`GET /api/commands` 能拿到每条命令的 JSON schema**（参数名 / 类型 / 默认值）—— **用它，别猜参数名**。
   `/api/exec` 对**带参命令**的 body 形态很难猜（`args` / `parameters` / `arguments` / `params` / 平铺
   全试过 = 400）⇒ **别在 HTTP 上纠缠，用 CLI `unity command`**；真正好用是那几个 **GET 端点**：
   `/api/status`、`/api/dialog`、`/api/progress`、`/api/commands`。

---

## 七、Unity 6 + Unity CLI（AI 自动化操作的强制前置条件）

> 这是对 **AI** 的要求，不是对项目 Unity 版本的要求。只要 AI 要自动化操作/验证客户端代码，就必须走这套。

- AI 自动化操作 Unity 一律 **Unity 6（6000.x）+ Unity CLI（`unity` 命令）**，禁止直接拼 `Unity.exe -batchmode -executeMethod` 当首选手段。
- 检测到没有 CLI（PowerShell：`Get-Command unity` 为空）**不要问用户，直接装**：
  ```powershell
  $env:UNITY_CLI_CHANNEL='beta'; irm https://public-cdn.cloud.unity3d.com/hub/prod/cli/install.ps1 | iex
  ```
  装完重启终端 / 新开 shell 后重新自检通过再继续（macOS/Linux 用 `install.sh`）。
- 给 AI 解析的输出一律加 `--format json`。
- ★ **`unity status` 返回空表时，第一件事是 `cd` 进工程目录**（见上文 P-1）。
- 详见 `reference/unity-cli.md`。

## 八、★ 驱动工具链时 **PowerShell 本身**的坑（每条都踩过 ≥1 次）

> 这些坑的症状都很"玄学"（莫名的结果、莫名的类型错），但成因都在这张表里。
> 用别的 shell 可以跳过本章，但**在 Windows 上驱动 Unity / 处理素材**时躲不开。

| # | 坑 | 症状 | 规矩 |
|---|---|---|---|
| 1 | 变量名**大小写不敏感** —— `$Out` 和 `$out` 是**同一个**变量 | 先 `param([string]$Out)` 声明输出路径，后面又 `$out = New-Object ...` ⇒ **把参数覆盖了** ⇒ 报 `[System.String] 不包含名为 "Add" 的方法`、`Set-Content 参数 Path 为空字符串` 这类完全指错方向的错 | `$out` / `$input` / `$args` / `$pid` / `$home` / `$error` / `$host` 这类名字**一律别用**（`$pid` 还是**只读自动变量**，赋值直接报错） |
| 2 | `$Matches` 会被**下一个**正则覆盖 | 一个条件里连用两个 `-match`，后面用 `$Matches[2]` 时它已经是**第二个**正则的捕获组 ⇒ 解析出的字典"条目数对、内容全空" ⇒ 下游全部查不到 | **一次 `-match` 立刻把 `$Matches` 落到局部变量**再往下走；更稳的是**不在一个条件里连用两个 `-match`** |
| 3 | `New-Object T(a, b)` **不是**构造函数调用 | 参数会被当成**一个数组**传，或类型不对时**返回 null 且不报错**（隐藏成因：`[math]::Ceiling()` 返回 `double`，`Bitmap(double,double)` 没有重载 ⇒ null）⇒ 后面 `$bmp.Save(...)` 报"无法对 Null 值表达式调用方法" | 一律用 `[Type]::new(...)`，并且**显式 `[int]` 转换** |
| 4 | PowerShell 5.1 按 **ANSI** 读 `.ps1` | 脚本里写中文注释/字符串（尤其 `→`、`★` 这类符号）会把字符串**截断** ⇒ 报一堆莫名的"缺少终止符" | **`.ps1` 内部只用 ASCII**；要输出中文就让它**从数据文件里读** |
| 5 | `Set-Content` 改文件有**编码风险** | 中文变乱码，或直接被工具拦下 | 优先用编辑工具（`replace_in_file`）；非要脚本写就用 `[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))` |
| 6 | `Get-Content` **不带编码** | Windows 上可能按 GBK 解码 UTF-8 文本 ⇒ 中文乱码/匹配不上 | 读日志用 `Select-String`（自带编码探测）或显式 `-Encoding UTF8`；大文件用 `search_content` |
| 7 | 长命令后面接 `Select-Object -Last N` | 它会把输出**全部缓冲到进程结束**，期间一个字都不显示 ⇒ 人和 AI 都会以为程序死了（另见 `unity-cli.md` §9.3） | 长命令**后台启动 + 自己轮询** |
