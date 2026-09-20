# 启动与流程编排（App Flow）：启动 → 菜单 → 创角 → 进图 → 暂停 → 回菜单

> **这是「完整成品」的第一段，也是最容易整段缺失的一段。**
> **世界上没有一个完整游戏是"打开就直接站在游戏场景里"的** —— 缺了启动与菜单链路，玩家第一眼就判定是半成品。
> 本范式给出「**状态机（`Game.Fsm`）+ 面板（`Game.UI`）+ 场景（`Game.Scene`）**」的编排写法；API 均以 `clover-client-unity-engine/Runtime/**` 为准。

---

## 0. 硬规则（违反任一条即交付失败）

| # | 规则 |
|---|---|
| 1 | **必须有启动链路**：启动画面 / Logo → 主菜单 → 创角（或选角）→ **读条** → 游戏场景。**禁止**应用一起来就直接进游戏场景 |
| 2 | **每个界面都要"能进能出"**：有入口、有返回路径（取消 / 返回 / ESC），**禁止单向死路** |
| 3 | **主菜单每一项都要真能用**：开始 / 继续 / 设置 / 退出（联机再加登录 / 注册）。不许摆一个点了没反应的按钮 |
| 4 | **创角 / 选角要真的通服务端**：建角 → 服务端回包 → 用**自己建的角色**进图（不许本地造一个假角色） |
| 5 | **读条要真读条**：走 `Game.Scene.Load(name, onProgress, onDone)`，进度真的在跳（不是黑屏、不是瞬间硬切、不是假进度条） |
| 6 | **游戏内流程要做**：暂停菜单（继续 / 设置 / 回主菜单 / 退出）；结算界面；**回主菜单后能再进一次**（第二次进入无残留状态） |
| 7 | **回菜单必须清场**：面板 / 实体 / 对象池 / 事件订阅 / 定时器 / 网络订阅全部收干净（见 §5），否则第二次进游戏是脏的 |
| 8 | **UI 场景与游戏场景分离**：主菜单/创角在**纯 UI 场景**里，进图时才加载游戏场景；**禁止**把主菜单塞进游戏场景 |
| 9 | 每条非预期分支都要打日志（`Game.Logger.Info/Warn/Error(tag, msg)`，见 `SKILL.md`「错误处理与日志」） |

> 参考游戏有**更细的站点**（如"选阵营 / 买枪 / 关卡选择 / 难度选择 / 存档槽 / 成就"）时，**按原版加站点**，不要只做上面 6 个就交差 —— 站点清单以 `patterns/game-demo.md` §0.0c 的**原版系统清单**为准。

---

## 1. 流程骨架（先定这张表，再写代码）

站点顺序（单机与联机只差"登录/注册"这一段）：

```text
Boot(启动画面) → MainMenu(主菜单) → [Login/Register(联机)]
                                  → CharSelect(选角) / CharCreate(创角)
                                  → Loading(读条) → Stage(游戏场景)
                                                      ├─ Pause(暂停菜单) → Settings / MainMenu / Quit
                                                      └─ Result(结算)   → MainMenu / 再来一局
```

要落到"状态机 + 面板 + 场景"三元组：

| 站点 | 状态机状态 | 面板（`Resources/UI/{类名}`） | Unity 场景 | 必做内容 |
|---|---|---|---|---|
| 启动画面 | `Boot` | `BootPanel` | `Boot` | Logo / 版权字 / 首次进入的"点击开始"；初始化（配置、热更、登录前置） |
| 主菜单 | `MainMenu` | `MainMenuPanel` | `Menu` | 开始（新游戏）/ 继续（有存档时）/ 设置 / 退出；联机时：登录 / 注册入口 |
| 设置 | `MainMenu`（子面板） | `SettingsPanel` | `Menu` | **真能改**：音量（`Game.Sound`）、画质（`Game.Quality`）、分辨率/全屏；改完能存（`Game.Setting`） |
| 登录 / 注册 | `Login` | `LoginPanel` / `RegisterPanel` | `Menu` | 账号服换 token → `EMsg.Login` → `SetupSession`；失败要有提示与重试 |
| 选角 | `CharSelect` | `CharSelectPanel` | `Menu` | 列出已有角色、选中、进入、删除（有二次确认弹窗） |
| 创角 | `CharCreate` | `CharCreatePanel` | `Menu` | 名字 / 外观 / 职业 / 属性预览 → 提交 → 服务端回包 → 进选角或直接进图 |
| 读条 | `Loading` | `LoadingPanel` | 切换中 | 真实进度 + 提示文案；**加载完成再关面板** |
| 游戏内 | `Stage` | `HudPanel`（+ 玩法面板） | `Stage01`（≥1 个游戏场景） | 见 `patterns/game-demo.md` 首场景全部内容 |
| 暂停 | `Pause` | `PausePanel` | `Stage01` | 继续 / 设置 / 回主菜单（二次确认）/ 退出 |
| 结算 | `Stage`（子面板） | `ResultPanel` | `Stage01` | 胜负 / 数值 / 再来一局 / 回主菜单 |

