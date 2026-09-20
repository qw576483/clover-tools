# 模块速查表（提炼自 clover-server-engine-index.md）

业务只依赖 [`clover-server-engine/pkg/*`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/*.md) 转发层（类型别名 + 工厂），不引 `internal/*`。

| 我想做某事 | 包 | 关键入口 |
|---|---|---|
| 加载一条数据（自动落库+自动同步） | `pkg/app` | `g.LoadStruct(c, schema, id, &v)` / `g.LoadRecord(c, schema, id)` |
| 声明数据可见性 | `pkg/domain/data` | `data.Visibility` + `RecordSchema.Visibility` / `StructSchema.Visibility` |
| 存任意业务数据 | `pkg/domain/data` | `Store.SaveJSON/LoadJSON` + `Key{Owner, ID, Type}`（`Owner` 如 `data.OwnerPlayer`） |
| 强类型多行表（背包/邮件/任务） | `pkg/domain/data` + `pkg/app` | 用 `data.RecordSchema` 声明后走 `g.LoadRecord(c, schema, id)`（`pkg/domain/data` **不导出** `NewRecord`） |
| 监听客户端消息号 | `pkg/transport/event` | `g.OnMsg(msgID uint32, h event.Handler)` |
| 订阅领域事件 | `pkg/transport/event` | `g.OnEvent(typ string, h event.Handler)` |
| 推送给玩家 | `pkg/app` | 对象版 `g.PushToPlayer(playerID, msgID, v)`；原始字节 `g.PushToPlayerRaw(playerID, msgID, body)`；`g.PushToPlayerJSON(...)` 由 `internal/app/core.go` 的 `Core` 提升，等价 |
| 弹窗通知 | `pkg/app` + `pkg/shared/proto` | `g.Alert(c, &proto.EAlertNotify{Title, Content})` |
| 跨服投递事件 | `pkg/app` | `g.SendEventToPlayer(c, playerID, typ, payload)` / `g.SendEventToAll(typ, payload)` |
| MMO 大世界 / AOI 视野 | `pkg/domain/mmo` | `mmo.NewSceneManager(opts...)` / `mmo.NewGrid(cellSize)`（AOI 网格；`pkg/domain/mmo/aoi` 只导出接口，**没有 `New`**，构造入口统一走 `mmo` 包） |
| 房间（帧同步 / 状态同步） | `pkg/domain/room` | `room.NewModule(room.Config{...})` → 外壳 `EnsureRoom/JoinRoom/LeaveRoom/DestroyRoom`（+ owner 路由 / 跨服 takeover）+ 可插拔内核：`FrameCfg` 挂引擎内置帧同步内核，`Kernel` 挂业务自写内核（实现 `room.Kernel` 8 个方法） |
| 网关连接管理 | `internal/transport/gateway` | `gwcore.Gateway` / `conn.Conn`（非 pkg 导出） |
| 账号 / 角色 / 订单 | `pkg/domain/data/account|player|order` | 对应 `Store`（**订单归账号服**：`Game.OrderStore()` + `AuthGame.OrderStore()`，见 `patterns/auth-server.md` 模板 4） |
| 进程启动编排 | `pkg/app` | `app.Run(configPath, bootstrap...)` |
| 注册业务 handler / 挂载点 | `pkg/app` | `app.Mount(app.RoleGame, func(*Game))` |
| 定时器 / Cron | `pkg/runtime/timer` | `Game.Timer.Every/After/Cron`（回调 `timer.Task = func()`） |
| 消息队列（NATS） | `pkg/transport/event`（NATS 桥接在 internal） | 业务一般不直接调用 NATS |
| 服务发现（etcd） | `internal/app/discovery.go`（统一设施：logic / auth / log 共用「前缀 + 每实例唯一 key + 租约续租」注册与轮询解析；非 pkg 导出。`log` 前缀同时服务同步 RPC `CallLog` 与业务日志管道 `AddLog`） | 业务一般不直接调用 |
| 日志（程序运行日志） | `pkg/foundation/logger` | `logger.Init` |
| 业务日志落盘后端 | `pkg/foundation/logstore`（契约真身；引擎内置 `mysql` 为默认） | `app.RegisterLogBackend(name, factory)` + 配置 `log_backend` / `log_backend_config` —— **换后端只改配置，不改引擎代码** |
| 跨机对象迁移（MMO 切场景） | `pkg/domain/mmo`（`WithClusterRoute` / `WithRemoteTransferSubscriber`） | 配 `node_id` + etcd 后，在 `NewSceneManager` 注入 `g.SceneRoute()` / `g.NodeID()` / `g.SceneSubscriber()`；**不配即单机**（`TransferRemote` 返回 `ErrNoRoute`） |
| 通用工具（ID/缓存/几何/排行榜） | `pkg/shared/*` | `id` / `cache` / `geom` / `memrank` |
| 指标埋点（Counter/Gauge/Histogram） | `pkg/foundation/metrics` | `metrics.CounterOf/GaugeOf/HistogramOf`、`metrics.ForModule(metrics.ModuleXxx).Count/Timer`；**命名与 label 基数规则见同包 `naming.go`**（禁止 player_id / conn_id 等高基数 label）。运行时与进程指标（`go_*` / `process_*`，含 `process_cpu_seconds_total`、`process_cpu_ratio`）由 admin `/metrics` 抓取时**自动采集**，业务无需埋点；需要自己算进程 CPU 用 `metrics.ProcessCPUSeconds()` |
| 周期巡检 / 告警（看门狗） | `pkg/runtime/watchdog` | `watchdog.Default().RegisterFunc(name, check)`（check 返回 `watchdog.OK` 或 `watchdog.Warn/Critical(...)`；去抖动/单独周期用完整 `watchdog.Rule{Interval,Cooldown,MinConsecutive}`）。引擎在 `runApp` 里按进程统一拉起（所有 server_type），内置规则 `process_cpu_saturated`，运维快照 `GET /watchdog`。**接真实告警通道**：`watchdog.Install(watchdog.New(watchdog.Options{Sink: watchdog.SinkFunc(...)}))`，且**必须先 Install 再注册规则**；引擎默认不落库（历史由 TSDB 负责），投递满即丢并计 `clover_watchdog_alerts_dropped_total` |

## 包位置对照（容易找错的几处）

| 想找 | 实际位置 |
|---|---|
| redis 存储 | `internal/domain/data/store/redis` |
| 限流 | `pkg/runtime/ratelimit`（TokenBucket / FixedWindow / SlidingWindow / GCRA） |
| 异步 / 协程池 | `pkg/runtime/async`（Pool / Backpressure / Priority） |
| 进程 CPU 采集 / 看门狗装配 | 采集在 `pkg/foundation/metrics/proc*.go`（按平台分文件，不支持则**不导出**该指标）；看门狗装配在 `internal/app/watchdog.go`，进程级拉起在 `internal/app/app.go` 的 `runApp` |
| master | 独立子系统：`internal/domain/master/{client,server,state,failover,metrics}` + `pkg/domain/master`；`server_type=master` 经 **`internal/app/master_server.go` 的 `runMaster`** 启动，RPC 走 TCP。**业务侧经 `pkg/domain/master` 用**：`MasterRank` 排行榜（`NewMasterRank(g)`）、`PlayerLookup` 玩家定位查询（`NewPlayerLookup(g).Locate`，跨服好友/邀请/观战寻址，见 `patterns/player-lookup.md`） |
| auth（账号服） | 独立域：`internal/domain/auth/{state,server,client}` + 域配置 `config.go`；`server_type=auth` 经 `internal/app/auth_server.go` 的 `runAuth` 启动。**登录校验走 HTTP** `/auth/verify`；`auth.rpc_listen` 那条消息通道只跑业务消息 |
| push | `internal/transport/net/push`：`PushChannel` / `PlayerChannel` / `SceneChannel` / `AllChannel` + `Push` / `Alert`（**无** Group 连接组能力） |
| 数据 / MMO 领域包 | `pkg/domain/data` / `pkg/domain/mmo`（不是 `pkg/data` / `pkg/mmo`） |
| 回包 / 推送 / 存储等副作用方法 | 在 `Game` 上；`Ctx` 仅 `MarkReplied(body)` 纯写入 |
