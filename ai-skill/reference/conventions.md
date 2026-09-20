# Clover 业务代码约定（终态）

> 本文件是**团队约定层**的权威来源，与 `clover-doc/` 冲突时以本文件为准。
> 但 `clover-doc/`（77 篇官方手册）仍是 **API 手册**：语义、配置、环境、协议设计以它为准，
> 二者不冲突的部分照常读它。**不要因为本条就去读源码。** 优先级见 `SKILL.md`「信息获取优先级」。

## 1. 消息号三段 + E 前缀

全局消息号空间：引擎占 `[1, 10000]`，业务从 `>= 10001` 起。业务消息号必须 `>= 10001`，由 `app.Game.OnMsg` 启动期统一校验（误用 panic）。`InternalMsgMax = 10000`。

引擎 proto 常量用 `E` 前缀，**真身在 `pkg/shared/proto`**（`internal/shared/proto` 只做再导出，业务直接 import pkg）：

| 类别 | 文件 | 引擎常量 | 示例 |
|---|---|---|---|
| C2S | `pkg/shared/proto/msg.go` | `EMsg*` | `EMsgLogin=2`、`EMsgResumeSession=3`（1 号原为 `EMsgSignup`，已作废保留） |
| 网关直发（S2C，不经逻辑服） | `pkg/shared/proto/msg.go` | `EMsg*` | `EMsgUDPBindGrant=6`（UDP 绑定令牌）、`EMsgQueuePosition=7`（排队位置，开 `queue_cap` 才出现） |
| 回包 | `internal/shared/proto/reply.go` | 结构体（无独立消息号） | `ELoginReply`、`EResumeSessionReply`、`EErrorReply`、`ERankQueryReply` |
| 推送 | `pkg/shared/proto/push.go` | `EPush*` | `EPushPlayerFullSync=4001`、`EPushAlert=4002`、`EPushSceneInfo=4005` |
| 特殊 | — | `EMsgError = 0xFFFFFFFF` | 逻辑服→客户端错误回包通道，不参与顺序 |

业务（demo）消息分三段，各从独立起点起，与引擎段永不重叠：

| 段 | 文件 | 起点 | 示例 |
|---|---|---|---|
| C2S | `game/def/msg.go` | `1000101` | `MsgGetPlayerList=1000101`、`MsgCreatePlayer=1000102`、`MsgEnterGame=1000103` |
| 回包 | `game/def/reply.go` | 无（**回包不占消息号**） | 只定义回复体结构，命名 `<请求名>Reply`（如 `GetPlayerListReply`）；引擎按 requestID 配对，回包帧 msgID 恒为 0 |
| 推送 | `game/def/push.go` | `3002001` | `PushFrameRoomSync=3002001`、`PushDemoBroadcast=3003001` |

业务别名写法（保留 `Msg` 前缀，引擎常量用 `EMsg` 前缀）：

```go
// ① 复用**引擎**常量：直接用 `proto.EMsgXxx`（引擎只占 [1,10000]，业务不得占用）
const MsgRankQuery = proto.EMsgRankQuery

// ② 业务自己的消息号：从 10001 起自行定义（见 patterns/handler.md）
const MsgCreatePlayer = 10001
```

回包建议统一 `Reply` 前缀（如 `MsgCreatePlayerReply`），推送 `Push/Msg` 前缀（如 `MsgSettingsSync`），与 demo 命名一致。

> `InternalMsgMax` / `TargetAll` / `NATSSubjectNotify` 不是消息号，不享受 `E` 前缀。`msg-client` 的 proto 解析器须同时识别 `Msg`/`EMsg` 前缀用 `TrimPrefix` 取 base 关联 Req/Reply。

## 2. 数据结构 E 前缀

引擎数据结构用 `E` 前缀大写导出（`EAccount` / `EPlayer` / `EOrder` / `EChannel`），**真身在 `pkg/domain/data/*`**（`internal/domain/data/*` 只做别名）。SQL 表名**不带** `e_` 前缀：`account` / `player` / `orders` / `account_channel`（`order` 为保留字，已直接改名 `orders` 规避）。

## 3. Ctx 单一职责（终态）

`Ctx` 只保留请求元数据 + 纯数据访问器，**无副作用方法**。

**Ctx 字段（私有）**：`ctx, account, playerID, requestID, msgID, connID, body, payload, line, reply (*replySlot), bag (*dispatchBag), connBag`（见 `internal/transport/event/ctx.go:78-99`）。

