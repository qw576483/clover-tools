# 客户端模块速查表

> 本表的 API 全部以 `clover-client-unity-engine/Runtime/**` 源码为准（`Game` 门面见 `Runtime/Core/Game.cs`）。
>
> **表现域模块现已在门面上全部可用**（Map / UI / Scene / Atlas / Anim / Sound / Camera / Quality / Entity / Pool）。
> 契约定义在 `Runtime/Core/PresentationContracts.cs`（Core），实现在 `Runtime/Presentation/`（internal），
> 由 `CloverPresentation.Init` 挂接——**随 `Game.Launch` 自动完成，业务无需手动初始化**：
>
> ```csharp 
> Game.Launch(new GameConfig { ServerAddr = "127.0.0.1:8002" });
> Game.UI.Open<LoginPanel>();                 // 直接用
> ```
>需要精细控制（EditMode 测试 / 只想挂一部分）时关闭自动挂载：
> ```csharp 
> CloverPresentation.AutoMount = false;   // 必须在 Launch 之前设置
> Game.Launch(config);
> CloverPresentation.Init();              // 手动挂载（幂等）
> ```

## 模块架构

```
┌──────────────────────────── 业务游戏代码 ────────────────────────────┐
│                              Game 门面                               │
│  （已挂载：Logger Dispatcher Event Timer Fsm Setting DeviceId Net     │
│             Http Sync Schema Alert CloverScene FrameRoom LanBrowser   │
│             Table Localization Res Map Entity Pool Input              │
│             UI Scene Atlas Anim Sound Camera Quality）                 │
├──────────────┬──────────────┬──────────────┬────────────────────────┤
│   网络域      │   表现域      │   数据域      │   基础域                │
│  Net         │  Entity      │  Table       │  Event                 │
│  Sync        │  Pool        │  Setting     │  Timer                 │
│  Http        │  Input       │  Localization│  Fsm                   │
│              │  Scene       │              │  Dispatcher            │
│              │  UI          │              │  Logger                │
│              │  Atlas       │              │                        │
│              │  Anim        │              │                        │
│              │  Sound       │              │                        │
│              │  Camera      │              │                        │
│              │  Quality     │              │                        │
├──────────────┴──────────────┴──────────────┴────────────────────────┤
│  Editor 横切：Debugger（面板/GM/网络模拟/统计）                            │
└─────────────────────────────────────────────────────────────────────┘
```

## 能力域划分

| 域 | 模块 | 说明 |
|----|------|------|
| **基础域** | Event, Timer, Fsm, Dispatcher, Logger | 引擎基石，`Game.Launch` 时构造 |
| **数据域** | Table, Setting, Localization | 经 `CloverData.InitDataTable/InitLocalization` 挂载 |
| **网络域** | Net, Sync, Schema, Alert, CloverScene, FrameRoom, **Http** | 经 `CloverNet.Init` 一次挂载（**含 Http**；`CloverNet.InitHttp` 保留供「只用 HTTP 不连网关」的特例）。Schema 桥接 Sync；FrameRoom 需业务 `Configure()` 注入消息号 |
| **资源域** | Res | 经 **`CloverRes.Init(root)`** 挂载 —— ⚠️ **不会**随 `CloverPresentation` 自动挂（表现域挂 Map/Entity/Pool/UI/Scene/Atlas/Anim/Sound/Camera/Quality）。**业务必须显式调用**，否则 `Game.Res` 恒为 null：模型/贴图**静默加载失败**（只剩占位几何体），并在加载点抛 `NullReferenceException` 打断业务主流程（实测踩过，见 `patterns/game-demo.md` §4.2） |
| **表现域** | Map, Scene, UI, Atlas, Anim, Sound, Camera, Quality, Entity, Pool, Input | 契约在 `Runtime/Core/PresentationContracts.cs`，实现由 Presentation 提供；Map/Entity/Pool/UI/Scene/Atlas/Anim/Sound/Camera/Quality 由 `CloverPresentation.Init` 随 `Game.Launch` **自动挂载** |

