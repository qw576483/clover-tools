# 代码分层与模块化规范（强制，违反即返工）

> 目标：**一个系统一个模块、依赖单向、谁都别当上帝类**。
> 客户端业务代码**必须**按下表分层；服务端沿用 `game/{def,datadef,logic,table}` 四段（见 `scaffold/new-project.md`）。

---

## 1. 客户端目录 = 分层（强制）

```
client/Assets/Scripts/
├── Def/                    # 协议真源：消息号 + 请求/回包/推送结构体（镜像 server/game/def）
├── Core/                   # 与玩法无关的地基：配置、常量、事件名、数学/工具
│   ├── ClientConfig.cs
│   ├── Events.cs           # ★ 事件名常量集中一处（禁止业务里写裸字符串）
│   └── ...
├── Module/                 # ★ 业务系统，一个系统一个目录（互不直接引用）
│   ├── Flow/               # ★ 流程编排：启动/主菜单/创角/选角/读条进图/暂停/回菜单（App Flow，见 patterns/client/app-flow.md）
│   ├── Net/                # 网络门面：收发包、消息路由、重连/重登状态机
│   ├── Map/                # 本地碰撞解算（MapModule；数据与空间事实来自引擎 Game.Map）
│   ├── Player/             # 主角控制（PlayerMotor）
│   ├── CameraRig/          # 第三人称相机（ThirdPersonCamera）
│   ├── View/               # 实体视图（EntityView 工厂 + 视图更新）
│   ├── Combat/             # 战斗（攻击/伤害/受击/死亡/复活表现）
│   └── Audio/              # 音效（脚步/挥砍/命中/UI）
├── UI/                     # 面板：只读数据 + 发事件，不引用业务模块类
（含菜单/创角/选角/设置/暂停/结算等流程面板）
└── App/                    # ★ 唯一组装点：Bootstrap（装配 + 生命周期），不含业务逻辑
```

**依赖方向（单向，反过来就是错）**：

```
App ──▶ Module ──▶ Core ──▶ Def
 │        │
 └──▶ UI ─┘（UI 只允许 Core/Def + 事件总线）
```

- `Module/A` **不许** 引用 `Module/B` 的具体类；要协作就 **① 事件总线**（`Game.Event`）或 **② App 注入的接口**；
- `Core`/`Def` **不许** 反向引用任何 Module；
- `UI` **不许** `using` 任何 `Module.*`（只发/收事件 + `Game.UI.Open<T>(param)` 带参）；
- **网络调用只允许出现在 `Module/Net` 与 `App`**（装配期），其它模块一律调 `Net` 门面方法（便于统计/重放/校验/改协议）；
- **地图碰撞数据只允许 `Module/Map` 读**，其它模块只用它导出的只读 API；
- `Assets/Editor/` 下的工具只依赖 `Def` + `Core`（不得引用运行期 Module）。

## 2. 每个模块的样子（模板）

```csharp
// Module/Combat/CombatModule.cs —— 模块**唯一对外门面**
namespace CloverMmo1.Module.Combat
{
    public interface ICombat
    {
        void RequestAttack();                 // 外部只能通过这些方法用本模块
        int HitCount { get; }
    }

    internal sealed class CombatModule : ICombat { /* 内部只被 App 构造 */ }
}
```

规则：

- **一个模块 = 一个门面接口 + 若干内部类**；MonoBehaviour 全部 `internal`，
  只在模块目录内 `AddComponent`；
- 模块之间**不允许**通过 `FindObjectOfType` / `GameObject.Find` / 全局单例互相找；
- 模块拿到依赖**只能**经构造函数/`Init(...)` 注入（由 `App` 统一装配）；
- 事件名必须来自 `Core/Events.cs` 常量，禁止裸字符串（改一处能全局生效）。

## 3. App（Bootstrap）该多大（硬指标）

`App/Bootstrap.cs` **只做三件事**：`Game.Launch` + 各模块 `Init` + 生命周期转发；**启动后立刻交给 `Module/Flow` 进流程（启动画面 → 主菜单），不许直接进游戏场景**（`patterns/client/app-flow.md`）。

- 目标：**≤ 200 行**；超过就是"上帝类在长肉"，必须把逻辑挪进对应模块；
- **禁止**在 App 里出现：业务判断（伤害/胜负/掉落）、逐帧业务循环、资源加载细节、
  具体的 `Game.Net.Send`（除装配期的登录流程外）；
- 逐帧更新由各模块自己 `Update`（MonoBehaviour）或 App 转发 `Tick`，**App 不写业务每帧逻辑**。

**反面教材（本项目第一版真实踩过，引以为戒）**：`Bootstrap` 里同时塞了
登录流程 + 实体视图创建/销毁 + 模型加载与归一化 + 动画装配 + HUD 刷新 +
热键/探针/刷假人 + 相机装配 —— 结果：任何一处改动都要动同一个文件，
一次网络异常引发全部功能瘫痪。**这就是"耦合"的代价。**

## 4. 自检（交付前必须跑）

```bash
# ① 上帝类体检：App 行数
wc -l client/Assets/Scripts/App/*.cs

# ② 跨模块耦合体检：Module/X 里出现 Module/Y 的具体类型
grep -rn "Module\." client/Assets/Scripts/Module/ | grep -v "本模块命名空间"

# ③ UI 引用了业务模块（应 0 命中）
grep -rn "using CloverMmo1.Module" client/Assets/Scripts/UI/

# ④ 业务模块里直连网络（除 Net 模块外应 0 命中）
grep -rn "Game.Net\." client/Assets/Scripts/Module/ | grep -v "/Net/"

# ⑤ 裸事件名（应全部来自 Core/Events.cs 常量）
grep -rnE 'Game\.Event\.(On|Emit|Off)\("' client/Assets/Scripts/

# ⑥ 硬规则命中（应 0 命中；有例外必须已在「策划/验收表.md → 允许的差异」登记）
grep -rnE 'Debug\.Log|Resources\.Load|PlayerPrefs|GameObject\.Find|FindObjectOfType' client/Assets/Scripts/
grep -rnE '(^|[^.])\bInput\.(GetKey|GetMouse|mousePosition|GetAxis)' client/Assets/Scripts/   # 裸 UnityEngine.Input

# ⑦ 临时文件位置（应 0 命中：探针/一次性脚本只许在 <项目根>/.ai-tmp/）
grep -rn --include=*.cs --include=*.ps1 '' client/_dev _assets_src _assets_tmp 2>/dev/null | wc -l
```

> ⑥⑦ 与 §1~§5 一样是**交付前必跑**（完整 **7 条**见 `SKILL.md` §1.11）。
> 实测教训：⑥⑦**原来没有**，于是「临时文件遍地」「`Resources.Load` 绕过 `Game.Res`」这类违反
> **不留任何痕迹**，交付时"看起来完全合规"。

> 上面 ②③④⑤ **允许为 0 命中**；有命中就必须在交付说明里给出理由，否则返工。

## 5. 服务端分层（沿用仓库既有约定，不要另发明）

```
server/game/
├── def/        消息号 + 请求/回包/推送结构体（真源）
├── datadef/    数据 schema
├── logic/       handler + 业务规则（按系统分文件：world.go / handlers.go / combat.go / bot.go …）
└── table/      配表访问
```

- handler **只做**：取参 → 校验 → 改数据/调引擎 → 回包/广播；**不写**大段算法（挪到同目录的 `xxx_core.go` 或独立包）；
- 一个系统一个文件组；跨系统协作用事件/接口，不要互相 import 内部函数表；
- 所有非预期分支必须打日志（见 `SKILL.md` 日志硬约束）。
