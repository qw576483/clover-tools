# 范式：跨服事件投递

> API 取自 `clover-server-engine/pkg/app/app.go`。**事件发送在 `Game` 上，不在 `Ctx` 上。**
> 模块路径用 `{module}` 占位，生成时替换为实际 module 名。

跨服事件底层走 `CrossNodeEventBus`：本地短路 + 远程经 Master + NATS。

## 1. 投递事件

```go
package logic

import (
	"{module}/game/def"
	"clover-server-engine/pkg/app"
	"clover-server-engine/pkg/transport/event"
)

type gameLogic struct{ g *app.Game }

var G = &gameLogic{}

func init() {
	app.Mount(app.RoleGame, func(g *app.Game) {
		G.g = g
		g.OnMsg(def.MsgXxx, G.onMsgXxx)
		g.OnEvent("PlayerCrossNodeSync", G.onPlayerCrossNodeSync) // 订阅跨服事件
	})
}

func (l *gameLogic) onMsgXxx(c event.Ctx) error {
	// 投递给指定玩家（跨服自动寻址，本服玩家走本地短路）
	// ★ 在 g 上，且第一个参数是 c
	if err := l.g.SendEventToPlayer(c, targetPlayerID, "PlayerCrossNodeSync", payload); err != nil {
		return err
	}

	// 广播给全部 game node（含本节点）；无 NATS 时自动降级为本地派发
	if err := l.g.SendEventToAll("PlayerCrossNodeSync", payload); err != nil {
		return err
	}
	return nil
}
```

## 2. 订阅事件

```go
// 跨服事件到达后触发（本地 emit 与跨节点送达走同一入口）
func (l *gameLogic) onPlayerCrossNodeSync(c event.Ctx) error {
	var p MyPayload
	if err := c.BindEvent(&p); err != nil { // ★ 事件载荷用 BindEvent，不是 BindMsg
		return nil
	}
	// 处理…
	return nil
}
```

## 3. API 要点

| 用途 | 正确写法 |
|---|---|
| 投给指定玩家 | `g.SendEventToPlayer(c, playerID, typ string, payload any) error` |
| 广播全部节点 | `g.SendEventToAll(typ string, payload any) error`（**不传 c**） |
| 订阅 | `g.OnEvent(typ string, h event.Handler)` |
| 取事件载荷 | `c.BindEvent(&v)`（`Ctx` 上只有读，没有发送方法） |

> 「空间 / 房间广播」没有专门 API：用 `g.SendEventToAll`，或业务层自行维护成员列表逐个投递。

## 4. 相关

- 查玩家在哪个节点（跨服好友在线状态 / 邀请 / 观战寻址）：`patterns/player-lookup.md`
- Handler 与回包：`patterns/handler.md`
- 挂载：`patterns/mount.md`