## 模块 API 速查

### 基础域

| 模块 | API | 说明 |
|------|-----|------|
| **Event** | `Game.Event.On / On<T> / OnPriority(name, priority, handler) / Once / Off / Emit / OffAll`；订阅名支持通配 `*`（恰好一段）/ `**`（一段或多段） | 事件总线。`priority` 大者先执行、**精确订阅先于通配订阅**执行；同一 handler 重复注册被忽略并告警；分发期间 `Off`/`OffAll` 即时生效。**`On` 返回 `void`**，取消用 `Off`，没有 `Dispose` 句柄 |
| **Timer** | `Game.Timer.After / Every / AfterName / EveryName / AfterUnscaled / EveryUnscaled / Stop / StopNamed / StopScope / StopAll`（`After` / `Every` 另有带 `scope` 的重载） | 定时器：`After` / `Every` 系列返回 `long` id（**不是** IDisposable），`Stop*` 系列返回 `void`。无 `Group`、无 `Cron`、无 `FromSeconds`；`timeScale=0` 里要触发用 `*Unscaled` |
| **Fsm** | `Game.Fsm.RegisterState / AddTransition / Transition / Trigger / Force / Current / Tick / OnChange / OffChange` | 状态机；`Fsm` 是 `internal`，**禁止 `new Fsm()`**，用 `Game.Fsm` |
| **Logger** | `Game.Logger.Info / Warn / Error(tag, msg)` | 日志（**必须两个参数** tag+msg）；**永不为 null** —— 未 `Game.Launch` 时指向 `ConsoleLogger`（直接写 Unity Console），所以 `?.` 加不加都能用（见 `patterns/client/config.md`） |

### 数据域

| 模块 | API | 说明 |
|------|-----|------|
| **Table** | `Game.Table.Load<T>(file, parser)` / `Get<T>(id)` / `GetAll<T>()` / `Clear()` | 配表查询（TSV） |
| **Setting** | `Game.Setting.Get<T>(key, default)` / `Set<T>(key, value)` / `Save()` / `Load()` / `Delete` / `DeleteAll` | 本地设置（JSON 持久化） |
| **Localization** | `Game.Localization.Get(key)` / `.Language` | 多语言（**成员名是 `Localization`，不是 `Locale`**） |

### 网络域

| 模块 | API | 说明 |
|------|-----|------|
| **Net** | `Game.OnMsg(msgID, handler)` / `Game.OffMsg`（**唯一路由入口**）+ `Game.Net.Connect / Send / SendUnreliable / Call<T> / SetupSession / IsConnected / IsUdpBound / Session / IsQueued / QueueAhead / QueueTotal / IsChannelEncrypted` | 可靠 TCP + 裸 UDP，**不含路由**（INetwork 已剥离 OnMsg/OffMsg）。`OnMsg` 返回 `void`，取消用 `OffMsg`；回包走 `Call<T>` 不注册回调。排队/通道加密由引擎自动处理（事件 `Net.QueuePosition`），业务不接线 |
| **Http** | `Game.Http.Get(url, cb)` / `Post(url, body, cb)` | HTTP 请求，**回调式**（无泛型、无 await） |
| **Sync** | `Game.Sync.OnFullSync(...)` / `OffFullSync(...)` / `OnData(...)` / `OffData(...)` / `OnEntityEnter` / `OnEntityLeave` / `OnEntityMove` / `OnEntityProperty`（每个 `On*` 都有配对的 `Off*`，另可 `Clear()` 一次清空）；`TryGetPosition(id, out x, out y, out z)` 读**插值后**的当前位置 | **数据/实体订阅唯一入口**（增量 + 全量）。**没有 `GetEntity`**。`OnEntityMove` 给的是**服务端原始目标坐标**（离散），驱动表现要用 `TryGetPosition`（见 `patterns/client/entity-view`） |
| **Schema** | `Game.Schema.RegisterSchema(type, schema)` / `GetSchema(type)` | 仅 Schema 声明表（登记字段结构），**不做数据订阅**。**门面名是 `Schema`，不是 `Data`** |
| **Alert** | `Game.Alert.OnAlert += handler` / `-= handler` | 公告推送（`EMsg.PushAlert`）；同时发 `Net.Alert` 事件 |
| **CloverScene** | `Game.CloverScene.IsValid / SceneID / InstanceID / Name / UnityScene / AutoLoadUnityScene` / `RegisterMapping(sceneId, unityScene)` / `OnChanged` / `OffChanged` | 服务端场景投影（`EMsg.PushSceneInfo`）。**与 `Game.Scene`（Unity 关卡）不同义** |
| **FrameRoom** | `Game.FrameRoom.Configure(msgIds)`（必须）/ `CreateRoomAsync` / `JoinRoomAsync` / `LeaveRoomAsync` / `SendInput` / `OnFrame` / `OnClosed` / `OnTakeover` | 帧同步房间；消息号由业务注入，引擎不含业务消息号 |

