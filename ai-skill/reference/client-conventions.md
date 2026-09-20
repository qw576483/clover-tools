# 客户端团队约定

## 1. Game 门面单一入口

所有模块访问通过 `Game` 门面，禁止直接实例化模块：

```typescript C#
Game.Net.Send(EMsg.Xxx, msg);
Game.Event.On("xxx", handler);
Game.Timer.After(3f, callback);
Game.Res.LoadAsset<GameObject>("xxx", go => { /* 使用 go */ });   // 回调式，无 await
Game.UI.Open<SomePanel>();
Game.Entity.Get(entityId);
```

## 2. 初始化顺序

必须先 `Game.Launch`，再使用各模块。
**只要工程里有 UI（按钮/输入框）或任何键鼠操作，`CloverInput.Init()` 必须排在「构建 UI」之前**——
它负责挂载输入后端与 EventSystem，晚于 UI 构建就会导致按钮点不了：

```typescript C#
void Start()
{
    // 1. 启动引擎
    Game.Launch(config);

    // 2. 输入与 UI 基础设施（有 UI/键鼠就必须有，且必须在建 UI 之前）
    CloverInput.Init();

    // 3. 初始化网络（仅联机项目需要；单机项目跳过这一步）
    //    参数 = 网关 TCP 口 + 网关 UDP 口（对应 server.yaml 的 gateway.listen_tcp / listen_udp）
    //    ⚠️ 必须填 8002：8001 是 WS 口，Unity 原生客户端走裸 TCP
    //    GameConfig.UseTls 必须与服务端 gateway.tcp_tls_disabled **相反**（服务端默认 false → 这里 true）
    CloverNet.Init("127.0.0.1:8002", "127.0.0.1:8003");

    // 4. 注册监听（在 Login 之前）
    Game.OnMsg(EMsg.SomeNotify, handler);

    // 5. 登录（回包类型是 ELoginReply，请求是 ELoginRequest）
    await Game.Net.Call<ELoginReply>(EMsg.Login, request);
}
```

## 3. 网络消息处理

### 3.0 消息号与协议必须先落到 `Def/`（硬规则）

- 业务消息号**唯一定义处** = `Assets/Scripts/Def/MsgDef.cs`；协议结构体 = `Assets/Scripts/Def/ProtoDef.cs`。
- 引擎消息号走 `CloverEngine.EMsg`（`[1,10000]`，封闭 static class，业务不许往里加）；
  业务消息号走 `{Name}.Def.MsgDef`，必须 `>= 10001`。
- **禁止**把消息号/协议结构写在 `GameManager.cs`、UI 脚本或任何业务类里（散落 = 不合格，
  常见错法：在 `GameManager` 里 `private const uint MsgMatch = 1000101;`）。
- 客户端 `MsgDef` 与服务端 `server/game/def/{msg,reply,push}.go` **逐条对齐**，改一端必须同步另一端。
- 模板见 `patterns/client/network.md` 模板 0。
- **引擎号位由引擎自己用，业务不要碰**：`1-7` 是引擎请求 + 网关直发帧（含 `EMsg.QueuePosition`=7 排队位置），`4001+` 是引擎推送；
  业务只从 `10001` 起编自己的号。排队位置与会话通道加密**全在引擎里闭环**（见 `patterns/client/network.md` 模板 7）——
  业务不要给 `ELoginRequest.encrypt` 赋值，也不要自己实现排队 UI 之外的东西。

### 3.1 注册监听

```typescript C#
// 注册推送监听（OnMsg 返回 void，没有可保存的句柄）
void OnSomeNotify(NetCtx ctx)
{
    var msg = ctx.Bind<SomeNotify>();
    // 处理逻辑
}
Game.OnMsg(EMsg.SomeNotify, OnSomeNotify);

// 取消监听：传回同一个方法引用
Game.OffMsg(EMsg.SomeNotify, OnSomeNotify);
```

### 3.2 请求-回包

```typescript C#
// 异步请求
try
{
    var reply = await Game.Net.Call<BuyItemReply>(EMsg.BuyItem, new BuyItemRequest
    {
        item_id = 1001,    // ★ 业务消息字段必须是 snake_case（与服务端 json tag 对齐）
        count = 1,
    });
    Game.Logger?.Info("Net", $"购买成功: {reply.order_id}");
}
catch (CloverCallException ex)
{
    // 服务端业务错误（EMsg.Error 回包）：ex.ServerError 为描述，ex.Code 为机器可读错误码（见 ErrCode）
    Game.Logger?.Error("Net", $"业务错误: {ex.ServerError} (code={ex.Code})");
}
catch (TimeoutException)
{
    // 请求超时
    Game.Logger?.Error("Net", "请求超时");
}
```