> **`Game.Scene` ≠ `Game.CloverScene`**：前者是 Unity 关卡（本文件讲的），后者是**服务端场景投影**（`EMsg.PushSceneInfo`）。两者不同义，别混用。

---

## 2. 分层落点（`reference/architecture.md` 的强制分层）

```text
client/Assets/Scripts/
├── Core/ClientConfig.cs        # 配置（地址/超时…）—— 见 patterns/client/config.md
├── Core/Events.cs              # ★ 流程事件名常量（唯一来源，禁止裸字符串）
├── Module/Flow/                # ★ 流程编排模块：AppFlow（唯对外门面 IAppFlow）
├── UI/Panels/                  # 各站点面板（只发/收事件，不引用 Module）
└── App/Bootstrap.cs            # 唯一组装点：Game.Launch + Init + Flow.Enter()
```

- **状态切谁的**：`Game.Fsm` 只标记"我在哪个站点"，**面板与场景的开关写在 Flow 模块里**（`RegisterState` 的 `onEnter/onExit`）；
- **UI 不许 `using Module`**：面板只 `Game.Event.Emit(Events.Xxx)` / `Game.UI.Close<T>()`，Flow 订阅后驱动；
- **网络调用只允许出现在 `Module/Flow`（登录/建角）与 `App`**，其它模块一律走门面；
- **事件名必须来自 `Core/Events.cs`**，禁止裸字符串（`reference/architecture.md` 自检 ⑤）。

---

## 3. 代码模板

### 模板 1：`Core/Events.cs`（流程事件名常量，唯一来源）

```csharp
namespace {Name}.Core
{
    /// <summary>流程/界面事件名常量。禁止在业务里写裸字符串。</summary>
    public static class Events
    {
        // 主菜单 → Flow
        public const string StartNewGame = "Flow.StartNewGame";
        public const string ContinueGame  = "Flow.ContinueGame";
        public const string OpenSettings  = "Flow.OpenSettings";
        public const string QuitGame      = "Flow.QuitGame";

        // 登录/创角/选角 → Flow
        public const string LoginSubmit   = "Flow.LoginSubmit";
        public const string CreateSubmit  = "Flow.CreateSubmit";   // 参数：CreateCharArgs
        public const string CharChosen    = "Flow.CharChosen";     // 参数：long playerID
        public const string CharDeleted   = "Flow.CharDeleted";

        // 游戏内 → Flow
        public const string PauseOpened   = "Flow.PauseOpened";
        public const string Resume        = "Flow.Resume";
        public const string BackToMain    = "Flow.BackToMain";
        public const string StageFinished = "Flow.StageFinished";  // 参数：ResultArgs
    }
}
```

### 模板 2：`Module/Flow/AppFlow.cs`（流程门面 + 状态机 + 场景/面板编排）

