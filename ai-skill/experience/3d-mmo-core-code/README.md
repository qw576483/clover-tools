# 3D MMO 核心演示代码（**只看做法，不照抄整文件**）

> ⚠️ **这些示例是「从 0→1」的通用写法，不是可以直接复制的 API。**
> 动手前先走 `SKILL.md` 的「★ 混合模式找依据」：**用户 skill / 用户文档 / 用户源码 → 引擎源码 / 引擎文档 / 引擎 skill（含这里）→ 联网 → 自己设计**；这里是"引擎 skill"那一档的通用做法，**给做法、不给能直接抄的 API**。
> 用户项目可能已经有一套完全不同的模块名 / 入口 / 管理器 / API —— **冲突时以用户项目为准**，
> 这里只提供"做法与形状"，落到用户项目时必须换成它真实的 API（查不到出处就不许写）。

> 这里**只放"决定成败的那几十行"**：每段都是"去掉它就会出 bug"的关键逻辑。
> **不搬整份文件**——完整实现见真实工程，写业务时照这个形状自己写。
>每段按 `reference/architecture.md` 的**分层**标注落点：属于哪一层、哪个模块、对外只暴露什么。
> 目标形态（写新代码时按这个来，禁止上帝类）：

```
App/Bootstrap（≤200 行，只装配） → Module/{Net,Map,Player,CameraRig,Combat,Audio}（各一个门面） → Core → Def
UI（只发/收事件，不引用 Module）
```

| 文件 | 分层落点 | 只看这一段里的什么 |
|---|---|---|
| `01-map-collision.cs` | `Module/Map`（静态工具，无依赖） | 同源位图判定 + **分轴滑墙** + **扫掠细分防穿墙** |
| `02-player-motor.cs` | `Module/Player`（门面注入 `SendMove`，不直连网络） | 本地预测 + **seq/ack 回放** + **停发闸门** |
| `03-third-person-camera.cs` | `Module/CameraRig`（纯表现，无业务依赖） | 多高度探针 + **被挡允许拉近** + 重叠兜底 |
| `04-net-reconnect.cs` | `Module/Net`（唯一允许碰 `Game.Net` 的模块） | 重连事件→动作 + **重入保护** + 进图闸门 |
| `05-server-move-correction.go` | `server/game/logic` | 拒绝移动时**必须回校正（带 ack）** |
| `06-editor-measure-place.cs` | `Assets/Editor`（只依赖 Def/Core） | 量测优先（缩放/贴地）+ **主动补 Collider** |
| `07-server-combat.go` | `server/game/logic`（一个系统一个文件组） | 属性多返回值顺序 + **唯一结算入口** + **接收者=观看者** + 靶子要"够得着/会还手/原地重生" |

**配套**：做法出处与验收信号见 `experience/3d-mmo-playbook.md`；症状速查见 `patterns/client/3d-mmo-basics.md`。
