# 范式：signup / login 对称鉴权

**登录**由引擎内置：游戏服每次登录都调账号服 `/auth/verify` 换 `owner`。
**注册**完全在账号服侧（`POST {账号服}/auth/signup`），游戏服不接收注册报文（原 `EMsgSignup=1` 号位已作废保留）。

> 登录链路**唯一**（账号服校验），不存在可替换的鉴权实现。

## 1. 唯一登录路径

```csharp
// 客户端两步：① HTTP 换 token → ② 长连接发 token
var token = await CloverAuth.LoginAsync(account, password);
var reply = await Game.Net.Call<ELoginReply>(EMsg.Login, new ELoginRequest { token = token });
```

引擎侧：`EMsgLogin` → `auth.Handler` → `RemoteAuthenticator` → 账号服 `/auth/verify` → `owner`。

## 2. 注册：账号服 HTTP（无长连接报文）

注册没有 C2S 报文——原 `EMsgSignup=1` 号位已作废**保留不复用**（号位是双端协议契约，
回收会让旧客户端发出语义完全不同的报文）；服务端 `ESignupRequest` / `ESignupReply` 
与客户端同名协议体都已删除。

```text
POST {账号服}/auth/signup  {account, password} → {success, owner, token, exp, err}
```

注册成功即签发 token（注册即登录），直接接上 §1 的两步登录。

## 3. auth handler 回包写法（无 Game 引用）

auth handler 没有 `*app.Game` 引用，用 `Ctx.MarkReplied` 纯数据写入：

```go
b, _ := ujson.Marshal(v)
c.MarkReplied(b)
```

## 4. 引擎 handler 回包写法（有 Game 引用）

```go
// ELoginReply 的字段只有 Owner / Token / Success / Err / SessionKey（**没有 PlayerID**）。
// Owner 是"已认证对象标识"，网关靠它绑定会话 —— 业务要传的是 owner，不是 playerID。
// SessionKey 也不要自己填：会话通道加密由引擎的 auth.Handler 协商（客户端声明 encrypt 时生成并回填），
// 自写登录 handler 时保持空即可（回包不带密钥 = 本次会话明文，网关不会启用加解密）。
g.Reply(c, &proto.ELoginReply{Owner: owner, Token: token, Success: true})
```

## 要点

- 注册**只能**走账号服 HTTP：游戏服没有 signup handler，也没有可注入的注册器。
- 鉴权成功绑定玩家：`c.SetPlayerID(playerID)`（通知网关双 key 索引）。
- auth handler 无 Game 引用 → 用 `c.MarkReplied(b)`；引擎 handler → 用 `g.Reply(c, v)`（回包按 requestID 配对，不携带消息号）。
- 引擎内置消息号：`EMsgLogin=2` / `EMsgResumeSession=3`（1 号原为 `EMsgSignup`，已作废保留）、网关直发帧 `EMsgUDPBindGrant=6` / `EMsgQueuePosition=7`（排队位置）；推送 `EPushPlayerFullSync=4001` / `EPushAlert=4002` / `EPushDataSync=4003`。错误回包使用特殊值 `EMsgError=0xFFFFFFFF`。
- **登录门禁默认开启**（网关层）：未登录连接只放行 `EMsgLogin` / `EMsgResumeSession` 两个消息号，其余直接被网关拒（`EErrorReply{code:401}`），业务无需在 handler 里逐条判断"有没有登录"。
- **登录体系一律外置**：引擎只有一种登录模式——账号体系在独立的账号服（只暴露 HTTP），游戏服每次登录都调它的 `/auth/verify` 换 `owner`，自己**不碰账号表、不存密码**；客户端两步——先 HTTP 换 token，再 `EMsgLogin{token}`。账号服是登录链路的**必经依赖**（`auth.verify_addr` 对 game/gateway/all 必填，缺失启动即 panic）。
- **权限校验别散落在 handler**：消息级业务权限（房间归属、状态前置条件）用 `g.OnBeforeDispatch(fn)` 统一挂载，返回 `*proto.BizError` 可携带错误码（401/403/...），客户端据码分支。