### 资源域

| 模块 | API | 说明 |
|------|-----|------|
| **Res** | `Game.Res.LoadAsset<T>(path, cb)` / `LoadAsset<T>(path, progress, cb)` / `Release(path)` / `UnloadAll()` / `Preload(paths, onDone, progress)` / `TryGet<T>(path)`（同步取已驻留资源） | 资源加载。**没有 `LoadAsync`/`LoadSync`/`Unload`/`LoadScene`**；**热更成员是 5 个**：`Version` / `UpdateState` / `CheckUpdate(Action<ResourceUpdateInfo>)` / `DownloadUpdate(...)` / `ClearDownloaded()`（见 `Runtime/Core/Contracts.cs` 与 `patterns/client/resource.md` 模板 6） |

### 表现域（已挂载）

| 模块 | API | 说明 |
|------|-----|------|
| **Entity** | `Game.Entity.Create(objectID, typeID, group)` / `Get(objectID)` / `GetAll()` / `GetByGroup(group)` / `Destroy(objectID)` / `BindView(objectID, view)` / `GetView(objectID)` / `DestroyGroup(group)` / `ClearAll()` | 实体管理（`Create` 参数**不是** prefab+position）。异步 View 工厂（模型异步加载 + 占位 + 竞态 + 贴地）走 `CloverPresentation.EntityView.CreateView(...)` 后 `BindView` 登记 |
| **Map** | `Game.Map.Load(bytes, out error)` / `LoadFromResource(path, onDone)` / `WalkableAt(x, z)` / `Clear()`（另有 `Loaded` / `Status` / `Width` / `Depth` 等只读属性） | 逻辑地图（服务端权威地图的**只读投影**，本地碰撞 / 寻路查询用）。**与 `Game.Scene`（Unity 关卡）、`Game.CloverScene`（服务端场景）不同义** |
| **Pool** | `Game.Pool.Spawn(key, parent, group)` / `Despawn(obj)` / `Preload(key, count, group)` / `Clear(key)` / `ClearGroup(group)` / `ClearAll()` / `GetActiveCount(key)` / `GetInactiveCount(key)`；空闲过期回收 `IdleExpirySeconds`（默认 0 = 关闭）/ `TrimIdle()` | 对象池。回收是 **`Despawn`**，不是 `Recycle`。`IdleExpirySeconds>0` 时闲置超时报废，回收惰性发生在 `Spawn`/`Despawn` 内（不新增 Tick） |
| **Input** | `Game.Input.State` / `.Available` / `.BackendName` / `.IsLocked` | 输入（后端无关；需 `CloverInput.Init()` 挂载） |
| **Scene** | `Game.Scene.Load(name, onProgress, onDone)` / `Unload(name, onDone)` / `CurrentScene` / `OnSceneLoaded` / `OnSceneUnloaded` | 场景管理（异步 + 加载门控 + 场景级回收）。**方法名是 `Load`，不是 `LoadScene`** |
| **UI** | `Game.UI.Open<T>(param)` / `Close<T>()` / `Close(name)` / `CloseAll()` / `Get<T>()` / `IsOpen<T>()` / `OnPanelOpened` / `OnPanelClosed` | UI 管理。面板实现 `IUIPanel`；预制体放 `Resources/UI/{面板类型名}`。**是 `Open`/`Close`，不是 `Show`/`Hide`** |
| **Animation** | `Game.Anim.CreateAnimator(go, controller)` / `Destroy(player)` | 动画（基于 Unity Animator；Spine / 龙骨由业务自接 SDK） |
| **Sound** | `Game.Sound.PlayBGM / StopBGM / PlaySFX / PlaySFXAt / PlayVoice / StopAll / SetVolume / GetVolume / SetMute`（`SoundGroup.BGM/SFX/Voice`） | 声音 |
| **Camera** | `Game.Camera.Follow(target, smoothTime)` / `Unfollow()` / `Shake(duration, intensity)` / `SetBounds(bounds)` | 相机 |
| **Quality** | `Game.Quality.Level / Config / IsThrottling / CurrentFPS` / `SetLevel(level)` / `AutoDetect()` / `OnLevelChanged` / `OnThrottling` | 设备性能等级与画质档位 |
| **SpriteAtlas** | `Game.Atlas.Load(name, cb)` / `Release(name)` / `GetSprite(atlas, sprite, cb)` / `UnloadAll()` | 图集（带引用计数） |

