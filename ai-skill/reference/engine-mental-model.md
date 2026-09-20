# 引擎心智模型与上手动线（做游戏前必读）

> 这份文档来自一次完整的"从零做一个可玩 MMO 客户端"的实战复盘：**先把下面这些搞清楚，
> 再动手写代码**，能省掉绝大部分返工。每条都标注了出处/实测结论。

---

## 1. 客户端模块「谁自动挂 / 谁必须业务挂」（最容易翻车）

`Game.Launch(config)` 之后：

| 模块 | 谁挂 | 说明 |
|---|---|---|
| Logger / Event / Timer / Fsm / Dispatcher / Setting | `Game.Launch` 自动 | 基础域 |
| Net / Sync / Schema / Alert / CloverScene / FrameRoom / Http | **业务** `CloverNet.Init(addr, udpAddr)` | 不调就完全没网络（静默） |
| Res | **业务** `CloverRes.Init(root)` | ⚠️ **不挂 ⇒ `Game.Res` 为 null**：模型/贴图静默加载失败（只剩占位体），并在加载点抛 `NullReferenceException` 打断主流程。**实测踩过** |
| Input | **业务** `CloverInput.Init()` | 同时创建 EventSystem + 匹配后端的 InputModule（UI 点击依赖它） |
| Table / Localization | **业务** `CloverData.InitDataTable(dir)` / `InitLocalization(...)` | 不调就没配表 |
| Map / Entity / Pool / UI / Scene / Atlas / Anim / Sound / Camera / Quality | `CloverPresentation.Init()`（**随 Launch 自动**，`AutoMount=true`） | 表现域全自动（Map 为纯数据模块） |
| LanBrowser | **随 Launch 自动**（`CloverLan` 的启动钩子；仅原生平台） | 局域网寻服：`Game.LanBrowser.Scan()` 找同网段主机 → 选一台 → 再 `CloverNet.Init(host.Address, host.UdpAddress)`（**必须早于** `CloverNet.Init`） |

**业务启动顺序（照抄）**：

```csharp
Game.Launch(new GameConfig { ServerAddr = "...:8002", ... });
CloverRes.Init(Cfg.Game.res_root);      // 资源（不做就等着 null）
CloverInput.Init();                     // 输入 + EventSystem
CloverNet.Init(Cfg.Server.addr, Cfg.Server.udp_addr);   // 网络
// 之后才是 Game.UI.Open<XxxPanel>() / 业务订阅
// 然后交给流程模块进「启动画面 → 主菜单 → 创角/选角 → 读条进图」
//   —— ★ 不许直接进游戏场景（patterns/client/app-flow.md）
```

## 2. 生命周期顺序（决定你把代码写哪）

```
Awake()        ← 引擎还没 Launch：**禁止**碰 Game.Event / Game.UI / Game.Res
Start()        ← 在这里 Game.Launch(...) + 各模块 Init + 订阅
Update()       ← Game.Tick 由引擎的 EngineRunner 驱动（Camera/Anim 等模块也在这里 Tick）
```
**实测踩过**：在 `Awake()` 里 `Game.Event.On(...)` → `NullReferenceException`。事件订阅一律放 `Start()`。

## 3. 一条数据的完整链路（MMO 玩法的骨架）

```
客户端输入(Game.Input) → 业务上行(Game.Net.Send/SendUnreliable, 业务消息号 ≥10001)
   → 服务端 g.OnMsg(handler): 校验 → scene/数据层改动（Load-Modify-Return）
   → 广播(PushToPlayer/PushToPlayerJSON) 或 回包(g.Reply)
   → 客户端 Game.OnMsg(消息号) 或 Game.Sync.OnEntity*/TryGetPosition（插值后）
   → 业务视图（模型/动画/HUD）
```
- **位置表现读 `TryGetPosition`（插值后）**，`OnEntityMove` 给的是服务端原始目标坐标（离散）；
- **自己**的位置要本地预测（见 §5），**不要**用插值覆盖自己。

## 4. 表现域能力边界（先查再写，别自己造轮子也别硬套）