```csharp
using {Name}.Core;
using CloverEngine;
using UnityEngine;

namespace {Name}.Module.Flow
{
    public interface IAppFlow
    {
        void Enter();                        // Boot → MainMenu（应用起来后调它）
        string CurrentState { get; }
    }

    /// <summary>启动与菜单流程编排：状态机只标站点，面板与场景的开关都在这里。</summary>
    internal sealed class AppFlow : IAppFlow
    {
        private const string SceneBoot   = "Boot";
        private const string SceneMenu   = "Menu";
        private const string SceneStage1 = "Stage01";

        public string CurrentState => Game.Fsm.Current;

        public void Enter()
        {
            RegisterStates();
            Game.Event.Emit(Events.StartNewGame);   // 或改成先停在主菜单，等玩家点"开始"
            Game.Fsm.Force("Boot");
            Game.Fsm.Trigger("BootDone");           // → MainMenu
        }

        private void RegisterStates()
        {
            // 站点：引擎在 Launch 时已预注册 Launching/CheckingUpdate/Logging/MainCity/Battle/Disconnected，
            // 运行时不自动驱动，业务可同名覆盖或另起状态名。这里用业务自己的站点名，避免与引擎语义打架。
            Game.Fsm.RegisterState("Boot",     onEnter: () => ShowBoot(),     onExit: () => Game.UI.Close<BootPanel>());
            Game.Fsm.RegisterState("MainMenu", onEnter: () => ShowMainMenu(), onExit: () => Game.UI.Close<MainMenuPanel>());
            Game.Fsm.RegisterState("Login",    onEnter: () => Game.UI.Open<LoginPanel>());
            Game.Fsm.RegisterState("CharSelect", onEnter: () => EnterCharSelect());
            Game.Fsm.RegisterState("CharCreate", onEnter: () => Game.UI.Open<CharCreatePanel>());
            Game.Fsm.RegisterState("Loading",  onEnter: () => { /* 由 GoStage 打开 LoadingPanel */ });
            Game.Fsm.RegisterState("Stage",    onEnter: () => { }, onExit: () => LeaveStage());
            Game.Fsm.RegisterState("Pause",    onEnter: () => Game.UI.Open<PausePanel>(), onExit: () => Game.UI.Close<PausePanel>());

            Game.Fsm.AddTransition("BootDone",   "MainMenu");
            Game.Fsm.AddTransition("NeedLogin",  "Login");
            Game.Fsm.AddTransition("LoggedIn",   "CharSelect");
            Game.Fsm.AddTransition("NeedCreate", "CharCreate");
            Game.Fsm.AddTransition("EnterStage", "Loading");
            Game.Fsm.AddTransition("StageReady", "Stage");
            Game.Fsm.AddTransition("Pause",      "Pause");
            Game.Fsm.AddTransition("Resume",     "Stage");
            Game.Fsm.AddTransition("ToMain",     "MainMenu");

            // UI 只发事件，Flow 负责驱动（UI 不引用 Module，保持单向依赖）
            Game.Event.On(Events.OpenSettings, () => Game.UI.Open<SettingsPanel>());
            Game.Event.On(Events.PauseOpened,  () => Game.Fsm.Trigger("Pause"));
            Game.Event.On(Events.Resume,       () => Game.Fsm.Trigger("Resume"));
            Game.Event.On(Events.BackToMain,   () => GoMainMenu());
            Game.Event.On(Events.QuitGame,     QuitGame);
        }

        private void ShowBoot() => EnsureMenuScene(() => Game.UI.Open<BootPanel>());

        private void ShowMainMenu()
        {
            // 主菜单是纯 UI 场景：从舞台退回时要把游戏场景卸掉（见 LeaveStage）
            EnsureMenuScene(() => Game.UI.Open<MainMenuPanel>());
        }

        private void EnterCharSelect()
        {
            EnsureMenuScene(() => Game.UI.Open<CharSelectPanel>());
        }

        /// <summary>菜单类站点统一在 Menu 场景里；已在则不重复加载。</summary>
        private void EnsureMenuScene(System.Action onReady)
        {
            if (Game.Scene.CurrentScene == SceneMenu) { onReady(); return; }
            Game.Scene.Load(SceneMenu, null, () =>
            {
                Game.Logger.Info("Flow", $"menu scene loaded: {SceneMenu}");
                onReady();
            });
        }

        /// <summary>进图：先读条，场景真的加载完了再切状态。</summary>
        public void GoStage()
        {
            Game.Fsm.Trigger("EnterStage");
            Game.UI.Open<LoadingPanel>();
            Game.Scene.Load(SceneStage1, p =>
            {
                var panel = Game.UI.Get<LoadingPanel>();
                if (panel != null) panel.SetProgress(p);
            }, () =>
            {
                Game.UI.Close<LoadingPanel>();
                Game.Fsm.Trigger("StageReady");
                Game.Logger.Info("Flow", $"enter stage: {SceneStage1}");
            });
        }

        /// <summary>离开舞台：把游戏侧的东西全部收干净，否则第二次进图是脏的（见 §5 清场清单）。</summary>
        private void LeaveStage()
        {
            Game.UI.CloseAll();
            Game.Entity.ClearAll();
            Game.Pool.ClearAll();
            Game.Timer.StopScope("stage");
            Game.Sync.Clear();
            // 事件注销要传**同一个方法引用**（没有句柄）：Flow 长驻，订阅一律用具名私有方法，
            // 不要用匿名 lambda，否则 Off 不掉（见 §7）。例：Game.Event.Off(Events.PauseOpened, OnPauseOpened);
            Game.Sound.StopAll();
        }

        private void GoMainMenu()
        {
            Game.Fsm.Trigger("ToMain");
            Game.Scene.Unload(SceneStage1, () => Game.Logger.Info("Flow", "stage unloaded"));
        }

        private void QuitGame()
        {
#if UNITY_EDITOR
            UnityEditor.EditorApplication.isPlaying = false;
#else
            Application.Quit();
#endif
        }
    }
}
```

