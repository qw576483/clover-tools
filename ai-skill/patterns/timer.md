# 范式：定时任务

通过 `Game.Timer`（底层 `pkg/runtime/timer`）注册。在 `app.Mount(app.RoleGame, ...)` 内挂载。

```go
package logic

import (
    "time"

    "github.com/qw576483/clover-server-engine/pkg/app"
)

func init() {
    app.Mount(app.RoleGame, func(g *app.Game) {
        // 每 60 秒执行一次（回调签名是 timer.Task = func()，无参数）
        g.Timer.Every("announce_tick", 60*time.Second, func() {
            // ...
        })

        // 启动后 10 秒执行一次
        g.Timer.After("warmup", 10*time.Second, func() {
            // ...
        })

        // Cron 表达式：每天 0 点（5 段：分 时 日 月 周）
        g.Timer.Cron("daily_reset", "0 0 * * *", func() {
            // ...
        })
    })
}
```

## 要点

- `Every(name, dur, task)` / `After(name, dur, task)` / `Cron(name, spec, task)`，`task` 类型为 `timer.Task = func()`（**无参数**，不是 `func(ctx context.Context)`）。
- 第一个参数为任务名，配合 `g.Timer.StopTimer(name)` 可按名取消。
- Cron 为 **5 段**格式（分 时 日 月 周），写 6 段会直接报错。
- 回调内若需访问 Game，闭包捕获 `g` 即可。

---

## 到期型任务：deadline 是唯一真相源（离线 / 跨重启 / 跨宕机）

> **适用**：行军到达、建造 / 征兵完成、挂机产出、拍卖截止、邮件过期、CD 结束……
> **一句话**：把「何时该发生」写进**数据**（绝对时间戳），定时器只做「到点主动做一次」的**加速器**。

### 为什么不能只挂内存定时器

`pkg/runtime/timer` 是**进程内**结构（单 goroutine + 截止时间最小堆）。进程重启 / 崩溃 / 滚动发布 → 堆没了、任务蒸发。
**别指望 `PersistScope` 兜底**：

- `g.Timer` 建调度器时只传时区（`internal/app/mount.go:107-111`）、**未注入 `PersistBackend`** ⇒ 业务调
  `PersistScope` / `RestoreScope` 直接得 `timer.ErrNoBackend`；
- 即便注入：`DumpScope` 存的是**相对剩余时间**（`timer.go:644-651`），`ImportScope` 按剩余量重新入堆、
  **不补触发已过期任务**（`timer.go:732-744`）⇒ 停机 2 小时会把任务**顺延**而不是补算。

### 三条铁律

1. **deadline 落数据**：凡「到点该发生」的事，数据里必须有一个**绝对时间**字段（Unix 秒或 RFC3339）。
2. **定时器可丢**：它只负责「到点时立刻做」；丢了最多晚一点，**不能丢结果**。
3. **结算幂等**：结果只由数据 + deadline 决定，重复执行不产生二次收益。

### 重建时机（按 owner 维度选，**不要全表扫描**）

| owner | 重建时机 | 加载方式 |
|---|---|---|
| 玩家 | **进游戏 handler**（如 `MsgEnterGame`，此处有 `event.Ctx`） | `g.LoadStruct(c, schema, c.PlayerID(), &v)` |
| 服务器 | **进程启动**（`app.Mount` 回调内）+ `Every` 周期兜底 | `g.Data().LoadJSON(ctx, data.Key{...}, &v)` |
| 场景对象 | 对象创建 / 加载时 | 对象自身加载路径 |

玩家维度**天然分布式**：谁上线谁重建自己那部分。

### 核心原语：`Group.OnTimer`（绝对时刻，已过期立即触发）

`Group.OnTimer(name string, when time.Time, task timer.Task)`（`pkg/runtime/timer/timer.go:561-569`）：
`when` 是**绝对时刻**，内部 `delay < 0 → 0` ⇒ **已过期立即执行**。这正是「按 deadline 精确重建」的入口 ——
过期 → 立即补算；未过期 → 到点触发。

### 模板 1：玩家维度（照抄可编译）

