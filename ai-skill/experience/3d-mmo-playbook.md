# 3D MMO 落地手册（从零到可玩，9 步）

> 这是**实践顺序**版：每步写清「做什么 / 正确做法 / 代码落点 / 验收信号」。
> 症状速查用 `patterns/client/3d-mmo-basics.md`；强制分层用 `reference/architecture.md`。

---

## 步 0：读文档 + 环境自检（10 分钟，省掉几小时）

- 读：`SKILL.md` → 本文件夹 → `reference/engine-mental-model.md`（**模块挂载矩阵 + 启动顺序**）→ `patterns/client/3d-mmo-basics.md`；
- 自检：服务端依赖（`env.exe info`）+ Unity（`unity --version`、`editors list` 里必须有 **带 `location`** 的 6000.x）+ CLI；
- **建工程后立刻登记 Hub 并置顶**（`unity projects add` + `pin`），交付说明写清「Hub 里的名字 + 绝对路径」；
- 验收：用户能在 Hub 里一眼看到工程。

## 步 1：冻结契约（两端先对齐，再写业务）

- 客户端 `Def/MsgDef.cs` + `Def/ProtoDef.cs` ↔ 服务端 `def/{msg,reply,push}.go` **逐条对齐**（字段名即 JSON key）；
- 约定：C2S `1000xxx`、推送 `3002xxx`、回包不占号；
- 验收：`go build` 通过 + 客户端编译通过；消息号在两端 `grep` 数量一致。

## 步 2：地图管线（**已下沉引擎**，业务只摆场景 / 填参数 / 摆出生点）

> **2026-09 起地图管线是引擎能力，不要自己写导出器 / 加载器 / 解码器。**
> 契约与边界见 `clover-server-engine/pkg/domain/mmo/mapdata/README.md`：
> 格式契约（CloverMap 二进制 v1）、Unity 烘焙器（`Editor/MapBake/`，面板 `Clover/地图烘焙` + `-executeMethod`）、
> Go 加载器（`mapdata.Load` → `ApplyTo`）、客户端查询（`Game.Map`）**全部在引擎里**。
> 业务侧只剩三件事：**摆场景、给烘焙参数、摆出生点标记**。

- Editor 铺场景（量测优先，见 `3d-mmo-core-code/06-editor-measure-place.cs`）→ 用 **`Clover/地图烘焙`** 烘焙：
  一次写两份**同源字节**到 `Assets/MapData/`（服务端读）与 `Assets/Resources/MapData/`（客户端读）；
- 服务端：`m, err := mapdata.Load(path)` → `m.ApplyTo(scene)`，日志会打 `mapdata: 地图已构建 scene=… name=… %dx%d cell=… 可走=… 阻挡=… 碰撞体=… 出生点=… v=…`（grep `mapdata: 地图已构建` 即可命中）；
- 客户端：`Game.Map.LoadFromResource("MapData/<名字>")` 后打印同一组数字，**两端数字必须一致**；
- 验收：`GridProbe`（服务端探针）与客户端 `Game.Map.WalkableAt()` 对同一批点结论一致（道路=true / 建筑=false）。

**业务侧不该出现的代码**（出现即说明在重造轮子）：自己定义地图文件格式、自己解位图、自己算格换算、自己写 Unity 侧烘焙器。
唯一该业务写的是**预测手感**（步 3 的 ①）：半径采样 + 分轴滑墙 + 扫掠细分。

## 步 3：移动三件套（**一次全上**，缺一必翻车）

| 件 | 做什么 | 代码落点 |
|---|---|---|
| ① 本地碰撞 | 同源地图数据 + 半径 8 向采样 + 分轴滑墙 + **扫掠细分 ≤0.25m** | `3d-mmo-core-code/01-map-collision.cs` |
| ② 预测/校正/回放 | 本地立即动 + 20Hz 上行带 `seq`；服务端拒绝回 `MoveCorrect(ack)`；客户端「置权威值 → 丢弃 ≤ack → **回放剩余**」 | `3d-mmo-core-code/02-player-motor.cs` + `3d-mmo-core-code/05-server-move-correction.go` |
| ③ 停发闸门 | 未进图/断线 `SendEnabled=false`；接 `Net.OnDisconnected/OnResumed/OnResumeFailed/OnUnauthorized/OnKicked` | `3d-mmo-core-code/04-net-reconnect.cs` |