## 门面属性 ↔ 接口 ↔ 模块名

门面用短名（与 `Game.Res`/`Game.Net`/`Game.Sync` 的既有风格一致），下表用于消除"一个东西两个名字"的歧义：

| 门面属性 | 接口 | 模块名（架构文档） |
|---|---|---|
| `Game.Net` | `INetwork` | Network |
| `Game.Sync` | `IWorldSync` | WorldSync |
| `Game.Http` | `IWebRequest` | WebRequest |
| `Game.Schema` | `ISchemaRegistry` | SchemaRegistry（**不是** `Game.Data`，避免与服务端 `Game.Data()` 撞名） |
| `Game.Alert` | `IAlert` | Alert |
| `Game.CloverScene` | `ICloverScene` | CloverScene（服务端场景投影，**不是** `Game.Scene`） |
| `Game.FrameRoom` | `IFrameRoom` | FrameRoom |
| `Game.Res` | `IResourceManager` | Resource |
| `Game.Table` | `IDataTable` | DataTable |
| `Game.Setting` | `ISetting` | Setting |
| `Game.Pool` | `IObjectPool` | ObjectPool |
| `Game.Entity` | `IEntityManager` | Entity |
| `Game.Map` | `IMapData` | Map（逻辑地图投影，**不是** `Game.Scene`） |
| `Game.Atlas` | `ISpriteAtlasManager` | SpriteAtlas |
| `Game.Anim` | `IAnimationManager` | Animation |
| `Game.UI` / `Game.Scene` / `Game.Sound` / `Game.Camera` / `Game.Quality` | `IUIManager` / `ISceneManager` / `ISoundManager` / `ICameraManager` / `IQualityManager` | UI / Scene / Sound / Camera / Quality |

> **接收消息只有一个入口**：`Game.OnMsg(msgID, handler)`，与服务端 `g.OnMsg` 同名。
> `INetwork` **不含** OnMsg/OffMsg（路由独立为 `IRouter`，只经 `Game` 门面暴露），因此没有 `Game.Net.OnMsg` 第二入口，也没有 `OnPush` / `OnReceive` 之类的近似 API。
> **回包不注册回调**：`await Game.Net.Call<T>(msgID, req)` 按 requestID 自动配对。

## 常用 API 速查

### 网络通信