```go
// game/datadef/todo.go
package datadef

import "github.com/qw576483/clover-server-engine/pkg/domain/data"

// 待办表：一个玩家一条，Items = 任务名 → 绝对 deadline
var PlayerTodo = data.StructSchema{
	Type:       "player_todo",
	OwnerType:  data.OwnerPlayer,
	Visibility: data.ServerOnly, // 纯服务器数据，不推客户端
}

type TodoItem struct {
	Name     string `json:"name"`
	Deadline int64  `json:"deadline"` // ★ Unix 秒，绝对时刻（不是剩余时长）
	Kind     string `json:"kind"`     // 业务自定义：march / build / recruit ...
	Payload  string `json:"payload"`  // 结算所需的最小上下文（业务 ID 或 JSON）
}

type TodoData struct {
	Items map[string]TodoItem `json:"items"`
}

func init() { data.RegisterTypeBySchema(PlayerTodo) }
```

```go
// game/logic/todo.go
package logic

import (
	"context"
	"errors"
	"time"

	"{module}/game/datadef"
	"github.com/qw576483/clover-server-engine/pkg/domain/data"
	"github.com/qw576483/clover-server-engine/pkg/foundation/logger"
	"github.com/qw576483/clover-server-engine/pkg/transport/event"
)

// scope 用显式前缀 "todo:"，★ 与引擎的 owner **不同名**：
// 断线时引擎只会 StopTimerGroup(owner)，其中 owner 是登录回执 owner / AccountID（**不是角色 ID**）；
// 直接拿 owner 当 scope 会把「离线也要跑」的任务一起清掉，加前缀即与之错开。
func todoScope(playerID string) string { return "todo:" + playerID }

// 在「进游戏」handler 里调用（如 MsgEnterGame 的成功分支）
func (l *gameLogic) rebuildTodo(c event.Ctx, playerID string) {
	var v datadef.TodoData
	if err := l.g.LoadStruct(c, datadef.PlayerTodo, playerID, &v); err != nil {
		logger.Errorf("todo: load player_todo failed player=%s err=%v", playerID, err)
		return
	}
	if len(v.Items) == 0 {
		return
	}
	group := l.g.Timer.TimerGroup(todoScope(playerID))
	nowSec := time.Now().Unix()
	for name, it := range v.Items {
		item := it // 闭包捕获循环变量，先拷贝一份
		if nowSec >= item.Deadline {
			l.settleTodo(playerID, item) // 已过期：立即补算（离线 / 宕机期间到点的部分）
			continue
		}
		group.OnTimer(name, time.Unix(item.Deadline, 0), func() {
			l.settleTodoByID(playerID, name)
		})
	}
}

// 定时器回调入口：没有 event.Ctx，走 g.Data() 直接读写
func (l *gameLogic) settleTodoByID(playerID, name string) {
	ctx := context.Background()
	key := data.Key{Owner: data.OwnerPlayer, ID: playerID, Type: datadef.PlayerTodo.Type}
	var v datadef.TodoData
	if err := l.g.Data().LoadJSON(ctx, key, &v); err != nil {
		logger.Errorf("todo: reload failed player=%s name=%s err=%v", playerID, name, err)
		return
	}
	item, ok := v.Items[name]
	if !ok {
		return // 已被登录补算摘除：幂等正常路径，不是错误
	}
	if time.Now().Unix() < item.Deadline {
		return // 时钟回拨等异常：等下次重建重新挂，不在回调里递归注册
	}
	l.settleTodo(playerID, item)
}

// 结算：先摘除（幂等闸门）再执行副作用。
// 定时器触发 / 启动扫描 / 登录补算 三条路径可能同时命中同一任务，靠这里的「摘除」去重。
// 注意：若副作用本身可能失败，更稳的做法是让副作用按「已结算」状态判定（真幂等），
// 而不是只依赖这里的 delete。
func (l *gameLogic) settleTodo(playerID string, item datadef.TodoItem) {
	ctx := context.Background()
	key := data.Key{Owner: data.OwnerPlayer, ID: playerID, Type: datadef.PlayerTodo.Type}
	var v datadef.TodoData
	if err := l.g.Data().LoadJSON(ctx, key, &v); err != nil && !errors.Is(err, data.ErrNotFound) {
		logger.Errorf("todo: settle load failed player=%s name=%s err=%v", playerID, item.Name, err)
		return
	}
	cur, ok := v.Items[item.Name]
	if !ok {
		return // 已结算过
	}
	delete(v.Items, item.Name)
	if v.Items == nil {
		v.Items = map[string]datadef.TodoItem{}
	}
	if err := l.g.Data().SaveJSON(ctx, key, &v); err != nil {
		logger.Errorf("todo: settle save failed player=%s name=%s err=%v", playerID, item.Name, err)
		return
	}
	switch cur.Kind {
	case "march":
		l.onMarchArrive(playerID, cur)
	default:
		logger.Warnf("todo: unknown kind=%q player=%s name=%s", cur.Kind, playerID, cur.Name)
	}
}
```

