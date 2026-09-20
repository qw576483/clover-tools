# 网络通信模板

## 模板 0：业务消息号与协议（★ 先建 Def，再写任何网络代码）

引擎 `CloverEngine.EMsg` 是**封闭的 static class**（占 `[1,10000]`），业务消息号**不能**往里加。
业务侧必须有自己的 `Def/`，且与服务端 `server/game/def/{msg,reply,push}.go` **逐条对齐**。

**目录**（缺一不可）：

```
Assets/Scripts/Def/
├── MsgDef.cs      # 业务消息号常量（唯一定义处）
└── ProtoDef.cs    # 业务协议结构体，字段与服务端 datadef 对齐
```

**`Assets/Scripts/Def/MsgDef.cs`**

```c#
namespace CQ.Def   // ← 换成实际项目名，不要复用 CloverEngine 命名空间（会与引擎 EMsg 冲突）
{
    /// <summary>
    /// 业务消息号，与 server/game/def/{msg,reply,push}.go 一一对应，改动必须同步。
    /// 引擎消息号见 CloverEngine.EMsg（[1,10000]），业务必须 >= 10001。
    /// </summary>
    public static class MsgDef
    {
        // ---- C2S（1000101 起）----
        public const uint Match = 1000101;   // 匹配请求
        public const uint Play  = 1000102;   // 出拳

        // ---- 回包：**不定义消息号常量** ----
        // 回包帧的 msgID 恒为 0（引擎按 requestID 配对），客户端只写回包**结构体**、
        // 用 `await Game.Net.Call<MatchReply>(...)` 接收，不需要也不该有 2000101 这类常量。

        // ---- 推送（3002001 起）----
        public const uint RoundResult = 3002001;  // 回合结算推送
    }
}
```

**`Assets/Scripts/Def/ProtoDef.cs`**

```c#
namespace CQ.Def
{
    public class MatchRequest       { public int mode; }
    public class MatchReply         { public bool ok; public string room_id; }
    public class PlayRequest        { public int hand; }   // 0 石头 1 剪刀 2 布
    public class PlayReply          { public bool ok; }
    public class RoundResultNotify  { public int my_hand; public int rival_hand; public int result; public int score; }
}
```

**使用**：

```c#
using CQ.Def;   // 引擎消息号仍用 CloverEngine.EMsg（如 EMsg.Login），业务用 MsgDef，两者不混

Game.Net.Send(MsgDef.Play, new PlayRequest { hand = 0 });
var reply = await Game.Net.Call<MatchReply>(MsgDef.Match, new MatchRequest { mode = 1 });
Game.OnMsg(MsgDef.RoundResult, ctx => { var n = ctx.Bind<RoundResultNotify>(); /* ... */ });
```

**铁律**：
- 消息号**只能**定义在 `Def/MsgDef.cs`；**禁止**在 `GameManager.cs` / UI / 任意业务脚本里写
  `const uint` 消息号或裸字面量（`Game.Net.Send(1000101, ...)` 一律算错）。
- 新增/修改消息号时，服务端 `game/def/` 与客户端 `Def/MsgDef.cs` **同一批改动**，不许只改一边。

## 模板 1：注册消息监听

```typescript C#
using CloverEngine;
using UnityEngine;

public class NetworkHandler : MonoBehaviour
{
    void Start()
    {
        // 注册推送监听（业务消息号来自 Def/MsgDef.cs；EMsg 只含引擎号）
        Game.OnMsg(MsgDef.SomeNotify, ctx =>
        {
            var msg = ctx.Bind<SomeNotify>();
            Game.Logger?.Info("Net", $"收到推送: {msg.Data}");
            // 处理逻辑...
        });
    }
}
```

## 模板 2：发送消息

```typescript C#
// 可靠发送（TCP，保证到达）
public void SendChatMessage(string content)
{
    Game.Net.Send(MsgDef.ChatMessage, new ChatMessage
    {
        Content = content,
    });
}

// 非可靠发送（UDP，可能丢失，适合高频数据）
public void SendPosition(Vector3 position)
{
    Game.Net.SendUnreliable(MsgDef.PositionSync, new PositionData
    {
        X = position.x,
        Y = position.y,
        Z = position.z,
    });
}
```

## 模板 3：请求-回包

```typescript C#
// 异步请求
public async void BuyItem(int itemId, int count)
{
    try
    {
        var reply = await Game.Net.Call<BuyItemReply>(MsgDef.BuyItem, new BuyItemRequest
        {
            item_id = itemId,      // ★ 业务消息字段名必须是 snake_case，与服务端 json tag 对齐
            count = count,
        });
        Game.Logger?.Info("Net", $"购买成功: {reply.order_id}");
    }
    catch (CloverCallException ex)
    {
        // 服务端业务错误（EMsg.Error 回包），描述在 ex.ServerError
        Game.Logger?.Error("Net", $"业务错误: {ex.ServerError}");
        HandleBusinessError(ex.ServerError);
    }
    catch (TimeoutException)
    {
        // 请求超时
        Game.Logger?.Error("Net", "请求超时");
        ShowTimeoutTip();
    }
}
```

## 模板 4：网络生命周期监听