**Ctx 公开方法**：
- 纯取数据：`Context()` `TraceID()` `SpanID()` `Account()` `Line()` `PlayerID()` `IsLoggedIn()` `MsgID()` `ConnID()` `ConnValue()` `SetConnValue(key, value any)` `SetConnBag()` `TargetID()`
  - ⚠️ `Session()` **不在** `event.Ctx` 接口上（只有内部 `*Ctx` 有），业务取不到 —— 要拿会话信息用 `Account()` / `PlayerID()` / `ConnID()`
- 请求体：`Body()` `BindMsg(&req)`（反序列化；注意**不叫** `Bind`）
- 事件载荷：`Payload()` `BindEvent(&evt)`（取事件载荷并反序列化）
- 连接：`SetPlayerID()` `ConnID()` `Line()`
- 回包标志：`MarkReplied(body)` `SetNoAutoReply()` `NoAutoReply()`
- 标志位：`SetNoPush()` `NoPush()`

**Ctx 方法补充**：`TargetID()` 是**公开**接口方法（业务可用，见 `pkg/transport/event/eventcore.go`）。
`SetConn()` **不在** `event.Ctx` 接口上（引擎内部用），业务改连接级数据请用 `SetConnValue/SetConnBag`。

**以下能力在 `Game` 上（不在 `Ctx`）**：
- `LoadStruct`/`LoadRecord` → `Game.LoadStruct`/`Game.LoadRecord`
- `Push`/`Alert` → `Game.PushToPlayerJSON(playerID, msgID, v)` / `Game.Alert(c, &proto.EAlertNotify{Title, Content})`（均引擎内置）
- `SendEventToPlayer` → `Game.SendEventToPlayer(c, playerID, typ, payload)`；全节点广播 `Game.SendEventToAll(typ, payload)`（引擎**无** `SendEventToSpace`）
- `Reply`/`ReplyRaw` → `Game.Reply`/`Game.ReplyRaw` 封装编码 + 回包；`Ctx` 仅留 `MarkReplied(body)` 纯数据写入

**engine handler 调用写法**：`g.Reply(c, v)` / `l.g.Reply(c, v)`（回包按 requestID 配对，不携带业务消息号）。
**auth handler 调用写法（无 Game 引用）**：`b, _ := ujson.Marshal(v); c.MarkReplied(b)`。

## 4. 写入模型：Load-Modify-Return

`g.LoadStruct(c, schema, id, &v)` 读出的结构体可直接修改，**无需显式 Save**——handler 返回后引擎自动 Commit（dirty → 写库 → 字段级增量广播，除非 `c.SetNoPush()`）。`OwnerServer` 数据改动还会自动镜像到所有 Game 节点。

## 5. 登录与注册的职责边界

**注册**一律走**账号服**（`POST {账号服}/auth/signup`）；游戏服不接收注册报文（`EMsgSignup=1` 号位已作废保留），客户端发过来会落到「没有 handler」或被登录门禁挡下。**登录**由引擎内置的 `RemoteAuthenticator`（`internal/domain/auth/client`）负责（调账号服 `/auth/verify` 换 owner），链路唯一，`ELoginRequest` 只带 `token`。

## 6. 到期型任务：deadline 是唯一真相源（终态）

凡「到点必须发生」的事（行军到达、建造 / 征兵完成、挂机产出、拍卖截止、邮件过期、CD 结束），
必须把**绝对时间戳**写进数据层；内存定时器只做「到点主动做一次」的**加速器**。

- 调度器是**进程内**的，重启 / 崩溃 / 滚动发布即丢。**不能靠定时器持久化兜底**：`g.Timer` 未注入
  `PersistBackend`（`PersistScope`/`RestoreScope` 返回 `ErrNoBackend`），且 `DumpScope` 存的是**相对剩余**、  
  `ImportScope` **不补触发已过期任务**（`pkg/runtime/timer/timer.go:644-651,732-744`）。
- 重建时机按 owner 维度选，**不做全表扫描**：玩家 = 进游戏 handler（`g.LoadStruct`）；
  服务器 = `app.Mount` 启动 + `Every` 周期兜底（`g.Data().LoadJSON`）；场景对象 = 对象加载时。
- 未到期用 `Group.OnTimer(name, when, …)`（**绝对时刻、已过期立即触发**，`timer.go:561-569`）；结算必须**幂等**。