> `Game.Event.On/Off` **没有句柄**（返回 `void`），注销必须用**同一个方法引用**（或按需一次性清）。写成匿名 lambda 就 `Off` 不掉 —— 长驻的 Flow 模块请用具名私有方法。

### 模板 3：`App/Bootstrap.cs`（唯一组装点，≤200 行）

```csharp
using {Name}.Core;
using {Name}.Module.Flow;
using CloverEngine;
using UnityEngine;

namespace {Name}.App
{
    /// <summary>唯一入口：只做「启动引擎 + 挂模块 + 进流程」，不写业务逻辑。</summary>
    public class Bootstrap : MonoBehaviour
    {
        private IAppFlow _flow;

        private void Start()
        {
            // 1) 启动引擎（配置一律走 Cfg，禁止硬编码地址/账号/超时）
            Game.Launch(new GameConfig
            {
                ServerAddr = Cfg.Server.addr,
                CallTimeoutSeconds = Cfg.Server.call_timeout,
                MaxReconnectCount = Cfg.Server.max_reconnect_count,
                UseTls = Cfg.Server.tls,
            });

            // 2) 资源 / 配表（可选模块，漏了 Game.Res 恒为 null）
            //    Init 的参数是 Resources 内的相对前缀（不是磁盘路径）；空串 = 以 Resources 根为根
            CloverRes.Init("");
            // CloverData.InitDataTable("Table");

            // 3) 输入 + EventSystem：有 UI / 键鼠就必须有，且必须在建 UI 之前
            CloverInput.Init();

            // 4) 联机项目：连网关（单机项目整段不要）
            CloverNet.Init(Cfg.Server.addr,
                string.IsNullOrEmpty(Cfg.Server.udp_addr) ? null : Cfg.Server.udp_addr);

            // 5) 装配 + 进流程（菜单链路从这里开始，不许直接进游戏场景）
            _flow = new AppFlow();
            _flow.Enter();
            Game.Logger.Info("App", $"app flow entered: {_flow.CurrentState}");

            // 6) 网络生命周期：断了要回登录/主菜单，而不是停在游戏里假装没事
            //    Net.OnKicked 为无参发布 → 处理器必须零参
            Game.Event.On("Net.OnKicked", () =>
            {
                Game.Logger.Warn("App", "被踢出，回主菜单");
                Game.UI.CloseAll();
                Game.Fsm.Force("MainMenu");
            });
        }
    }
}
```

### 模板 4：主菜单面板（UI 只发事件，不引用 Module）

```csharp
using {Name}.Core;
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

namespace {Name}.UI
{
    public class MainMenuPanel : UIPanel
    {
        [SerializeField] private Button _startButton;
        [SerializeField] private Button _continueButton;
        [SerializeField] private Button _settingsButton;
        [SerializeField] private Button _quitButton;

        public override void OnOpen(object param)
        {
            Bind(_startButton, Events.StartNewGame);
            Bind(_settingsButton, Events.OpenSettings);
            Bind(_quitButton, Events.QuitGame);

            // 「继续」只在有存档/有角色时可用（真判断，不是摆设）
            var hasSave = Game.Setting.Get("flow.has_save", false);
            _continueButton.interactable = hasSave;
            Bind(_continueButton, Events.ContinueGame);
        }

        private static void Bind(Button b, string evt)
        {
            if (b == null)
            {
                Game.Logger.Error("UI", $"MainMenuPanel 缺少按钮引用，事件 {evt} 未绑定");
                return;
            }
            b.onClick.RemoveAllListeners();
            b.onClick.AddListener(() => Game.Event.Emit(evt));
        }
    }
}
```