> ⛔ **硬闸门：动手写任何"通用设施"之前，先在这张表里找一遍。**
> UI 构建、对象池、存档/设置、图集、本地化、配表、动画、相机、场景、音效 ——
> **只要属于这十类，就必须先确认引擎有没有现成的；有就用引擎的。**
>
> **为什么单列这条**：实测一个项目手搓了 `UIBuilder`（7.7KB，节点/文本/按钮/铺满全自己写），
> 而引擎的 `UIWidgets` 里**这些全都有**（含专门的 `Stretch(rt)`）—— 业务不知道，
> 于是白写一遍，还因为"面板根节点没铺满"踩了坑（引擎的 `Stretch` 正好能防住）。
> **这份表当时只列了 Camera/Anim/UI/Input/Res 五项，漏了下面标 ★ 的那几行** ——
> **清单不全 = 业务必然重造轮子**。看到表里没有的通用需求，先去引擎 `Runtime/**` 搜一遍再动手。

| 模块 | 真实能力 | 缺口/注意 |
|---|---|---|
| `Game.Camera` | `Follow(target, smoothTime)` / `Unfollow` / `Shake` / `SetBounds` | **`Follow` 是"锁 Z 的简单跟随"**（无环绕/无旋转/无防穿墙）⇒ 第三人称请业务自写；**只要不调 `Follow`，引擎不会抢相机 Transform**。**2D 跟随 + 夹关卡边界直接用 `SetBounds`**。另有相机侧三个**纯件**（E-core-18 下沉；**不经 `Game.Camera` 门面**，业务自持 / 自传配置）：`ViewBob`（第一人称视点晃动 + 落地沉降，纯逻辑类 + `ViewBobConfig` 七个数值）、`CameraMath`（`FovYFromFovX` 水平→垂直 FOV / `AimDirection` yaw,pitch→视线方向 / `Follow` 指数平滑）、`LookAccumulator`（鼠标位移 → yaw/pitch 累加；符号与夹紧口径由调用方给） |
| `Game.Anim` | `CreateAnimator(go, RuntimeAnimatorController)` → `IAnimPlayer`（`Play/CrossFade/SetFloat/SetBool/SetInteger/SetTrigger/OnComplete`） | **`AnimatorController` 只能 Editor 侧生成**（`using UnityEditor.Animations`）。**注意**：它是"Unity Animator 驱动"的模型；**代码逐帧切 Sprite 的轻量动画它不覆盖**（那类要么改用 AnimatorController，要么业务自写） |
| `Game.UI` | `Open<T>(param)` / `Close<T>()` / `CloseAll` / `Confirm`；面板预制体查 **`Resources/UI/{类名}`**，找不到只打 Error 且**不打开**；内置层：`ToastLayer` / `FloatTextLayer` / `LoadingLayer` / `ConfirmLayer` / `GuideLayer` | 纯代码搭 UI 也要生成真预制体（用 `CloverPresentation.PanelProvider` 可自定义来源）。⚠️ **`param` 在 `Awake` 之后才到**：面板**不要在 `Awake` 里读上下文**取状态值，用 `OnOpen(param)` 刷（实测踩过：读条屏命数恒显示默认值） |
| ★ **`CloverEngine.UIFactory`** | **`CreateNode / Stretch / CreateCentered / CreatePanel / CreateText / CreateButton / DefaultFont / UICamera`** —— 代码搭 uGUI 的全套脚手架 | **E2 起对业务公开**：业务"用代码搭 UI"一律用它，**不要再手写锚点/铺满/文本的工具类**（那类重复实现是踩坑高发区）。`Stretch(rt)` = 根节点铺满父层，`CreateCentered` = 相对父层中心定位。<br>❗**注意**：`DefaultFont()` 是引擎内置字体；**像素风项目要自己的像素字体**（字号还得按字体规格取整），那层包装仍归业务。通用件（Toast/飘字/Loading/确认框/引导）走 `Game.UI`，不要直接碰那些 internal 的 Layer |
| ★ **`CloverEngine.UIFactory` 通用控件工厂**（E-core-16） | `CreateSlider` / `CreateInputField` / `CreateSelector`（「◀ 值 ▶」选择行）/ `CreateToggleRow`（开关行）+ 布局助手 `Place` / `AnchoredTopLeft` / `AnchoredBottom` / `CreateLabel` / `CreateBoxRect` / `CreateBottomLabel` + 进度条比例 `SetBarWidth`（**锚点宽度**口径）+ 句柄类型 `CloverEngine.Selector` / `CloverEngine.ToggleRow`（都在 `Presentation` 程序集，`Runtime/Presentation/UIWidgetControls.cs`） | ⛔ **别再自己造锚点/控件工具类，也别自建 Slider/InputField/Selector/ToggleRow**（实测同一个 uGUI 控件在项目里被造了三遍、三份互指对方有坑）。**配色 / 文案 / 字号 / 回调一律由参数传入**（`Widget*Style` 结构体），引擎不含任何项目取值。三个已封在里面的坑：① `SetBarWidth` 走锚点宽度 —— 空 sprite 的 `Image.fillAmount` **静默失效**；② InputField 的 `placeholder` 声明类型是 `Graphic`，必须 `as Text` 才取得到 `.font/.fontSize/.text`（否则 CS1061）；③ Slider 别用 `DefaultControls.CreateSlider`（配色在私有层级里）。**安全区仍归业务面板**（`Screen.safeArea`），控件工厂只管摆放 |
| `Game.Input` | `State.MoveDirection`（WASD 已归一）、`GetKey/GetKeyDown/GetKeyUp(GameKey)`、`GetMouseButton*/MouseDelta/MousePosition/GetAxis`、`Lock/Unlock` | **后端无关**（Legacy/InputSystem 自动探测）⇒ **业务永远用 `Game.Input`，不要直连 `Keyboard.current`**（旧后端下为 null） |
| `Game.Res` | `LoadAsset<T>(path, cb)` / `Release` / `Preload` / `UnloadAll` / **`TryGet<T>(path)`**（同步取**已驻留**资源；不触发加载、不阻塞、纯读）/ **`Exists(path)`**（E-core-09：**同步回答"在不在"**，不驻留、不动引用计数；Resources 后端探测一次并按路径缓存）/ **`LoadAll<T>(path)`**（E-core-10：**同步批量取**，会加载、不进缓存；条带/图集整条取）；热更成员另有 `CheckUpdate` / `DownloadUpdate` / `ClearDownloaded` / `Version` / `UpdateState` | 路径相对 `Resources`（`CloverRes.Init(root)` 决定前缀）。**无阻塞式单资源加载，但有同步批量取** —— 要"立刻拿到一个"就**先 `Preload`、后 `TryGet`**；要"这条在不在"用 `Exists`；要"整条帧序列"用 `LoadAll`。⛔ **别用 `Resources.Load*` 绕开 `Game.Res`**（缓存 / LRU / 根前缀 / 热更后端全失效，换后端时静默不跟着变；`Exists`/`LoadAll` 就是为消掉这类绕开才加的） |
| ★ **`CloverEngine.Rng`**（E-core-05） | 注入式**可复现**随机：`Next / Next(max) / Next(min,max) / NextFloat / Range / Chance / Pick / PickWeighted / Shuffle / NextGrid / Index` + `FromTime / Derive / DeriveSeed` | ⛔ **禁止 `UnityEngine.Random`**（全局静态状态 ⇒ 序列不可复现，地图/掉落对不上）。由调用方持有实例并**显式传参**；非法参数**不抛异常**（返回安全值 + 限频告警） |
| ★ **`CloverEngine.LogThrottle`**（E-core-06 时间口径 / **E-core-14 计数口径**） | **两种口径**：① 时间口径 `ShouldLog / WarnThrottled / ErrorThrottled / WarnOnce / ErrorOnce`（同一 key 在间隔内只出第一条；空 key 恒 false + 只报一次）；② **计数口径** `ShouldLogEvery(key, everyN)` / `InfoCounted` / `WarnCounted` / `ErrorCounted`（每 key 独立计数、**第 1 次必打**、之后每 N 次一条，行尾补 `（同类第 N 次）`；空 key 归并 `"default"`；`everyN<=1` 视为 1）+ `Reset()`（**两种记录一起清**）+ 可注入 `Clock`（`ClockSource` = Injected/Unity/Process） | 高频回调里的非预期分支**必须**走它（否则刷屏打爆日志）；**两种口径语义不同、不可互相替换**（时间治"每帧刷屏"、计数治"偶发但一局出现很多次"的打点抽样）；**离线宿主必须注入 `Clock`** 才有确定性（不注入会自动降级到 `Stopwatch` 并只报一次） |
| ★ **`CloverEngine.LogBuffer`**（E-core-15） | 运行时日志**环形缓冲**（最近 N 行的只读窗口）：`Capacity` / `Version` / `Lines` / `Installed` / `Install(capacity = 400)` / `Uninstall` / `Drain()` / `Push(text)` / `Clear()`；入队线程安全（挂 `Application.logMessageReceivedThreaded`），`Version` / `Lines` / `Drain` 只保证主线程读；默认 `BeforeSceneLoad` 自动装一次 | **要做"游戏内控制台看最近日志"就直接用它**，⛔ 别再自己挂 `logMessageReceivedThreaded` + 手写环形缓冲；它**不是第二套 logger**（只收行 / 不写行，`ILogger` 一行未动）；"面板怎么画"仍归业务 |
| ★ **`CloverEngine.AStar`**（E-core-07） | `Find / FindSmoothed / Smooth / HasLineOfSight / Describe`，**回调式** `Func<Vector2Int,bool> walkable` ⇒ 零业务类型依赖 | 运行时寻路。⚠️ 引擎 `MapBake` 是**静态烘焙**（不解决"每局随机生成"的地图）⇒ 随机地图/运行时格子寻路用这个，⛔ 别再自写 |
| ★ **`CloverEngine.IsoLayout`**（E-core-08） | 等距正/逆投影（`GridToWorld` / `WorldToGrid` / `ScreenToGrid`…）、`SortOrder`（含 `layerOffset` 重载）、`GridDistance*`、`DirectionTo`（构造收 `halfW/halfH/sortOrderStep/sortBase`） | 2.5D/等距游戏通用；⛔ 别把格宽/排序步长硬编码进业务（做成 `IsoLayout` 的构造参数） |
| ★ **`CloverEngine.CloverTable`**（E-core-11） | `LoadAll(streamingAssetsDir, dataDir)`（成功 `null` / 失败 **可定位错误串**）+ `Get<T>(tableName, int\|string key)`（反射填 public 字段、按 (表,类型,列) 缓存）+ `Dir` / `ResolveDir` / `RequiredTables` | 读**自家打表工具**的产物（tsv + 生成的强类型行类）。⚠️ 引擎旧入口 `CloverData.InitDataTable` 要求行类实现 `IDataRow`、**读不了打表产物** ⇒ 工程侧一律用这个；打表生成的 `Tables.Default.*` 强类型壳仍可继续用（它是"便捷访问层"，不是加载器） |
| ★ **`CloverEngine.TextHooks`**（E-core-12） | `Current`（`ITextHook`）+ `NotifyCreated(Text)`：引擎通用件（`UIFactory.CreateText` 是**唯一** Text 创建点）每建一个 `Text` 都会通知挂钩 | **像素风/自备字模的项目必备**：`null` 时行为与"只用引擎内置字体"逐字一致；注册后引擎自带的 Toast / Loading / Confirm / Guide / 飘字也走你的字模。⚠️ 两个坑：① 挂钩里**不许再调 `UIFactory.CreateText`**（会递归爆栈）；② 字模依赖 `Game.Res` 时，`Game.Launch` 期间建的 Text 要**寄存到资源就绪后再挂**（否则字模图集加载失败会把"字模不可用"标成静态开关 = 一局内全项目降级） |
| ★ **`Game.Timer`** | `After / After(scope) / Every / AfterName / EveryName / Stop*` + **`AfterUnscaled` / `EveryUnscaled`**（E1 新增） | **`timeScale = 0` 时普通 `After` 永不触发**（引擎用 `Time.deltaTime` 推进）。暂停菜单 / 结算屏 / GameOver 这类"冻结画面里还要走时间"的**一律用 `AfterUnscaled`** |
| ★ **对象池** | `ObjectPool`：`Spawn(key,parent,group) / Despawn / Preload / Clear / ClearGroup / ClearAll / GetActiveCount / GetInactiveCount`；另有**空闲过期回收** `IdleExpirySeconds`（默认 0 = 关闭）/ `TrimIdle()`（契约 `Runtime/Core/EntityPool.cs:147-163`，实现在 `Runtime/Presentation/ObjectPool.cs:21-32,50-78,123-124`）（`Runtime/Core/EntityPool.cs` 只是**契约文件**：`IEntityManager` / `IObjectPool`，不是可用的池类型） | **手写"`new GameObject` + `List` + 自己 Reap"前先看这里**（子弹/敌人/特效/金币这类高频增删）。`IdleExpirySeconds>0` 时闲置超时的空闲对象在下一次 `Spawn`/`Despawn` 里被销毁（惰性，不新增 Tick） |
| ★ **引用池 `ReferencePool`** | `ReferencePool.Acquire<T>() / Release(T) / Count<T>() / Clear<T>() / ClearAll()`（`Runtime/Core/EntityPool.cs:321-404`） | **纯 C# 托管对象**的复用池（无 GameObject、无 Unity 依赖），与上面的 `Game.Pool` 是**互补**而非替代：治"每帧 new 一堆短命对象顶 GC"（输入帧 / 事件参数 / 临时 List）。对象可选实现 `IReferencePoolable.OnAcquire/OnRelease`（`:286-293`）自动复位；**主线程专用**；同一实例重复归还被忽略并告警 |
| ★ **`Setting`** | `Get<T>(key, default)` / `Set<T>(key, value)` / `Save / Load / Delete / DeleteAll` | **存档/设置/最高分别裸用 `PlayerPrefs`**，走引擎这一层 |
| ★ **`CloverEngine.FileSlotStore`**（E-core-13） | 「键 → 文本」的**槽位**存储（**一槽一文件**）：`Exists` / `Write(key, content, out error)` / `Read(key)` / `Delete(key)` / `List()`（**字典序**）/ `LastCorruptPath` / `Dir`；构造 `(dir, extension = ".json")` | **"一只角色一个文件 / 一局回放一个文件 / 一章关卡草稿一个文件"就用它**，⛔ 别再自己写「`.tmp` + `File.Replace` + 坏文件留档 + 目录枚举」（那类重写是 D 桶浪费）。与 `Setting` **互补**：那个是单文件 KV（设置在内存里攒、一次写全），这个是每次 `Write` 独立落盘、`List()` 可枚举。⚠️ `List()` 是**字典序、不是插入序** ⇒ 要"创建先后"（选角屏卡片顺序这类）**自己维护索引键**。`.json` 槽会校验内容可解析：坏档 ⇒ `Read` 返回 `null` + 留档 `.corrupt`（**副本**，现场不丢）+ 限频告警一次；`Write` 遇到坏档先留档再写 |
| ★ **`SpriteAtlas`** | 图集加载/取图 | 大量小图别再散装成几百个 PNG（像素游戏用图集要留意 PPU / `FilterMode`） |
| ★ **配表 / 本地化** | `CloverData.InitDataTable(...)` / `DataTable` / `Localization` | 同质化配置与 UI 文案不要硬编码、也不要自己写解析器 |
| ★ **`Game.Sound`** | `PlayBGM / StopBGM / PlaySFX / PlaySFXAt / PlayVoice / StopAll / SetVolume / GetVolume / SetMute`（`SoundGroup.BGM/SFX/Voice`）＋ 播放闸门 `MaxPlaysPerFrame` / `MaxConcurrentPerClip`（E-core-17，默认 `0` = 不限） | BGM 双音源交替 + 淡入淡出；路径约定 `Sound/{BGM,SFX,Voice}/{name}`（走 `Game.Res`）。**音量/静音只在内存、不持久化**——要存设置自己写 `Game.Setting`。★ **播放闸门（E-core-17）**：① 缺失**整进程只报一次**（四处 `clip == null`：`PlayBGM`/`PlaySFX`/`PlaySFXAt`/`PlayVoice`，全走 `LogThrottle.WarnOnce("Sound","missing:<path>")`，⛔ 不再每次一条裸 Warn；`Sound.cs` 内**已无裸 `Game.Logger?.Warn`** —— 池满告警也走 `LogThrottle.WarnOnce`）；② `MaxPlaysPerFrame`（单帧最多**真正起播**几次）/ `MaxConcurrentPerClip`（同一路径**同时播放**的音源数上限，真并发口径）—— 默认都是 `0` = **不限**（不改任何既有表现），超限**丢弃该次播放**（⛔ 不排队、⛔ 不打断在播音源）+ `LogThrottle.WarnThrottled` 限频告警；只作用于 `PlaySFX`/`PlaySFXAt`/`PlayVoice`（BGM 不受影响）。阈值的**取值**仍是业务的（引擎只执行口径），例如 cs16 在 `AudioModule.Start` 把 `CsAudioTuning` 的两个常量下发给引擎。⛔ 别在业务侧再写探测缓存 / 帧计数 / 并发表（那是重复的闸门）；要"问一句这条音效在不在"用 `Game.Res.Exists`（引擎按路径缓存） |
| ★ **地图 / 场景** | `Game.Map`（`IMapData`：`LoadFromResource` / `Load` / `WalkableAt`；格式层 `CloverMapFormat` / `CloverMapData` / `CloverMapWriter` 属引擎 internal）；`Game.Scene.Load` | 有现成地图格式；**但它是偏 3D 的**，2D 平台跳跃要不要用需先评估格式适配度，别硬套 |