### 3.3 可靠/非可靠发送

```typescript C#
// 可靠发送（TCP，保证到达）
Game.Net.Send(EMsg.ChatMessage, new ChatMessage
{
    Content = "你好",
});

// 非可靠发送（UDP，可能丢失）
Game.Net.SendUnreliable(EMsg.PositionSync, new PositionData
{
    X = transform.position.x,
    Y = transform.position.y,
    Z = transform.position.z,
});
```

### 3.4 高频周期同步：优先用推送，别用高频 Call 轮询

服务端有公开推送能力（`g.PushToPlayer` / `PushToScene` / `PushToAll`，对象版内部 JSON 序列化）。
**频率高于 5Hz 的周期状态同步（如 20fps 房间状态）应当走推送**：服务端 `Push*` + 客户端 `Game.OnMsg`。

用 `Game.Net.Call` 高频轮询的代价（历史事故的真实数据）：2 人 × 20fps = 每秒 40 次 `Call`，
每次都要分配 requestID、往 pending 表插一项、挂一个超时定时器；同时服务端 `def/push.go` 里
定义好的 `Push*` 消息号全程没人用、客户端也不注册 `OnMsg` —— 推送通道白白浪费。

> 引擎 `pkg/app/app.go` 的注释确实认可「20fps 状态同步也可直接 `g.Reply` 轮询」，所以轮询**不算违规**。
> 但既然 `def/push.go` 已经定义了推送消息号，就应该真用起来，否则那些常量是死代码，还会误导后续维护者。
>
> **配套要求**：选轮询 → 切勿在 `def/push.go` 里留一堆没人用的推送消息号；选推送 → 客户端必须注册对应 `OnMsg`。
>
> **禁止未查证断言**：不确定引擎有没有某个 API 时，去 `clover-server-engine` 源码查
> （推送在 `pkg/app/app.go`），**不要在注释或交付说明里写「引擎没有 X API」**——这是 `SKILL.md` 明列的违规项。

## 4. 生命周期事件

使用 `Game.Event` 监听网络状态：

```typescript C#
// 连接成功（无参事件 → 零参 lambda）
Game.Event.On("Net.OnConnected", () =>
{
    Game.Logger?.Info("Net", "已连接到服务器");
});

// 连接断开（自动重连中；无参事件）
Game.Event.On("Net.OnDisconnected", () =>
{
    Game.Logger?.Warn("Net", "连接断开，正在重连...");
});

// 会话恢复成功（带参事件 → 显式 On<T>）
Game.Event.On<EResumeSessionReply>("Net.OnResumed", data =>
{
    Game.Logger?.Info("Net", "会话已恢复");
});

// 被踢出（重连次数耗尽；无参事件）
Game.Event.On("Net.OnKicked", () =>
{
    Game.Logger?.Warn("Net", "被踢出，需要重新登录");
    // 清理本地状态，返回登录界面
});

// 未认证（code=401）：未登录就发业务消息被网关登录门禁拒绝，或逻辑服返回 401。
// 与 OnKicked 的区别：OnKicked 是连接断了；这个是连接还在、但请求被拒。
Game.Event.On<EErrorReply>("Net.OnUnauthorized", e =>
{
    Game.Logger?.Warn("Net", $"未认证（code={e.code}）: {e.err}");
    // 清理本地会话，返回登录界面
});
```

## 5. 模块依赖方向

遵循 asmdef 依赖规范 —— **asmdef 层面各模块只引用 `Core`，模块之间互不引用**；
跨模块能力一律经 `Core` 的接口 + `Game` 门面访问：

```
CloverEngine.Core            （无依赖）
CloverEngine.Data            （引用 Core）
CloverEngine.Network         （引用 Core）
CloverEngine.Resource        （引用 Core）
CloverEngine.Presentation    （引用 Core）
```

### 依赖方向

`Network` / `Data` / `Resource` / `Presentation` → `Core` **单向依赖**：

```typescript C#
using CloverEngine;   // 命名空间只有 CloverEngine（没有 CloverEngine.<子名>，子模块类型也在此命名空间内）
```

## 6. 资源加载

### 资源创建规范（占位资源）

**必须创建的文件夹结构**：
```
Assets/
├── Resources/
│   ├── Placeholder/          # 占位图片（必须创建）
│   │   ├── placeholder_64x64.png   # 默认 64x64 像素
│   │   ├── placeholder_128x128.png # 可选 128x128 像素
│   │   └── placeholder_256x256.png # 可选 256x256 像素
│   ├── Sound/SFX/                # 占位音效（可选）
│   └── Models/               # 占位模型（可选）
├── Scripts/
│   └── Resources/            # 资源管理脚本
└── Configs/
```