### 模板 2：服务器维度（进程启动重建 + 周期兜底）

```go
const worldTodoID = "world" // 业务自定义 ID，不依赖配置里的区服号

func init() {
	app.Mount(app.RoleGame, func(g *app.Game) {
		key := data.Key{Owner: data.OwnerServer, ID: worldTodoID, Type: "world_todo"}
		var v datadef.WorldTodoData
		switch err := g.Data().LoadJSON(context.Background(), key, &v); {
		case err == nil:
			rebuildWorldTodo(g, key, v) // 结构同玩家维度：过期立即结算、未过期 OnTimer(deadline)
		case errors.Is(err, data.ErrNotFound):
			logger.Infof("todo: world_todo 尚未创建，跳过重建")
		default:
			logger.Errorf("todo: world_todo load failed err=%v", err)
		}
		// 兜底：即便启动期加载失败，也靠周期扫描补上（Task 无参，闭包捕获 g）
		g.Timer.Every("world_todo_scan", 30*time.Second, func() { scanWorldTodo(g, key) })
	})
}
```

### 定时器回调里没有 `event.Ctx`

> `g.Timer` / `g.Data()` 这类**非 event.Ctx 方法**由 `pkg/app.Game` 嵌入 internal 实现后**自动提升**，
> 业务可直接调用、无需 import internal（`pkg/app/app.go:362-364`）。

`timer.Task` 是 `func()`，所以：

1. 读写数据走 `g.Data()` 的 `Load / Save / LoadJSON / SaveJSON`（ID 用 `data.Key{Owner, ID, Type}` 显式拼）；
   **回调里用不了 `g.LoadStruct`** —— 它需要 `event.Ctx`；
2. 或者把活再投递回有 Ctx 的语义：`g.SendEventToPlayer(...)` / `g.SendEventToGObject(...)`。

> ⚠️ `g.Data()` 直写**不经过 handler 的 commit 通道** ⇒ 不会自动做字段级增量广播（对比
> `patterns/datadef.md` §4）。需要同步给客户端的，用 `g.PushToPlayer(...)` 显式推。

### 常见坑

- **scope 别用 `owner`**：引擎断线时只清 `StopTimerGroup(owner)` 这一个 scope，而 `owner` 是**连接级 owner**
  （登录回执的 `owner` / `AccountID`，`internal/app/game.go:781-789`、`bootstrap.go:347-365`），**不是角色 ID**。  
  给「离线也要跑」的任务加显式前缀（`"todo:"+playerID`）即与它错开。  
  反过来看：`"player:"+PlayerID` 这类 scope 与 owner **不同名**，**不会**被自动清理 ——  
  要依赖自动清理就得让 scope 等于 owner，否则业务得自己 `StopTimerGroup`   
  （[`clover-doc/server/concepts/timer.md`](https://github.com/qw576483/clover-doc/blob/main/server/concepts/timer.md) §作用域与清理 已按源码更正）。
- **数据里存绝对时刻**，重建时才换算成 `when`；不要在 `OnTimer` 里用 `time.Now().Add(剩余)` 糊 deadline。
- **`OnTimer` 名字要稳定**：只有具名任务才能被 `StopTimer` / 迁移导出；匿名任务不可重建。
- **结算幂等**：定时器 + 启动扫描 + 登录补算会同时命中同一任务。
- **日志**：所有失败分支都要打（`logger.*`）；周期扫描 / 高频回调必须防刷屏（首次打全量或降频，见 SKILL.md「错误处理与日志」）。
- **规模**：上万行的世界待办不要塞进单个 struct —— 目前**没有**「按 owner 穷举记录」的业务可用入口
  （`Store.LoadAll` 要给定 `id`、`LoadRecord` 要 `event.Ctx`），需要业务自建索引  
  （如 `Key{OwnerServer, "world", "march_index"}` 存 id 列表）或按玩家 / 地图分块存放。
  该限制属**已定设计**（原登记 `服务器待做.md` §三 C1(b) 已判定为"已有入口即设计选择"并随该节删除）；需要通用入口时另开条目登记。