## 5. 主角操作的三条铁律（做 3D/2.5D 玩法必看）

1. **输入走 `Game.Input`**（见上表）；`State.MoveDirection` 直接可用；
2. **本地预测 + 服务端校正**：本地立即移动（手感），20Hz 上行；服务端权威位置只做校正
   （<0.3m 忽略 / 0.3~1.5m 缓收 / >1.5m 硬贴 + 告警）。**绝不能用权威位置逐帧覆盖自己**   
   —— 那是"按了 WASD 不动/漂移"的经典根因（实测踩过）；
3. **相机自写第三人称**：`LateUpdate` 跟随 + `SphereCast` 防穿墙 + 收缩快/拉伸慢，避免抖动。

## 6. 素材接入的"量测优先"原则（第三方 FBX/GLB 通用）

各素材包**单位/枢轴/朝向都不一致**，不要假定：

```
实例化 → 实测 Renderer 包围盒（合并所有子 Renderer）
      → 按目标尺寸反算缩放（迭代修正一次）→ 按包围盒贴地/居中
      → 需要碰撞时**自己 AddComponent<BoxCollider>()**（FBX 默认不生成碰撞体！）
```
**两个已踩坑**：① FBX 不自动带 collider ⇒ 烘焙出的地图会是"0 障碍物"的空地图（`Clover/地图烘焙` 的日志会自证：`计为障碍=0`）；
② 把模型**中心**对到**格角点**会整体偏移半格（4m 砖覆盖 [x-2,x+2)）⇒ 一律传格中心。

