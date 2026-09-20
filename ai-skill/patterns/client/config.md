# 范式：客户端配置（config.json + 加载器）

## 何时使用

只要客户端存在「部署期会变、或不该散落在代码里」的量，就必须走配置文件，**不许写死在业务代码里**：

- 服务器地址 / 端口 / UDP 地址
- **账号服地址（`server.auth_addr`）—— 登录链路的必经依赖，必填**
- 请求超时 / 重连次数
- 开发期测试账号 / 密码 / 线路号
- 轮询间隔 / 默认昵称 / 玩法开关 / 帧率

> 历史事故：某工程「只建了目录、没建 json、也没写加载器」，结果地址/账号/密码/超时全部硬编码在
> `NetService.cs` 里，同一份超时还在两处各写一遍 —— 换环境就得改代码重新编译。
> **只建 `Configs/` 目录不算完成，必须连同加载器一起生成。**

## 目录与文件（固定路径，不要改名）

```
client/Assets/Configs/config.json            # 唯一配置入口
client/Assets/Scripts/Core/ClientConfig.cs   # 唯一加载器（静态类 Cfg）
```

## 1. `config.json` 模板

```json
{
  "server": {
    "addr": "127.0.0.1:8002",
    "udp_addr": "",
    "tls": true,
    "auth_addr": "http://127.0.0.1:8051",
    "call_timeout": 10,
    "max_reconnect_count": 5
  },
  "account": {
    "name_prefix": "p",
    "name_suffix": "_app",
    "password": "123456",
    "line": 0
  },
  "game": {
    "default_nick": "玩家一",
    "poll_interval_ms": 50
  }
}
```

⚠️ **`server.addr` 必须是网关 TCP 口**（服务端 `server.yaml` 的 `gateway.listen_tcp`，默认 **8002**）。
不要填 `8001` —— 那是 `gateway.listen_ws`（WebSocket），Unity 原生客户端走**裸 TCP**，
填错的症状是「TCP 连上 → 立刻断开 → 重连耗尽被踢 → 之后所有 `Call` 超时」。详见
`scaffold/new-project.md` §2.4。

`udp_addr` 填 `gateway.listen_udp`（默认 8003）；不需要不可靠通道就留空字符串。

`tls`：线路是否走 TLS（TCP→`SslStream`、WS→`wss://`）。**必须与服务端 `gateway.tcp_tls_disabled` 相反**——
服务端配了 `tls_cert` 后 TCP 口默认也走 TLS（`false`），所以客户端 `tls: true` 是默认形态；
两边不一致的症状是「连上就断」。证书只走系统信任链，引擎没有跳过校验的开关（§N7）。

## 2. `ClientConfig.cs` 加载器模板

照抄即可，只需替换命名空间。

```csharp
using System;
using System.IO;
using UnityEngine;

namespace {Name}
{
    [Serializable]
    public class ServerSection
    {
        public string addr = "127.0.0.1:8002";   // 网关 TCP 口
        public string udp_addr = "";             // 网关 UDP 口，留空=不启用
        public bool tls = true;                  // 线路走 TLS；与服务端 gateway.tcp_tls_disabled 相反
        public string auth_addr = "http://127.0.0.1:8051"; // 账号服 HTTP 地址（必填；登录链路必经依赖）
        public int call_timeout = 10;
        public int max_reconnect_count = 5;
    }

    [Serializable]
    public class AccountSection
    {
        public string name_prefix = "p";
        public string name_suffix = "_app";
        public string password = "123456";
        public int line = 0;
    }

    [Serializable]
    public class GameSection
    {
        public string default_nick = "玩家一";
        public int poll_interval_ms = 50;
    }

    [Serializable]
    public class RootSection
    {
        public ServerSection server = new ServerSection();
        public AccountSection account = new AccountSection();
        public GameSection game = new GameSection();
    }

    /// <summary>客户端配置入口：从 Assets/Configs/config.json 读取。</summary>
    public static class Cfg
    {
        private const string RelativePath = "Configs/config.json";
        private static RootSection _config;

        public static RootSection Config => _config ??= Load();
        public static ServerSection Server => Config.server;
        public static AccountSection Account => Config.account;
        public static GameSection Game => Config.game;

        public static string FilePath => Path.Combine(Application.dataPath, RelativePath);
        public static void Reload() => _config = Load();

        private static RootSection Load()
        {
            try
            {
                var path = FilePath;
                if (!File.Exists(path))
                {
                    Game.Logger?.Warn("Cfg", $"未找到 {path}，使用内置默认值");
                    return new RootSection();
                }
                var root = JsonUtility.FromJson<RootSection>(File.ReadAllText(path));
                if (root == null)
                {
                    Game.Logger?.Error("Cfg", $"解析失败（空/格式错误）: {path}，回退默认值");
                    return new RootSection();
                }
                // JsonUtility 对 json 里缺失的字段会留 null，逐段兜底防下游空引用
                root.server ??= new ServerSection();
                root.account ??= new AccountSection();
                root.game ??= new GameSection();
                return root;
            }
            catch (Exception e)
            {
                Game.Logger?.Error("Cfg", $"读取异常，回退默认值: {e.Message}");
                return new RootSection();
            }
        }
    }
}
```