**占位图片生成规则**：
- 默认生成 64x64 像素的 PNG 占位图（纯色块，如灰色或红色）
- 必须创建对应的文件夹结构
- 图片文件必须实际存在，不能只是代码引用
- 如果用户指定了特定尺寸，按用户要求生成
- 标注尺寸的目的是方便用户替换时对齐真实美术资源，Unity uGUI 默认会自动拉伸占位图

**尺寸标注的重要性**：
- 占位图本身会被 uGUI 自动拉伸到 RectTransform 尺寸，不影响占位显示
- 但标注尺寸是为了用户替换时知道目标尺寸，避免比例失调或清晰度不一致
- Sliced/Tiled 模式的 UI、游戏内 Sprite、图标/头像等固定比例资源，尺寸标注尤为重要

### 资源欠缺清单（client 根目录，强制交付物）

- **固定路径：`client/资源欠缺清单.md`**（client 工程根目录，全工程唯一）
- 只要实现**引用了**外部资源（图片/Sprite、音效、动画、模型、字体、美术预制体），收尾必须新建或更新
- 占位资源也要登记（当前占位 = 纯色块 / 空实现 / 内置几何体）；纯逻辑功能不用建
- 已存在时**追加/更新行**，不覆盖已完成的条目；替换完改状态为 ✅
- 每行必须可照做：目标路径 + 尺寸/规格 + 影响的代码位置
- 模板见 `patterns/client/resource.md`

### 客户端交付必须"打开即跑"（禁止留手工操作给用户）

- 场景由 AI 创建并保存（如 `Assets/Scenes/Main.unity`），并加进 Build Settings
- 业务脚本由 AI **挂到场景里的 GameObject 上**，不许让用户自己"创建空物体再挂脚本"
- 编译无错（`unity console tail` 干净）才能交付
- 详见 `reference/unity-cli.md` §7

**资源引用规范**：
- 资源引用必须收敛到资源常量表 / 集中路径
- 禁止散落路径字面量
- 用户替换资源时，只换文件不碰逻辑

### 资源加载代码

```typescript C#
// 异步加载（回调式，没有 await / 同步版本）
Game.Res.LoadAsset<GameObject>("Prefabs/Enemy", prefab => { /* Instantiate(prefab) */ });

// 带进度
Game.Res.LoadAsset<GameObject>("Prefabs/Enemy", p => { }, prefab => { /* ... */ });

// 预先批量加载
Game.Res.Preload(new List<string> { "Prefabs/Enemy" }, () => { /* done */ });

// 释放引用（只降计数、不立即卸载；缓存淘汰由内存水位 + LRU 决定）
Game.Res.Release("Prefabs/Enemy");
```

## 7. UI 管理

```typescript C#
// 打开 / 关闭（面板继承 UIPanel 或实现 IUIPanel；预制体放 Resources/UI/{类名}）
Game.UI.Open<LoginPanel>();
Game.UI.Close<LoginPanel>();

// 取已打开实例（不用传名字）
var panel = Game.UI.Get<LoginPanel>();
```

## 8. 事件总线

```typescript C#
// 注册监听（带参事件必须显式泛型）
Game.Event.On<LevelUpData>("Player.LevelUp", data =>
{
    Game.Logger?.Info("Event", $"升级: {data.Level}");
});

// 发布事件
Game.Event.Emit("Player.LevelUp", new LevelUpData { Level = 10 });

// 一次性监听（无参事件 → 零参 lambda）
Game.Event.Once("Net.OnConnected", () =>
{
    // 只触发一次
});

// 取消监听：On 返回 void，用 Off 传回同一个方法引用
Game.Event.Off("xxx", callback);
```

## 9. 定时器

```typescript C#
// 延迟执行
Game.Timer.After(3f, () =>
{
    Game.Logger?.Info("Timer", "3秒后执行");
});

// 循环执行（返回 long id，不是 IDisposable）
Game.Timer.Every(1f, () =>
{
    Game.Logger?.Info("Timer", "每秒执行");
});

// 取消定时器：拿 id 用 Stop
var id = Game.Timer.Every(1f, callback);
Game.Timer.Stop(id);

// 也可用具名（同名先停旧，随时按名停）
Game.Timer.EveryName("tick", 1f, callback);
Game.Timer.StopNamed("tick");
```

## 10. 状态机