## 7. 怎么"看见"引擎在干什么（验证手段，别猜）

| 手段 | 用法 |
|---|---|
| **服务端探针消息** | 业务加一条"给点返回权威状态"的消息（本项目 `MsgGridProbe` 返回可走性 + 命中碰撞体 id），把不可见的逻辑变成可见断言 |
| **运行日志** | 每个非预期分支都打日志（服务端 `logger`、客户端 `Game.Logger`）；高频路径"首条 + 每 N 条" |
| **运行时自证** | 编辑器 Play 中 `eval_file` 打印关键对象状态（组件是否挂上/目标是否绑定/模型是否加载/Animator 当前 clip） |
| **截图** | `capture_game_view --source screen`（Play 模式，含 Overlay HUD） |
| **既有测试** | 端到端 PlayMode 用例（登录→进图→探针→视野事件断言）作为回归 |

## 8. 静默失败清单（这些都不报错，但结果错）

| 静默现象 | 真因 |
|---|---|
| 模型不显示、只有占位体 | `CloverRes.Init` 没调（`Game.Res` 为 null） |
| 客户端收不到某类推送 | 推送线格式与客户端解析器不匹配（例：客户端只认外层 key=事件名的 `EPushDataSync`，其他形状**静默丢弃**） |
| AOI 事件"算了但没人收" | `WireEntitySync(...)` 没接（引擎算得出，但没有订阅者） |
| 服务端导出"0 障碍物" | 场景物体没有 Collider（FBX 不带，需自己加） |
| 配表整表被跳过 | 源表名没带 `_cs` / `_c` / `_s` 后缀 |
| 面板打不开 | `Resources/UI/{类名}` 预制体缺失（只打 Error 后 return） |
| WASD 无反应 | 直连 `Keyboard.current`（旧后端为 null）／自己的位置被权威位置覆盖 |

> **写代码前先过一遍这张表**；写完再按 §7 自证。这样就很难交付"看着像那么回事"的半成品。