**scope 命名（与引擎断线清理的关系）**——引擎断线时只清 `StopTimerGroup(owner)` 这一个 scope，
该 `owner` 是**连接级 owner**（登录回执的 `owner` 字段 / `AccountID`，`internal/app/game.go:811-812`），
**不是角色 ID**：

| 诉求 | scope |
|---|---|
| 要随掉线自动清理（buff 结算、在线巡检、临时状态） | scope **必须等于**该 owner |
| 离线也要继续推进（行军 / 建造 / 挂机产出） | scope **故意不用** owner，加前缀即可（如 `"todo:"+playerID`） |

> 因此 `"player:"+PlayerID` 这类 scope **不会**被自动清理；`clover-doc/server/concepts/timer.md` 早期示例的
> 「下线自动清理」说法已按源码更正。落地模板见 `patterns/timer.md` §「到期型任务」。

## 7. 数据归属与分片：高频数据不跨节点共享（终态）

1. **一份数据只有一个 owner 节点**（单写者）：跨节点只发请求 / 事件，**不做共享内存**（跨机只能走网络）。
2. **owner 写 → 广播 → 各节点本地缓存**（`MirrorEvent` 即此模式）；**禁止多节点同写一份内存数据**。
3. **热点按 `util.Xxhash64Key` 分片**（数据层进程内已 64 分片；SLG 大地图按 Chunk 分块）。
4. **Redis 只做慢路径**：跨节点可见（`TierSnapshot`）与重启可恢复（session / rank 备份）；
   热点在本地内存聚合后批量写，**禁止「每次操作都写 Redis」**（同机 RTT ~0.1ms，跨机房 30ms+ 是杀手）。
5. **master 多节点 = 按 uid 哈希分片**：定位表分片、节点表与健康交 etcd lease + watch，**不做主备、不做选主**。

> 行业对照：WoW = realm + 区域内 sharding；EVE = 单分片 + Time Dilation；SLG = Chunk 分片 + 内存权威 +
> 异步落盘。结论：**「归属 + 分片 + 消息」是主流写法**，不是兼容层。

**已落地：master 分片（多节点、非主备）**——定位按 `uid`、排行榜按`榜名`、session 按 `playerID` 
分片（`Xxhash64(key) % total`），注册在 `clover/services/master/<index>`；节点表 / 健康交 etcd 
**节点目录**（`clover/nodes/<nodeID>`，`type` 必须与 `state.Node.Type` 同取值）。
`master_shard.total`（默认 1）或未配 etcd 时，全部回落原「单连接 + master 节点表」路径，行为不变。
配置与验收见 `clover-doc/server/concepts/cluster.md`「Master 分片与节点目录」。客户端 / 网关侧无需改动。

## 8. 品牌与署名（硬约束，**每个游戏都要有**）

> 规则原文在 `SKILL.md::品牌与署名`；本节是落地细则与自检方法。

- **引擎名只有一个：`clover-engine`**。
  ⛔ **禁止编造别名 / 花名 / 中译名**。
  实测教训：AI 凭空把引擎写成「**黑月引擎（Clover）**」放进了游戏首页，用户质问
  「**我们也不叫黑月引擎。我也不知道你首页上的黑月是怎么来的？？？**」。
  ⇒ **任何界面、文档、注释、交付说明里出现非 `clover-engine` 的引擎自称，一律算 bug，立刻铲掉。**
- **所有做出来的游戏，首页页面下方必须有一行署名**：**`by clover-engine`**
  （居底居中、字号小、颜色低调，不抢画面）。它要**写进该游戏的 `策划/验收表.md` 一行并实机截图验证**。

**怎么查（这一条只能人工核对，⛔ 不许把具体名字当 grep 模式）**：

1. 对交付物（界面文案 / 标题 / README / 交付说明 / 注释）grep `引擎|Engine`，**逐条人工核对**每个自称；
2. ⛔ **不许**把某个历史自造名（如上面那个）写成固定 grep 模式 ——
   **自造名是无穷的，个案不是规则**：写死只抓得到历史那一次，下次换个名字照样漏；
   更糟的是会**假阳性**（万一用户点名要做的游戏本身就叫那个名字，会被当成 bug 改掉）；
3. **顺手清理**：把**这次**自造出的那个名字在仓库里全局 grep 一遍，出现的地方一起改掉
   （⛔ 别只改首页那一处）。**每次自造的名字都不一样，按当次实际出现的字符串搜。**