```typescript C#
public class NetworkLifecycle : MonoBehaviour
{
    void Start()
    {
        // 连接成功（本条事件无参发布 → 处理器必须零参）
        Game.Event.On("Net.OnConnected", () =>
        {
            Game.Logger?.Info("Net", "已连接到服务器");
            ShowConnectedTip();
        });

        // 连接断开（自动重连中）
        Game.Event.On("Net.OnDisconnected", () =>
        {
            Game.Logger?.Warn("Net", "连接断开，正在重连...");
            ShowReconnectingTip();
        });

        // 会话恢复成功（本条事件带参发布 → 参数类型显式写进泛型）
        Game.Event.On<EResumeSessionReply>("Net.OnResumed", data =>
        {
            Game.Logger?.Info("Net", "会话已恢复");
            HideReconnectingTip();
        });

        // 被踢出（重连次数耗尽）
        Game.Event.On("Net.OnKicked", () =>
        {
            Game.Logger?.Warn("Net", "被踢出，需要重新登录");
            ReturnToLogin();
        });
    }

    void ReturnToLogin()
    {
        // 清理本地状态
        // 返回登录界面
    }
}
```

## 模板 5：登录流程

```typescript C#
public class LoginManager : MonoBehaviour
{
    public async void Login(string account, string password)
    {
        try
        {
            // 账号服地址**必填**（来自 config.json 的 server.auth_addr）：为空即配置错误，
            // LoginAsync / SignupAsync 会抛 InvalidOperationException。
            CloverAuth.AuthAddr = Cfg.Server.auth_addr;

            // 唯一登录路径：账号密码只发给账号服（HTTP），换回 JWT 后再走长连接。
            // ELoginRequest 只有 token 一个字段——「把账号密码发进游戏服」的旧设计已删除。
            var token = await CloverAuth.LoginAsync(account, password);

            var reply = await Game.Net.Call<ELoginReply>(EMsg.Login, new ELoginRequest
            {
                token = token,
            });
            if (!reply.success)
            {
                Game.Logger?.Warn("Net", $"登录失败: {reply.err}");
                return;
            }
            // ★ 断线恢复前提。第二个参数**传 null**（不是 reply.session_key；与官方 Sample 一致）——
            //   session_key 是会话通道加密密钥（AES 通道密钥），当恢复凭证上交会让恢复会话
            //   token mismatch 被踢；真正的恢复凭证 session_token 由 PushPlayerFullSync
            //   下发并**非空覆盖**。传了非空值引擎会打 Warn（NetworkManager.cs:715-718）。
            Game.Net.SetupSession(account, null);
            Game.Logger?.Info("Net", $"登录成功: owner={reply.owner}");
            EnterMainCity();
        }
        catch (CloverCallException ex)
        {
            Game.Logger?.Error("Net", $"登录失败: {ex.ServerError}");
        }
    }

    void EnterMainCity()
    {
        Game.Scene.Load("MainCity");
    }
}
```

## 模板 6：取消监听

```typescript C#
public class NetworkHandler : MonoBehaviour
{
    // OnMsg 返回 void，没有句柄；注销要传回「同一个方法引用」（或按 msgID 全清）
    private void OnSomeNotify(NetCtx ctx)
    {
        var msg = ctx.Bind<SomeNotify>();
        // 处理逻辑
    }

    void Start()
    {
        Game.OnMsg(MsgDef.SomeNotify, OnSomeNotify);
    }

    void OnDestroy()
    {
        Game.OffMsg(MsgDef.SomeNotify, OnSomeNotify);   // 只移除这一个处理器
        // Game.OffMsg(MsgDef.SomeNotify);              // 移除该 msgID 的全部处理器
    }
}
```

## 模板 7：服务端排队与通道加密（**引擎自动，业务不接线**）

这两件事都在引擎里闭环，业务**不要**自己实现、也不要给 `ELoginRequest` 填字段：

| 能力 | 引擎行为 | 业务能用的 |
|---|---|---|
| 排队位置 | 服务端限流/满载时连接进等候队列，网关直发 `EMsg.QueuePosition`（`requestID=0`）；入队一次 + 位置变化按 3s 刷新；排队期间顺延未决 `Call` 的超时，但**顺延有总期限**：`max(60s, CallTimeoutSeconds)`（`NetworkManager.cs:99,409-414,863-865`，E-net-01），达总期限后不再顺延、按超时结束 | 事件 `Net.QueuePosition`（参数 `EQueuePositionNotify{ahead,total,ticket}`）+ `Game.Net.IsQueued / QueueAhead / QueueTotal` |
| 会话通道加密 | 登录时按平台能力自动声明 `ELoginRequest.encrypt`；服务端回 `ELoginReply.session_key` 后，双方**整帧** AES-256-GCM 加解密 | `Game.Net.IsChannelEncrypted`（只读状态）；不需要业务代码 |

```csharp
// 排队等待界面（可选，但强烈建议：不然玩家看到的是"卡在登录"）
Game.Event.On<EQueuePositionNotify>("Net.QueuePosition", pos =>
{
    Game.UI.Toast($"前面还有 {pos.ahead} 人");
});

// 排查用：本次连接到底加密了没
Game.Logger?.Info("Net", $"encrypted={Game.Net.IsChannelEncrypted}");
```

> 平台不支持 AES-GCM（如 WebGL）时引擎**不会**声明加密，服务端保持明文、由 wss/QUIC/WT 的 TLS 兜底——
> 这是刻意的，不要为了"看起来一致"去手填 `encrypt`（会拿到密钥却解不开）。