```typescript C#
// 初始化（第 1 个参数 = 网关 TCP 口，第 2 个 = 网关 UDP 口；对应 server.yaml 的 gateway.listen_tcp / listen_udp）
// ⚠️ 不要填 8001 —— 8001 是 WS 口，Unity 原生客户端走裸 TCP
CloverNet.Init("127.0.0.1:8002", "127.0.0.1:8003");

// 注册监听（客户端唯一入口，服务端同名 g.OnMsg；返回 void）
Game.OnMsg(EMsg.Xxx, ctx => { var msg = ctx.Bind<SomeNotify>(); });
Game.OffMsg(EMsg.Xxx, handler);   // 传 handler 精确移除；不传则移除该消息号全部

// 数据订阅唯一入口：Game.Sync（增量按 type 自行分派，全量走 OnFullSync）
Action<string, object> onData = (type, value) => { if (type == "player") { /* ... */ } };
Game.Sync.OnData(onData);
Game.Sync.OffData(onData);   // 用同一委托引用精确移除；Game.Sync.Clear() 则清空全部

Game.Sync.OnFullSync((data, accountData) => { /* ... */ });

// Schema 仅登记字段结构（不做订阅）
Game.Schema.RegisterSchema("player", new ObjectSchema().String("name", 0).Int("level", 1));

// 服务端场景投影（与 Game.Scene 不同义）
Game.CloverScene.OnChanged(s => Debug.Log($"scene={s.SceneID} instance={s.InstanceID}"));

// 可靠发送
Game.Net.Send(EMsg.Xxx, msg);

// 非可靠发送
Game.Net.SendUnreliable(EMsg.Xxx, msg);

// 请求-回包（回包类型为 ELoginReply 这类 E 前缀类型）
var reply = await Game.Net.Call<ELoginReply>(EMsg.Login, request);

// 登录成功后登记会话（断线恢复的前提）
// 第二个参数**传 null**（不是 reply.session_key；与官方 Sample 一致）。
// session_key 是会话通道加密密钥（AES 通道密钥），当恢复凭证上交会被判 token mismatch 踢掉；
// 真正的恢复凭证 session_token 由随后的 PushPlayerFullSync 下发并**非空覆盖**。
// 传非空值时引擎会打 Warn（NetworkManager.cs:715-718）。
Game.Net.SetupSession(account, null, line);

// HTTP 请求：回调式，没有 await/泛型
Game.Http.Get(url, resp => { if (resp.IsSuccess) { /* resp.Data 为 byte[] */ } });
Game.Http.Post(url, jsonBody, resp => { /* ... */ });
```

### 实体管理

```typescript C#
// 创建 / 获取 / 销毁
var e = Game.Entity.Create(objectID, typeID, group);
var e2 = Game.Entity.Get(objectID);
Game.Entity.Destroy(objectID);

// 绑定视图
Game.Entity.BindView(objectID, go);
```

### 对象池

```typescript C#
var go = Game.Pool.Spawn("Bullet", parent, "battle");
Game.Pool.Despawn(go);      // ★ 不是 Recycle
Game.Pool.ClearGroup("battle");
```

### 资源加载

```typescript C#
// 异步加载（回调式，不是 await）
Game.Res.LoadAsset<GameObject>("prefabs/bullet", obj => {
    if (obj != null) Instantiate(obj);
});

// 带进度
Game.Res.LoadAsset<GameObject>("prefabs/big", p => Debug.Log(p), obj => { /* ... */ });

// 释放 / 卸载
Game.Res.Release("prefabs/bullet");
Game.Res.UnloadAll();
```

### UI / 场景 / 声音 / 相机 / 设备（表现域）

