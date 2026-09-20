# 范式：Mount 挂载（四角色统一入口）

> API 取自 [`clover-server-engine/pkg/app/app.go`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/app/app.go)。模块路径用 `{module}` 占位，
> 生成时替换为实际 module 名（如 `clover-cq`）。

业务通过 `app.Mount(role, fn)` 挂载，在 `init()` 中注册。**四个角色只有一个入口**，
`role` 决定 `fn` 的参数类型。

```go
package logic

import (
	"{module}/game/def"
	"github.com/qw576483/clover-server-engine/pkg/app"
	"github.com/qw576483/clover-server-engine/pkg/transport/event" // ★ 是 transport/event，不是 event
)

// 全局业务状态（持有 *app.Game 引用，供各 handler 用 l.g.Reply / l.g.Alert）
type gameLogic struct {
	g *app.Game
}

var G = &gameLogic{}

func init() {
	app.Mount(app.RoleGame, func(g *app.Game) {
		G.g = g
		g.OnMsg(def.MsgXxx, G.onXxx) // handler 签名 func(c event.Ctx) error
		// 事件：g.OnEvent(typ string, h event.Handler)
	})
}
```

`main.go` 通过 blank import 触发本包 `init()`：

```go
import _ "{module}/game/logic"
```

## 角色 ↔ 宿主类型

| role | `fn` 必须是 | 谁能发消息进来 |
|---|---|---|
| `app.RoleGame` | `func(*app.Game)` | **客户端**（经网关） |
| `app.RoleMaster` | `func(*app.MasterGame)` | game 转发（`g.Call(app.RoleMaster, ...)`） |
| `app.RoleLog` | `func(*app.LogGame)` | game 转发（`g.Call(app.RoleLog, ...)`） |
| `app.RoleAuth` | `func(*app.AuthGame)` | game 转发 + 账号服自身 HTTP `/auth/*` |

**客户端只直连网关**（以及账号服的 HTTP 登录接口）。master / log 的业务消息一律由 game 转发，
两边消息号配对：

```go
// 目标角色（挂在 master / log / auth 服上）
app.Mount(app.RoleLog, func(l *app.LogGame) {
	l.OnMsg(3000201, onXxx) // handler 签名同 Game：func(c event.Ctx) error
})

// game 侧转发（C2S 先到 game，再转给目标角色）
var resp def.XxxReply
if err := g.Call(app.RoleLog, 3000201, &req, &resp); err != nil {
	g.Alert(c, &proto.EAlertNotify{Title: "log", Content: err.Error()})
	return nil
}
g.Reply(c, resp)
```

**写错角色/签名不会静默**：`Mount` 在 `init` 期校验 `fn` 类型，不匹配直接 panic 
（进程启动就暴露，不会出现「挂错角色、消息永远到不了」）。

## 要点

- `app.Mount` 是业务**唯一**挂载点；`app.Run(configPath)` 启动对应进程时按注册顺序统一执行。
- 四个宿主的 `OnMsg` 签名**一致**：`func(c event.Ctx) error`
  （非 game 角色是「headless Core + 自己的传输层」，与 game 共用同一套派发内核）。
- `Game.OnMsg(msgID uint32, h event.Handler, priority ...int)` 可绑定多个 handler（按优先级降序）；
  非 game 宿主 `MasterGame/LogGame/AuthGame.OnMsg(msgID uint32, h event.Handler)` **没有 priority 参数**（`pkg/app/app.go:448/629/654/688`）。
- 四个角色的 `OnMsg` 传 `msgID <= proto.InternalMsgMax` 都会 **panic**（引擎保留段）。
- 需要宿主引用时，用包级 `G.g`（或 logic 结构体的 `l.g`），**不要在 handler 里现取**。
- 跨服事件在 **Game** 上，不在 Ctx 上：
  ```go
  l.g.SendEventToPlayer(c, playerID, "PlayerCreated", payload) // ★ g 上，要传 c
  ```
- 定时器、跨服事件模板见 `patterns/timer.md` / `patterns/crossnode.md`。

## 相关

- Handler 写法与 API 铁律：`patterns/handler.md`
- 新工程从零搭建：`scaffold/new-project.md`