- 验收（运行时自证）：
  - 小步前进 100 次 → **停在障碍前**（被挡次数 > 0）；
  - 一次 7.5m 位移 → **结果远小于目标**（细分防穿墙生效）；
  - 故意发非法移动 → 客户端日志出现「应用服务端校正 ack=… 丢弃输入=N 回放=M」；
  - 服务端日志出现「拒绝… + 下发校正」。

## 步 4：第三人称相机（遮挡避障是必做项）

- 六条：多高度探针 / **允许拉近到 0.2m** / `CheckSphere` 兜底 / 收缩快恢复慢 / 贴脸隐藏角色 / 不钻地；
- 输入：右键拖拽转视角 + 滚轮缩放 + pitch 限制；不要用 `Game.Camera.Follow`（只是简单跟随）；
- 代码落点：`3d-mmo-core-code/03-third-person-camera.cs`；
- 验收：把角色瞬移到障碍旁、相机朝向墙侧 → **相机-角色距离明显变小**（实测 5.5 → 1.47）；转开 → 回到 ~5.5。

## 步 5：动画（必须接线）

- Editor 侧生成 `AnimatorController`（`Idle/Run/Walk/Attack/Hit/Death` + `Speed/Attack/Hit/Death` 参数）→ `Assets/Resources/Anim/{角色}.controller`；
- 运行时 `Game.Anim.CreateAnimator(modelGo, controller)`，用 `SetFloat("Speed")` 驱动移动、`SetTrigger` 驱动攻击/受击/死亡；
- **其它实体也要驱动**：按位移速度算 `Speed`（假人/其他玩家才有跑动动画）；
- 代码落点：`Assets/Editor/AnimSetup.cs（Editor 生成 AnimatorController）+ 运行时在 View 模块` + 运行时在 View 模块；
- 验收：运行时自证打印 `clip=Idle clipTime=… ctrl={角色}`，且移动时 `Speed>0`。

## 步 6：HUD / UI（真面板）

- 用 Editor 脚本生成真预制体 `Assets/Resources/UI/{类名}.prefab` → `Game.UI.Open<T>()`；
- HUD 必含：血条、自身/目标信息、操作提示、功能按钮；控件取不到**必须打日志**；
- 代码落点：`UI/HudPanel.cs`（本项目已实现，可直接改）；
- 验收：`hud=True` + 截图里能看见面板。

## 步 7：战斗闭环（首场景必做）

```
攻击键 → 客户端发 Attack(target) → 服务端校验（冷却/距离/存活）
      → 扣血 → 广播 CombatNotify{attacker,target,kind,hp,maxHp,damage}
      → 客户端：命中音效 + 受击动画 + 血条变化；死亡 → 死亡动画 + 提示 + 3 秒后复活
```
- 服务端：`logic/combat.go`（HP map + 冷却 + 死亡处理 + 复活定时）；
- 客户端：`Module/Combat`（发意图 + 收广播 + 驱动动画/音效/HUD）；
- 验收：**血条真的会掉**、被打时真的有受击动画与音效、死亡后能复活继续玩。

## 步 8：音效（首场景必做）

- 有 CC0 音效素材就接；没有就**程序生成 WAV 占位**（脚步骤响、挥砍、命中、UI 点击）并登记进《资源欠缺清单》；
- 走 `Game.Sound.PlaySFX(name)` / `PlaySFXAt(name, pos)`；脚步按速度节流；
- 验收：能听见（日志 + 人工确认），且音量分组可调。

## 步 9：自证 + 取证 + 交付（**只汇报这一次**）

- 编译干净 → 进 Play → 日志零异常 → 运行时自证 → 截图 → 既有回归测试通过；
- 交付说明包含「参考游戏对照表」（逐项打勾）+「资源欠缺清单」+「本轮也改了引擎 N 处（E 编号）」；
- 中间**不要**逐项汇报（见 `SKILL.md` 铁律 9）。
