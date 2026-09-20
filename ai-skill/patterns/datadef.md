# 范式：数据 Schema 定义

位置：`game/datadef/`。

## 1. 单对象 StructSchema（自动落库 + 自动同步）

```go
package datadef

import "github.com/qw576483/clover-server-engine/pkg/domain/data"

var PlayerProfile = data.StructSchema{
    Type:       "player_profile",
    OwnerType:  data.OwnerPlayer, // 按玩家维度存储
    Visibility: data.ClientSelfOnly,
}

func init() {
    data.RegisterTypeBySchema(PlayerProfile)
}

// 业务结构体（引擎数据结构用 E 前缀，真身在 pkg/domain/data/*，internal 侧只做别名）
type PlayerProfileData struct {
    Name   string `json:"name"`
    Level  int    `json:"level"`
    Gold   int64  `json:"gold"`
}
```

## 2. 强类型多行表 RecordSchema（背包/邮件/任务）

```go
import (
    "github.com/qw576483/clover-server-engine/pkg/domain/data"
    "github.com/qw576483/clover-server-engine/pkg/domain/object"
)

var Bag = data.RecordSchema{
    Type:       "bag",
    OwnerType:  data.OwnerPlayer,
    Cols:       []string{"item_id", "count"},   // 列名
    ColTypes:   []object.Type{object.TypeString, object.TypeInt}, // 列类型
    Visibility: data.ClientSelfOnly,
}

func init() {
    data.RegisterTypeBySchema(Bag)
}
```

## 3. 按服务器维度广播（OwnerServer）

```go
var ServerAnnounce = data.StructSchema{
    Type:       "server_announce",
    OwnerType:  data.OwnerServer, // 改动自动广播全服，并镜像到所有 Game 节点
    Visibility: data.ClientVisible,
}
```

## 4. 读取 / 修改（Load-Modify-Return）

```go
func (l *gameLogic) onMsgXxx(c event.Ctx) error {
    var v datadef.PlayerProfileData
    // 首次访问自动注册 schema/visibility
    _ = l.g.LoadStruct(c, datadef.PlayerProfile, c.PlayerID(), &v)
    v.Gold += 100 // 直接改字段
    // handler 返回时引擎自动 Commit（dirty → 写库 → 字段级增量广播）
    return nil
}
```

## 要点

- 三元键 `Key{Owner, ID, Type}`：`Owner` ∈ `OwnerAccount/OwnerPlayer/OwnerServer/OwnerObject/OwnerMeta`。
- 可见性 `data.Visibility`：`ClientVisible` / `ClientSelfOnly` / `ServerOnly`。
- SQL 表名不带 `e_` 前缀：`player` / `orders`（非 `e_orders`，`order` 是保留字已改名 `orders`）。
- `Load*` 首次调用自动注册 schema；`RegisterTypeBySchema` 在 `init()` 显式注册亦可。
- 引擎数据结构用 `E` 前缀（`EAccount`/`EPlayer`/`EOrder`/`EChannel`）。
