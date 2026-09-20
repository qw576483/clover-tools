# 范式：玩家定位查询（跨服在线状态 / 寻址）

> API 取自 [`clover-server-engine/pkg/domain/master`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/domain/master/README.md) 的 `PlayerLookup`。
> 句柄由 `master.NewPlayerLookup(g)` 创建（`g` 是 `app.Game`），业务显式持有并调用 `Locate`。
> 用途：跨服好友「是否在线」、跨服邀请、观战寻址 —— 查某玩家当前在哪个 game 节点。

## 1. 基本用法

```go
package logic

import (
	"github.com/qw576483/clover-server-engine/pkg/app"
	"github.com/qw576483/clover-server-engine/pkg/domain/master"
	"github.com/qw576483/clover-server-engine/pkg/foundation/logger"
	"github.com/qw576483/clover-server-engine/pkg/transport/event"
)

type gameLogic struct {
	g  *app.Game
	lk master.PlayerLookup
}

var G = &gameLogic{}

func init() {
	app.Mount(app.RoleGame, func(g *app.Game) {
		G.g = g
		G.lk = master.NewPlayerLookup(g) // 显式创建并持有句柄
	})
}

func (l *gameLogic) onQueryFriend(c event.Ctx) error {
	var req def.FriendOnlineRequest
	if err := c.BindMsg(&req); err != nil {
		return err
	}

	nodeID, online, err := l.lk.Locate(c.Context(), req.PlayerID)
	if err != nil {
		// ★ 通道不可用（master 未就绪 / 已关闭）—— 这不等于「玩家离线」，必须打日志并区别处理
		logger.Errorf("locate player %s failed: %v", req.PlayerID, err)
		return err
	}

	// online=true 时 nodeID 是玩家所在 game 节点；online=false 表示玩家不在线（nodeID 为空串）
	l.g.Reply(c, def.FriendOnlineReply{PlayerID: req.PlayerID, Online: online, NodeID: nodeID})
	return nil
}
```

## 2. 语义（务必区分「离线」与「查询失败」）

| 返回 | 含义 | 处理 |
|---|---|---|
| `online=true` | 玩家在线，`nodeID` 为其所在 game 节点 | 正常业务（含跨节点寻址） |
| `online=false, err=nil` | 玩家不在线（未在 master 登记） | 按离线处理 |
| `err!=nil` | **查询通道不可用**（哨兵 `master.ErrPlayerLookupUnavailable`） | **不许当离线**：打日志、返回错误或稍后重试 |

> 定位表由引擎在玩家上/下线时自动登记与摘除（`CrossNodeEventBus`），**业务只查询、不写入**；
> 不要自己去注册 `uid → nodeID`。
> master 多分片部署时，查询会按 `uid` 自动路由到属主分片，业务无需关心。

## 3. 相关

- 跨服事件投递（拿到玩家所在节点后往目标节点送事件）：`patterns/crossnode.md`
- 排行榜（同属 `pkg/domain/master` 门面）：`master.NewMasterRank(g)`，见 [`clover-server-engine/pkg/domain/master/README.md`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/domain/master/README.md)
- 权威文档：[`clover-doc/server/development/handler.md`](https://github.com/qw576483/clover-doc/blob/main/server/development/handler.md)、[`clover-doc/server/concepts/cluster.md`](https://github.com/qw576483/clover-doc/blob/main/server/concepts/cluster.md)