```typescript C#
// 不要 new Fsm()（实现类是 internal）；用引擎共享的 Game.Fsm
var fsm = Game.Fsm;

// 注册状态（参数名是 onTick，不是 onUpdate）
fsm.RegisterState("idle",
    onEnter: () => Game.Logger?.Info("FSM", "进入空闲"),
    onTick: dt => { },
    onExit: () => Game.Logger?.Info("FSM", "退出空闲")
);

// 切换状态
fsm.Force("idle");
```

## 11. 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 消息号 | `EMsg.Xxx` | `EMsg.Login` |
| 消息结构 | `XxxRequest/Reply/Notify` | `LoginRequest` |
| UI 面板 | `XxxPanel` | `LoginPanel` |
| 实体视图 | `XxxView` | `PlayerView` |
| 事件名 | `"Module.EventName"` | `"Net.OnConnected"` |
| Unity 关卡 | `Game.Scene`（`ISceneManager`） | `Game.Scene.Load("MainCity")` |
| 服务端场景 | `CloverScene`（`Game.CloverScene`） | `Game.CloverScene.SceneID` |

### 场景命名硬规则

- `Game.Scene` / `ISceneManager` **只指 Unity 关卡**（资源加载/渲染/场景卸载清理），不得用于指代服务端场景。
- 客户端表示**服务端场景**（服务端 `mmo.Scene` 的投影）一律用 `CloverScene` / `Game.CloverScene`。
- 服务端 `mmo.Scene` 保持原名不改。
- `Entity.SceneGroup` 中的 Scene 也指 Unity 关卡（场景卸载时按组批量清理），不得赋予服务端场景语义。