## 3. 业务侧怎么用

```csharp
// 启动：全部从配置取
Game.Launch(new GameConfig
{
    ServerAddr = Cfg.Server.addr,
    CallTimeoutSeconds = Cfg.Server.call_timeout,
    MaxReconnectCount = Cfg.Server.max_reconnect_count,
    UseTls = Cfg.Server.tls,   // 与服务端 gateway.tcp_tls_disabled 相反（默认关了明文 TCP）
});

// 账号服地址（必填，来自 config.json 的 server.auth_addr）：
// 不设则 CloverAuth.LoginAsync / SignupAsync 会抛 InvalidOperationException
CloverAuth.AuthAddr = Cfg.Server.auth_addr;

CloverNet.Init(Cfg.Server.addr,
    string.IsNullOrEmpty(Cfg.Server.udp_addr) ? null : Cfg.Server.udp_addr);
```

## 铁律

1. 业务代码里**不许**出现地址 / 端口 / 账号 / 密码 / 超时 / 重连次数等字面量 —— 一律 `Cfg.xxx`。
2. 同一项配置**只允许一处默认值**（加载器里的字段默认值），业务里不要再写第二遍。
3. 文件缺失 / 解析失败 → 回退默认值 + 打一条可定位的警告，**绝不许抛异常**
   （配置问题不该让游戏起不来）。
4. 单机项目同样适用：把 `server` 段留作未使用即可，其它段照常读。

## 平台注意

`File.ReadAllText + Application.dataPath` 适用于 **Editor 与桌面平台**。
要出 **WebGL / Android / iOS** 时，把 `config.json` 放到某个 `Resources` 目录下，
并改成 `Resources.Load<TextAsset>("Configs/config")` 读取（`Application.dataPath` 在这些平台上不可读）。

## 常见问题

| 问题 | 现象 | 解法 |
|---|---|---|
| **配置类里有 `static Game` 成员，遮蔽了引擎的 `Game`** | 写 `Game.Logger.Info(...)` 编译报 `CS1061: "GameSection" 未包含 "Logger" 的定义` | 写全名 `CloverEngine.Game.Logger.Info(...)` |
| 引擎启动前就读配置并打日志 | 历史症状：`Game.Logger` 为 `null`，直接调用会空引用；用 `?.` 则**日志被静默丢弃** | **已由引擎修掉**：`Game.Logger` 现在永不为 null（未 Launch 时走写 Unity Console 的兜底实现）。配置类直接 `Game.Logger?.Info(...)` 即可，**不需要**再自己加 `UnityEngine.Debug` 兜底层 |
| 用裸 `Debug.Log` 打业务日志 | 绕过引擎的日志级别与落盘 | 统一 `Game.Logger.Info/Warn/Error(tag, msg)`，见 `SKILL.md`「错误处理与日志（硬约束）」 |

## 自检清单

```
□ client/Assets/Configs/config.json 存在，且 server.addr 是 TCP 口（8002，不是 8001）
□ server.auth_addr 已填（账号服地址）—— 留空则登录 / 注册必失败
□ server.tls 与服务端 gateway.tcp_tls_disabled 相反（服务端 false → 客户端 true）
□ client/Assets/Scripts/Core/ClientConfig.cs 存在
□ 全工程 grep 不到硬编码的 127.0.0.1 / 账号 / 密码 / 超时数字 / 重连次数
□ 删除 config.json 后仍能启动（回退默认值 + 警告）
```