```typescript C#
// —— UI：面板继承 UIPanel（已实现 IUIPanel 全部样板），预制体放 Resources/UI/{类名} ——
public class LoginPanel : UIPanel
{
    public override void OnOpen(object param) { /* 刷新数据 */ }
    // PanelName 默认取类名、Layer 默认 Normal、Root 默认 gameObject、OnClose/OnUpdate 已有空实现
}

Game.UI.Open<LoginPanel>();        // 打开（同名已开则重新 OnOpen 并置顶）
Game.UI.Close<LoginPanel>();       // 关闭
Game.UI.CloseAll();
var opened = Game.UI.IsOpen<LoginPanel>();
var panel = Game.UI.Get<LoginPanel>();

// 预制体来源默认查 Resources/UI/{类名}；接 Addressables/AB 时替换来源（建议 Launch 前设置）：
// CloverPresentation.PanelProvider = name => Addressables.LoadAssetAsync<GameObject>(name).WaitForCompletion();

// —— 场景：禁止直调 Unity SceneManager ——
Game.Scene.Load("MainCity", p => Debug.Log($"loading {p:P0}"), () => Debug.Log("done"));
Game.Scene.Unload("Login");

// —— 图集 ——
Game.Atlas.GetSprite("ui_common", "btn_ok", sp => btn.image.sprite = sp);
Game.Atlas.Release("ui_common");

// —— 动画（基于 Unity Animator；Spine / 龙骨等骨骼动画由业务自接 SDK） ——
var ap = Game.Anim.CreateAnimator(enemyGo, controller);
ap.Play("idle");
ap.CrossFade("run", 0.2f);
ap.OnComplete(() => Game.Anim.Destroy(ap));

// —— 声音 ——
Game.Sound.PlayBGM("bgm_main", 0.5f);
Game.Sound.PlaySFX("click");
Game.Sound.PlaySFXAt("boom", transform.position);
Game.Sound.SetVolume(SoundGroup.SFX, 0.8f);
Game.Sound.SetMute(SoundGroup.BGM, true);

// —— 相机 ——
Game.Camera.Follow(player.transform, 0.15f);
Game.Camera.Shake(0.3f, 0.5f);
Game.Camera.SetBounds(new Bounds(Vector3.zero, new Vector3(100, 100, 0)));

// —— 设备性能档（影响 Application.targetFrameRate / 阴影） ——
Game.Quality.AutoDetect();
Game.Quality.SetLevel(QualityTier.High);
Game.Quality.OnLevelChanged(lv => Debug.Log($"device level -> {lv}"));
```

> UI 点击依赖 `EventSystem`，而它归输入模块管理：请在 `Game.Launch` 后调用 `CloverInput.Init()` 
> （未挂接输入模块时 `CloverPresentation.Init` 会给出一次性告警，UI 仍可打开但点不动）。

### 事件总线

```typescript C#
Game.Event.On("MyEvent", () => Debug.Log("fired"));
Game.Event.On<MyData>("MyEventWithArg", d => Debug.Log(d));
Game.Event.Once("OnceEvent", () => Debug.Log("once"));
Game.Event.Emit("MyEvent");
Game.Event.Off("MyEvent", handler);   // 取消注册；没有 handler.Dispose()
```

### 定时器

```typescript C#
long id = Game.Timer.After(3f, () => Debug.Log("3s"));      // 返回 id
long id2 = Game.Timer.Every(1f, () => Debug.Log("tick"));   // 返回 id
Game.Timer.EveryName("heartbeat", 5f, cb);                  // 具名（同名先停旧）
Game.Timer.Stop(id);
Game.Timer.StopNamed("heartbeat");
Game.Timer.StopScope("battle");
```

### 状态机

```typescript C#
// ★ 不要 new Fsm()（internal）；直接用 Game.Fsm
Game.Fsm.RegisterState("idle",
    onEnter: () => Game.Logger?.Info("FSM", "进入空闲"),
    onTick: dt => { },
    onExit: () => Game.Logger?.Info("FSM", "退出空闲"));

Game.Fsm.AddTransition("click", "running");
Game.Fsm.Trigger("click");
Game.Fsm.Force("running");
string cur = Game.Fsm.Current;   // 不是 CurrentState
```

## asmdef 依赖

| asmdef | 所在目录 | 可引用 |
|--------|---------|--------|
| `CloverEngine.Core` | Runtime/Core | （无依赖） |
| `CloverEngine.Data` | Runtime/Data | Core |
| `CloverEngine.Network` | Runtime/Network | Core |
| `CloverEngine.Resource` | Runtime/Resource | Core |
| `CloverEngine.Presentation` | Runtime/Presentation | Core |
| `CloverEngine.Editor` | Editor | 全部 Runtime asmdef |