### 模板 5：创角面板（提交 → 服务端回包 → 进图）

```csharp
using {Name}.Core;
using {Name}.Def;
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

namespace {Name}.UI
{
    public class CharCreatePanel : UIPanel
    {
        [SerializeField] private InputField _nameInput;
        [SerializeField] private Button _confirmButton;
        [SerializeField] private Button _backButton;

        public override void OnOpen(object param)
        {
            _confirmButton.onClick.RemoveAllListeners();
            _confirmButton.onClick.AddListener(OnConfirm);
            _backButton.onClick.RemoveAllListeners();
            _backButton.onClick.AddListener(() => Game.Fsm.Force("MainMenu"));
        }

        private async void OnConfirm()
        {
            var name = _nameInput.text;
            if (string.IsNullOrEmpty(name))
            {
                Game.Logger.Warn("UI", "创角失败：名字为空");
                return;
            }

            try
            {
                var reply = await Game.Net.Call<CreatePlayerReply>(MsgDef.CreatePlayer, new CreatePlayerRequest
                {
                    name = name,          // ★ 字段名与服务端 json tag 对齐（snake_case）
                });
                if (!reply.ok)
                {
                    Game.Logger.Warn("UI", $"创角被拒: {reply.err}");
                    return;
                }
                Game.Logger.Info("UI", $"创角成功: player={reply.player_id}");
                Game.Event.Emit(Events.CharChosen, reply.player_id);
            }
            catch (CloverCallException ex)
            {
                Game.Logger.Error("UI", $"创角业务错误: {ex.ServerError}");
            }
            catch (System.TimeoutException)
            {
                Game.Logger.Error("UI", "创角超时");
            }
        }
    }
}
```

> 消息号/协议**只能**来自 `Def/MsgDef.cs` 与 `Def/ProtoDef.cs`（`patterns/client/network.md` 模板 0），**禁止**在这里写裸字面量。

### 模板 6：读条面板（`Game.Scene.Load` 的进度直接驱动）

```csharp
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

namespace {Name}.UI
{
    public class LoadingPanel : UIPanel
    {
        [SerializeField] private Image _bar;        // ★ Image.Type = Filled + 有效 sprite，否则 fillAmount 静默失效
        [SerializeField] private Text _tipText;

        public override UILayer Layer => UILayer.System;   // 读条要盖住一切

        public override void OnOpen(object param) => SetProgress(0f);

        public void SetProgress(float p)
        {
            if (_bar == null)
            {
                Game.Logger.Error("UI", "LoadingPanel 缺少进度条引用");
                return;
            }
            _bar.fillAmount = Mathf.Clamp01(p);
        }
    }
}
```

> 进度条的经典静默失效：`Image.Type = Filled` 但 **sprite 为空** ⇒ `fillAmount` 完全无效、看着像"永远不动"。交付前必须**看图**确认它在动（`reference/design-review.md` §3.4）。

---

## 4. 场景划分建议

| 场景 | 内容 | 说明 |
|---|---|---|
| `Boot` | 启动画面（Logo / 版权 / 点击开始） | 最轻，先加载；初始化与热更放这里 |
| `Menu` | 主菜单 / 登录 / 注册 / 设置 / 选角 / 创角 | **纯 UI 场景**（无角色、无战斗）；全部靠面板切换，不切场景 |
| `Stage01` | 游戏场景（首关卡） | 首次进图才加载；卸载后回 `Menu` |

- **一个场景能放多少面板**：同一套"菜单类站点"共用一个 UI 场景（`Menu`），靠 `Game.UI.Open/Close` 切面板 —— **不要每个面板一个 Unity 场景**，否则切界面要等读条，很假；
- 场景必须**在 Build Settings 里**（`unity command` 或 Editor 脚本加），漏了在打包/Play 时直接加载失败；
- 场景名与 `Game.Scene.Load(name)` 的字符串要对齐（**收敛成一个常量类**，禁止散落字符串）。

---

## 5. 回菜单 / 重进游戏的清场清单（第二次进图必须是干净的）

