# 账号服（auth 服）接入模板

**什么时候需要**：所有联网服务都需要——引擎**只有一种登录模式**，账号服是登录链路的
**必经依赖**（游戏服每次登录都调它的 `/auth/verify` 换 `owner`）。
部署上可二选一：单进程全功能用 `server_type: all`（内嵌账号服），或 `server_type: auth` 
独立起一个账号服进程。

原理与取舍见 [`clover-doc/server/security/auth-server.md`](https://github.com/qw576483/clover-doc/blob/main/server/security/auth-server.md)；本模板只给**可复制的操作**。

---

## 模板 0：登录链路速查

```text
① 客户端  ──HTTP──▶  账号服    /auth/login  {account, password} ──▶ JWT
② 客户端  ──长连接─▶  游戏网关  EMsgLogin{token}
③ 游戏服  ──HTTP──▶  账号服    /auth/verify {token} ──▶ owner（网关据此绑定会话）
```

登录模式只有这一种，**没有可切换的模式配置**；游戏服不碰账号表、不存密码、不持有密钥。

---

## 代码位置（改引擎或排障时看）

账号域在 `internal/domain/auth`，与 `domain/master` / `domain/log` 同构：

| 层 | 位置 | 职责 |
|---|---|---|
| 业务核心 | `domain/auth/state` | 注册 / 登录 / 验签 / 渠道登录 + 撞库防护（`guard.go`）+ 线协议契约（`wire.go`）+ 领域错误分类（`errors.go`） |
| 传输 | `domain/auth/server` | HTTP 路由注册 + 「领域错误 → 状态码」映射（`400/401/409/429/501/500`） |
| 调用侧 | `domain/auth/client` | `RemoteAuthenticator`：game 调 `/auth/verify` 换 owner |
| 域配置 / 扩展点 | `domain/auth/config.go`、`domain/auth/channel.go` | `AuthConfig`；`ChannelVerifier` 接口与注册表 |
| 角色宿主 | `internal/app/auth_server.go` | Core 接线、消息通道、`runAuth` 装配——**不含账号规则** |
| 订单（支付链路） | `domain/data/order`、`pkg/domain/data/order` | 订单表 + 三段状态机；**归属账号服**，`AuthGame.OrderStore()` 已对业务开放（引擎侧已闭环，见 `服务器待做.md` §一 S2 留痕） |

数据面：账号表 / 渠道绑定表在 `domain/data/account`；JWT 签发与验签在 `pkg/shared/jwt`。
业务侧**只需要** `pkg/app.RegisterChannelVerifier`，其余一层都不用碰。

---

## 模板 1：配置账号服（唯一链路）

**服务端**——按角色取用字段：

```yaml
# 单进程全功能（内嵌账号服）：listen 与 verify_addr 指向同一地址
server_type: all
auth:
  listen: "127.0.0.1:8051"             # 账号服 HTTP 监听
  jwt_secret: "<强随机串>"              # HS256 签名密钥
  token_ttl: 2h
  issuer: "clover-auth"
  verify_addr: "http://127.0.0.1:8051" # 游戏服校验地址（game/gateway/all 必填）
  verify_timeout: 3s

# 独立部署：账号服进程
server_type: auth
auth:
  listen: "127.0.0.1:8051"
  jwt_secret: "<同上一模一样>"

# 独立部署：游戏服进程（不持有 jwt_secret）
server_type: game
auth:
  verify_addr: "http://127.0.0.1:8051"
  verify_timeout: 3s
```

**客户端**——从配置读地址 + 初始化 HTTP（`server.auth_addr` **必填**）：

```csharp
// 地址来自 Assets/Configs/config.json（见 patterns/client/config.md），不要写死在代码里
CloverAuth.AuthAddr = Cfg.Server.auth_addr;
// HTTP 模块随 CloverNet.Init 一并挂接，无需单独初始化
```

```csharp
// 唯一登录路径：HTTP 换 token → 长连接发 token
var token = await CloverAuth.LoginAsync(account, password);
var reply = await Game.Net.Call<ELoginReply>(
    EMsg.Login, new ELoginRequest { token = token });
```

**两条硬约束**：

1. **游戏服必须配 `verify_addr`**——缺失会**启动即 panic**；账号服必须在线，
   否则登录报「账号服务不可用，请稍后重试」。
2. **不要给游戏服发注册报文**——`EMsgSignup=1` 号位已作废保留，游戏服不挂该
   handler、登录门禁也不放行它；注册改走 `CloverAuth.SignupAsync`（账号服 HTTP）。

---

## 模板 2：接第三方登录（微信 / QQ / Steam / 自建账号中心）

> ⚠️ **接两家以上渠道必须用 `app.NewChannelRouter()`**：引擎的注册位是**单值**
> （`internal/domain/auth/state/channel.go:27-40`，只有一个 `v`），`RegisterChannelVerifier` 调两次会
> **静默覆盖**前一家 —— 启动期无任何报错，只在玩家点那家渠道登录时表现为「票据校验失败」。
>
> ```go
> r := app.NewChannelRouter()          // 多渠道路由（真身 internal/domain/auth/router.go）
> r.Register("wechat", wxVerifier)     // 业务自己的 ChannelVerifier 实现
> r.Register("douyin", dyVerifier)
> app.RegisterChannelVerifier(r)       // ← 仍然只注册一次
> ```
> 未注册的渠道，router 返回与「未注册 verifier」**同一 Kind**（`KindChannelUnsupported`）。
> ⚠️ **但客户端收到的不是 501 而是 401** —— 见下面那条「引擎把任何 error 都映射成 401」；
> 501 只在**根本没注册 verifier** 时才会出现（`state.go:286-290`）。
>
> ⚠️ **引擎把 verifier 的任何 error 都映射成 401**（`internal/domain/auth/state/state.go:297-302` 写死 `KindTicketInvalid`）
> ⇒ **「渠道服务故障」与「玩家票据无效」在 HTTP 层不可区分**，只能靠日志（`logger.Warnf` 已带原始 err）区分。

**能，扩展点在账号服，不在游戏服。** 游戏服侧**没有任何登录注入点**——登录链路唯一
（`RemoteAuthenticator` → `/auth/verify`）。这是刻意的：账号体系的扩展能力该长在账号体系里，
否则「账号服必须在线」这条不变量就会出现例外。

**引擎已把整条链路做好，业务只实现一个接口**（`internal/domain/auth/state/channel.go` 定义、
`pkg/app/channel.go` 导出）：

```go
// main.go（须在 app.Run 之前）
app.RegisterChannelVerifier(wechatVerifier{appID: "...", secret: "..."})

type wechatVerifier struct{ appID, secret string }

// Verify 只回答「这张票据对应渠道里的谁」。
func (v wechatVerifier) Verify(ctx context.Context, channel, ticket string) (string, error) {
    return openid, nil   // 调渠道 SDK 换 openid
}
```

之后账号服即支持 `POST /auth/login {"channel":"wechat","ticket":"<code>"}`。

| 环节 | 谁做 |
|---|---|
| 票据 → 渠道账号（openid） | **业务**（`ChannelVerifier`） |
| 渠道账号 → 主账号 | 引擎（`ChannelStore.FindByChannel`） |
| 首次登录建号 + 绑定 | 引擎（注册即登录；账号名 = `{channel}_{channelAccount}_{短哈希}`） |
| 签发 JWT | 引擎 |

引擎已提供的 `ChannelStore` 能力（业务若要做「绑定管理」界面，可经**服务端** `*app.Game` 
的 `ChannelStore()` 取得）：`Bind` / `FindByChannel` / `ListByAccount` / `Unbind`。

> ⚠️ 几个**不要**踩的点：
> - 不要在 `Verify` 里做绑定或建号——那是账号服的职责，重复做会撞唯一键；
> - `Verify` 返回的渠道账号必须**稳定**（同一用户每次一致），否则每次登录都会新建账号；
> - 票据无效要**返回 error**，不要返回空字符串 + nil（引擎会判为校验器实现错误并回 500）；
> - 未注册校验器时接口返回 **501**（不是 401），便于区分「服务端没接渠道」与「票据错了」。

---

## 模板 3：自定义 JWT 验签（RS256 / JWKS / 自定义 claim）

**游戏服侧已不支持替换认证器**（原来的 `pkg/transport/auth` + `app.RegisterAuthenticator` 
注入点已删除）。要换签名算法或改 claim，改**账号服侧**：

- `pkg/shared/jwt` —— 签发与验签的实现
- `internal/domain/auth/state/state.go` —— 签发时机与 `/auth/verify` 的返回契约

好处是：**游戏服不参与验签、也不持有 `jwt_secret`**，所以算法怎么换它都无需感知——
这正是「game 服只认 owner」带来的解耦。

---

## 模板 4：接支付回调（订单归账号服）

**订单归属账号服**（已定）。渠道的异步支付回调是 **HTTP**，落在账号服唯一的公网面
（`AuthGame.OnHTTP`；鉴权按 `/auth/` 前缀放行，故路由挂在 `/auth/pay/...` 下）。

```text
game --CallAuth--> auth 建单(orders 表)，auth 回 orderID 给 game
客户端拿 orderID 拉起渠道支付
渠道 --HTTP--> auth /auth/pay/notify   验签 → MarkPaid → MarkDone
game --CallAuth--> auth 查「这单付了吗」（客户端回报触发；另有定时兜底）
game SendQueueEventToPlayer(c, playerID, "order.paid", payload) → 发货   ← 引擎已有能力
```

| 环节 | 谁做 |
|---|---|
| 渠道回调验签 / 向渠道查单 | **业务**（各渠道协议差异大，引擎不碰） |
| 建单 / 状态流转（`orders` 表） | 引擎（`pkg/domain/data/order` 的 `Store`） |
| 回调 HTTP 入口 | 引擎（`AuthGame.OnHTTP`，业务挂 `/auth/pay/notify`） |
| 发货（加道具 / 货币） | **游戏服**（在线角色数据只在 owner 节点内存） |

```go
// 账号服侧（app.Mount(app.RoleAuth, ...) 内）挂回调路由
ag.OnHTTP("/auth/pay/notify", func(ic event.HTTPCtx) error {
    // ① 业务：验签 + 向渠道查单
    // ② 引擎：订单状态流转 —— ag.OrderStore().MarkPaid(ic.Context(), orderID, channelOrderID) / MarkDone(ic.Context(), orderID)
    // ③ 发货：game 侧查单后 SendQueueEventToPlayer(c, playerID, "order.paid", payload) 触发
    //    —— 引擎已有能力，无需在账号服侧回推
    return nil
})
```

> ⚠️ 引擎侧**已无待做**：账号服能建单 / 流转状态（`AuthGame.OrderStore()`），回推由
> game 侧走既有**按玩家寻址事件通道**（`SendQueueEventToPlayer`）完成——不给账号服加
> NATS / master 依赖（`SendEventToPlayer` 的底座 `CrossNodeEventBus` 需要两者，且按
> game 节点设计）。剩余全是业务：渠道验签、查单应答、发货 handler。
> 见 `服务器待做.md` §一 S2 留痕。

---

## 排查清单

| 现象 | 原因 |
|---|---|
| 启动 panic `auth.verify_addr is required` | 游戏服角色（game/gateway/all）没配账号服地址 |
| 启动报 `auth.jwt_secret is required` | 账号服角色（auth/all）没配密钥 |
| 启动报 `auth.listen is required when server_type=auth` | 账号服角色没配监听地址 |
| 启动报 `account store unavailable` | `server_type=auth` 但 `data.tier` 不是 MySQL 系 |
| 登录报「账号服务不可用，请稍后重试」 | `verify_addr` 填错 / 账号服没起（服务端日志 `auth: verify unavailable`） |
| 登录报「尝试次数过多，请稍后再试」(HTTP 429) | 账号服撞库门禁（连续 10 次失败锁 5 分钟） |
| 客户端报「HTTP 模块未初始化」 | 没调 `CloverNet.Init`（HTTP 随它挂接） |
| 客户端报「账号服无响应」 | `auth_addr` 填错 / 账号服没启动 |
| 向游戏服发注册消息无响应 | 注册在账号服：该号位已作废，游戏服不挂 handler 且门禁不放行（设计如此） |

---

## 相关

- `patterns/signup-login.md` — 登录 handler 模板
- `patterns/client/network.md` — 客户端网络模板（含登录流程）
- [`clover-doc/server/security/auth-server.md`](https://github.com/qw576483/clover-doc/blob/main/server/security/auth-server.md) — 账号服原理、HTTP 契约与配置
- [`clover-doc/client/development/auth.md`](https://github.com/qw576483/clover-doc/blob/main/client/development/auth.md) — 客户端 `CloverAuth` 用法
- `服务器待做.md` §一 S2 留痕 — 支付链路（订单归账号服，引擎侧已闭环）
