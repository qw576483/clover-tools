# 范式：C2S + 回包 Handler

> **本文件所有 API 均逐条取自 `clover-server-engine` 源码**（`pkg/app/app.go`、
> `pkg/transport/event/eventcore.go`、`internal/app/core.go`），照抄即可编译。
> **旧稿、文档、以及"我记得是这样"若与本文件冲突，一律以本文件为准。**
> 模块路径统一用 `{module}` 占位，生成时替换为实际 module 名（如 `clover-cq`）。

## 1. 定义消息号

```go
// game/def/msg.go
package def

// 业务消息号必须 >= 10001（引擎占 [1,10000]，启动期统一校验，误用 panic）
const (
	MsgXxx = 10001 // C2S
	// ⚠️ **回包不占消息号**：回包帧的 msgID 恒为 0 由引擎填充（客户端按 requestID 配对），
	//    所以这里**不要**给 Reply 定义消息号常量，只定义回包**结构体**即可。
	//    详见 reference/conventions.md 与 scaffold/new-project.md。
)

// 请求体
type XxxRequest struct {
	Name string `json:"name"`
}

// 回包体
type XxxReply struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}
```

## 2. 编写 Handler 并挂载

```go
package logic

import (
	"{module}/game/def"
	"github.com/qw576483/clover-server-engine/pkg/app"
	"github.com/qw576483/clover-server-engine/pkg/shared/proto"
	"github.com/qw576483/clover-server-engine/pkg/transport/event"
)

type gameLogic struct{ g *app.Game }

var G = &gameLogic{}

func init() {
	app.Mount(app.RoleGame, func(g *app.Game) {
		G.g = g
		g.OnMsg(def.MsgXxx, G.onXxx)
	})
}

// ★ 签名是 event.Ctx（值接口），不是 *event.Ctx——写错会编译失败
func (l *gameLogic) onXxx(c event.Ctx) error {
	var req def.XxxRequest
	if err := c.BindMsg(&req); err != nil { // ★ BindMsg，不是 Bind
		l.g.Alert(c, &proto.EAlertNotify{Title: "错误", Content: "参数错误"})
		return nil // 已 Alert，返回 nil 即可
	}

	// 读数据：Load-Modify-Return，改完返回引擎自动 Commit，无需显式 Save
	// var v MyData
	// if err := l.g.LoadStruct(c, datadef.MySchema, c.PlayerID(), &v); err != nil { ... }
	// v.Field = ...

	l.g.Reply(c, def.XxxReply{OK: true}) // ★ 回包用 g.Reply
	return nil
}
```

## 3. API 铁律（照抄不会错）

### 3.1 Handler 与 Ctx

| 项 | 写法 |
|---|---|
| 签名 | `func(c event.Ctx) error` |
| import | [`clover-server-engine/pkg/transport/event`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/transport/event.md) |
| 取请求体 | `c.BindMsg(&req)` |
| 回包 | `g.Reply(c, v)` |
| 原始字节回包 | `c.MarkReplied(body []byte)` |

`Ctx` **只有**纯数据访问器，取用即可：`c.Account()` / `c.PlayerID()` / `c.RequestID()` /
`c.Line()` / `c.ConnID()` / `c.BindMsg(&req)` / `c.Payload()` / `c.TargetID()` /
`c.SetPlayerID(pid)` / `c.SetConnValue(k,v)` / `c.ConnValue(k)`。

### 3.2 回包 / 弹窗 / 推送 / 事件（全部在 `Game` 上，`Ctx` 上没有）

```go
// 回包（同一 Ctx 仅首次生效）
l.g.Reply(c, def.XxxReply{OK: true})

// 弹窗（自适应路由：PlayerID 优先，未登录回退 Account）
l.g.Alert(c, &proto.EAlertNotify{Title: "提示", Content: "欢迎回来"})

// 推送给指定玩家 —— ★ 不传 Ctx，body 传结构体（内部 JSON 序列化）
l.g.PushToPlayerJSON(playerID, def.MsgXxxPush, &def.XxxPush{...})

// 推送原始字节（自定义编码时用）—— ★ 是 PushToPlayerRaw，不是 PushToPlayer
l.g.PushToPlayerRaw(playerID, def.MsgXxxPush, body)

// 广播：场景内 / 全服（对象版，同样内部 JSON 序列化）
l.g.PushToScene(scene, def.MsgXxxPush, &def.XxxPush{...})
l.g.PushToAll(def.MsgXxxPush, &def.XxxPush{...})

// 领域事件（跨节点自动寻址）—— ★ 需要传 Ctx
err := l.g.SendEventToPlayer(c, playerID, "PlayerCreated", payload)

// 数据读写（Load-Modify-Return）
err := l.g.LoadStruct(c, datadef.MySchema, c.PlayerID(), &v)
```

> 签名速查（以 `clover-server-engine` 源码为准）：
> - 对象版：`pkg/app/app.go` 的 `Game` 上 —— `PushToPlayer(playerID, msgID uint32, v any, opts ...proto.DeliveryMode)`   
>   、`PushToScene(...)`、`PushToAll(...)`；**第三个参数是结构体/任意对象，内部做 JSON 序列化**。
> - 原始字节版：`PushToPlayerRaw(...) / PushToSceneRaw(...) / PushToAllRaw(...)`，body 为 `[]byte`。
> - `PushToPlayerJSON` / `PushToSceneJSON` / `PushToAllJSON` 定义在 `internal/app/core.go` 的 `Core` 上，  
>   由 `Game` 嵌入提升，因此也能直接 `g.PushToPlayerJSON(...)`（与上面的 `PushToPlayer` 等价）。
>⚠️ **别把 `PushToPlayer` 当原始字节用**：`PushToPlayer(p, id, []byte{...})` 会把 `[]byte` 
> 当对象做 JSON 编码（变成 base64 字符串），而不是发原始字节。
>`SendEventToPlayer` 定义在 `pkg/app/app.go` 的 `Game` 上，**要传 `c`**——两者别混。

### 3.3 错误与抑制回包

- handler 返回 **非 nil** → 框架自动回错误包；
- 返回 **nil 且未回包** → 不回包（fire-and-forget），适合纯推送场景；
- 抑制自动回包：`c.SetNoAutoReply()`；查询是否抑制：`c.NoAutoReply()`。

## 4. 相关

- 挂载与全局状态：`patterns/mount.md`
- 数据 schema：`patterns/datadef.md`
- 新工程从零搭建：`scaffold/new-project.md`（含 main.go / server.yaml 模板）