| 要清的 | 用哪个 API |
|---|---|
| 面板 | `Game.UI.CloseAll()`（或逐个 `Close<T>()`） |
| 实体与视图 | `Game.Entity.ClearAll()`（或 `DestroyGroup(group)`） |
| 对象池 | `Game.Pool.ClearAll()`（或 `ClearGroup(group)`） |
| 定时器 | `Game.Timer.StopScope("stage")`（舞台内的定时器统一打 scope） |
| 世界同步订阅 | `Game.Sync.Clear()` |
| 流程/网络事件 | `Game.Event.Off(Events.X, method)`（**同一方法引用**） |
| 消息监听 | `Game.OffMsg(msgID, handler)`（或 `Game.OffMsg(msgID)` 清该消息号） |
| 音效 / BGM | `Game.Sound.StopBGM()` / `Game.Sound.StopAll()` |

> **自证方法**：连续"主菜单 → 进图 → 回主菜单 → 再进图"两次，第二次截图 + 抄一遍上面的计数（实体数 / 池活跃数 / 面板数）应当与第一次一致 —— **不涨就是干净的**。

---

## 6. 交付前自检清单

```text
□ 打开应用（Play）后先进「启动画面 / 主菜单」，不直接落到游戏场景
□ 主菜单 4 项（开始 / 继续 / 设置 / 退出）逐个点过；联机版另有登录 / 注册，也逐个点过
□ 设置面板真能改音量 / 画质，并且重进后设置还在（Game.Setting）
□ 创角：空名字、重名、超时、服务端拒绝 —— 四种失败都有提示与日志
□ 选角：能选中、能删除（有二次确认）、能返回
□ 读条：进度条真的在动（截图两张不同进度），加载完才关面板
□ 游戏内：暂停菜单能开、能继续、能回主菜单（有二次确认）
□ 连续「回主菜单 → 再进图」两次，第二次画面与计数与第一次一致（§5 清场清单过了一遍）
□ `表现类` 的界面**都在联络图里逐格出现**（启动 / 主菜单 / 创角 / 读条 / 游戏内 / 暂停 / 结算），
  格号写进验收表；`数值类` 只留日志行（`SKILL.md` §2 硬性判定第 3 条）
□ ★ **与参考游戏的同角度截图逐项对照，全部"一致"**（`reference/design-review.md` §3.6）——
   **有任何一项不一致 = 不可交付**；**没有参考游戏名就先问用户要**
□ 全工程 grep：无裸事件名（`Game.Event.On("` 命中 0）、UI 无 `using {Name}.Module`（命中 0）
```

grep 命令见 `reference/architecture.md` §4（②③④⑤ 允许为 0 命中）。

---

## 7. 常见坑

| 现象 | 真因 | 正确做法 |
|---|---|---|
| 点不动任何按钮 | `EventSystem` 没建（输入模块未挂） | `Game.Launch` 后调 `CloverInput.Init()`，且在建 UI 之前 |
| 面板打开是空白 / 报"找不到预制体" | 预制体没放在 `Resources/UI/{类名}` | 预制体名必须 = 面板类名（`UIPanel.PanelName` 默认取类名）；由 AI 用 Editor 脚本生成 |
| 第二次进图怪物翻倍 / 血条错乱 | 上次退出没清实体与订阅 | 走 §5 清场清单 |
| 切界面卡黑屏 | 每个界面都切 Unity 场景 | 菜单类站点共用一个 UI 场景，只切面板 |
| 读条条永不动 | `Image.Type=Filled` 但 sprite 为空 | 给足 sprite，或改用 `RectTransform` 宽度；交付前**看图** |
| 暂停后角色还在被打 | 联机对战**不能真暂停**（原版也是） | 单人关卡才做真暂停（服务端暂停消息）；对战只做"菜单覆盖 + 可选投降" |
| 断线后停在游戏里 | 没监听网络生命周期 | 监听 `Net.OnDisconnected/OnResumed/OnKicked`，被踢 → 回主菜单/登录 |

---

## 8. 与其他文档的关系

- **首场景内容**（操作/相机/动画/战斗/UI/音效）→ `patterns/game-demo.md` §0.1；
- **原版系统清单**（菜单/创角/技能/背包/NPC…逐个做完）→ `patterns/game-demo.md` §0.0c；
- **感官验收**（**按类别取证**：`表现类` 的所有点**采一次联络图**、脚本按格判定、**AI 只读那张汇总图一次**；
  `数值类` 只留日志行）→ `reference/design-review.md` §3 与 `reference/visual-loop.md` 第八节；
- **分层与自检**（UI 不引 Module、App ≤ 200 行）→ `reference/architecture.md`；
- **复杂项目拆多 agent**（菜单链路与首场景可并行）→ `patterns/multi-agent.md`。