> 完整约定见 [`clover-doc/client/concepts/concept-naming.md`](https://github.com/qw576483/clover-doc/blob/main/client/concepts/concept-naming.md)。

## 12. 错误处理

```typescript C#
// 网络请求错误
try
{
    var reply = await Game.Net.Call<T>(EMsg.Xxx, request);
}
catch (System.TimeoutException)
{
    // 超时处理（默认 10s，GameConfig.CallTimeoutSeconds 可配）
}
catch (CloverCallException ex)
{
    // 服务端业务错误：ex.ServerError 是人类可读描述，ex.Code 是机器可读错误码（见 ErrCode）。
    // 按码分支，不要匹配文案——文案会变，码不会。
    switch (ex.Code)
    {
        case ErrCode.Unauthenticated: BackToLogin();                                   break;
        case ErrCode.Forbidden:       ShowNoPermission(ex.ServerError);                break;
        default:                      Game.Logger?.Error("Net", $"业务错误: {ex.ServerError}"); break;
    }
}
```

## 13. 输入（必须走 `Game.Input`，禁止直接用 `UnityEngine.Input`）

> **遇到「键鼠不好使 / 按钮点不了」时的强制动作（不准跳过）**
>用户反复反馈键鼠输入不好使时，**禁止**凭猜去改 UI 布局、RectTransform、CanvasScaler、
> 射线、DPI 缩放或 MaximizeOnPlay —— 历史证明这些都不是原因，全是白改。
> **必须**按顺序做这两件事：
> 1. 让用户贴出 Console 里 `[Clover.Input]` 那一行（它会直接告诉你当前生效的输入后端与可用性）；
> 2. 对照本文末尾的**排查清单**逐条核对（重点是 `activeInputHandler` 的取值   
>    和「改完是否重启了 Unity」）。
>一句话记忆：**键鼠无响应先查输入后端，不必排查 UI。**

**硬规则**：业务代码里**不许出现** `Input.GetKeyDown(...)` / `Input.mousePosition` / `Input.GetAxis(...)`。
一律走 `Game.Input`，按键用引擎的 `GameKey` 枚举。

```typescript C#
// 后端无关：GameKey 由引擎翻译成当前生效后端的实际按键
if (Game.Input.GetKeyDown(GameKey.Space)) Jump();
if (Game.Input.GetKeyDown(GameKey.Num1)) StartSolo();   // 数字 1
float h = Game.Input.GetAxis("Horizontal");
var mouse = Game.Input.MousePosition;
bool click = Game.Input.GetMouseButtonDown(0);
```

### 为什么禁止直连

`UnityEngine.Input`（旧输入）只有在 **Player Settings → Active Input Handling** 含
「Input Manager (Old)」时才可用。若该设置为「Input System Package (New)」：

- `Input.*` 每次调用都抛
  `InvalidOperationException: You are trying to read Input using the UnityEngine.Input class, but you have switched active Input handling to Input System package in Player Settings.`
- 异常在 `Update()` 里抛出会**中断整帧后续逻辑**，所以**键盘也一起失效**（不只是点不了按钮）；
- uGUI 的 `StandaloneInputModule` 内部同样读旧输入 → **按钮全部点不动**。
  现象就是「键鼠一点效果也没有」。

`Game.Input` 正是为消除这类事故而存在：运行时自动探测后端（旧输入 / 新输入 / 无），
不可用时只打一条可操作的日志，**绝不把异常抛进帧循环**。

### 后端选择顺序（默认只用新输入系统）

`CloverInput.Init()` 按以下顺序选后端，业务侧无需关心：

1. **新输入系统（InputSystem）—— 默认首选。** 需同时满足：`activeInputHandler` 含 `New`（`1` 或 `2`）、
   装了 `com.unity.inputsystem`、`Keyboard.current != null`、`InputSystem.Key` 枚举可解析。   
   任一条不满足即视为不可用并自动回退（专门避免「日志说可用、按键却全无反应」这种死局）。
2. **旧输入（Legacy）—— 兜底。** 仅当新后端不可用才用。
3. **None —— 都不可用时不抛异常**，只报一条可操作的修复日志。

UI 的输入模块**同步跟随**所选后端：新后端 → `InputSystemUIInputModule` 
（类型在 `Unity.InputSystem` 程序集；包内 versionDefine 在装了 `com.unity.ugui` 时定义
`UNITY_INPUT_SYSTEM_ENABLE_UI`）；旧后端 → `StandaloneInputModule`。**两者绝不并存。**

后端**自动选择**（无手动开关）：`InputSystem` 探测可用即用它，否则退回 `Legacy`。
若日志显示 `输入后端=InputSystem` 但按键仍无反应，属输入模块 / EventSystem 配置问题 ——
按日志提示改 `Active Input Handling` 并**完全重启 Unity 编辑器**。

### 排查清单（键鼠无响应时按序查）

| 序号 | 检查项 | 命令 / 位置 |
|---|---|---|
| 1 | 日志里 `[Clover.Input] 输入后端=?` | 期望 `InputSystem`。若是 `Legacy`，看括号里的回退原因；若是 `None` → 按同一行给出的修复提示改 Active Input Handling |
| 2 | `ProjectSettings.asset` 的 `activeInputHandler` | `0`=Old、`1`=New、`2`=Both；**引擎默认走新后端（New），所以至少要含 New**（即 `1` 或 `2`） |
| 3 | 改完设置后是否重启 Unity | **该设置在编辑器启动时只读一次，不重启不生效**。且 Unity 运行中手改该文件会被回写覆盖，必须先关编辑器再改 |
| 4 | 日志里 `InputModule=` 是什么 | 期望 `InputSystemUIInputModule`。若是 `StandaloneInputModule` → 说明回退了旧模块，`activeInputHandler` 为「只新」时 UI 点击会全灭 |
| 5 | 日志有没有 `[Clover.Input] EventSystem ...` | 没有 → 说明 `CloverInput.Init()` 没在构建 UI 之前调用 |
| 6 | 有没有第二个 InputModule | 引擎会打印「移除不匹配的输入模块」，出现两条并存即点击失效 |
| 7 | 是否在跑 headless 实例 | 有残留 `Unity.exe`（批处理）占着工程时，正常编辑器打不开，别误判为代码问题 |

> 排查顺序很重要：**先看 `[Clover.Input]` 的后端日志，再看 EventSystem**。
> 「键鼠全死」几乎都是后端（Active Input Handling）问题，不是 UI 布局/射线/DPI 问题——
> 不要再去调 RectTransform、CanvasScaler、MaximizeOnPlay 这些东西。

## 14. 测试策略（当前约定）

- **新功能先不写单测**：本仓库现阶段是**功能优先** —— 实现新能力时不要求同时产出单元测试。
  AI **不应**把「补单测」当作待办项反复汇报，也**不应**以「缺测试」为由阻塞功能交付。
- **既有测试套件要保持可用**：[`clover-client-unity-engine/Tests/`](https://github.com/qw576483/clover-client-unity-engine/tree/main/Tests)（EditMode + PlayMode）里的用例
  仍须**能编译、能跑过**；改引擎代码把它们跑红了，属于必须修的回归。
- **允许为真实缺陷补回归用例**：定位到真实 bug（尤其只在运行时暴露、编译期看不出来的那类）时，
  补一条能复现它的用例是**排查手段**，不算「给新功能写单测」。
- **不要自行改变本约定**：何时转为「必须写测试」由用户决定。

> 跑测试的方法（承载工程 + `testables` + 两个 PlayMode 坑）见 `reference/unity-cli.md` §4.1。
