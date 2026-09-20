# 新项目脚手架模板（从零建 `clover-{name}`）

> **本文件是"无中生有"建工程的唯一依据**（自包含，不依赖任何现有 demo 工程）。
> 所有字段名均已逐条比对 `clover-server-engine` 源码确认，可直接复制使用。
>占位符约定：`{name}`=项目名（如 `cq`）、`{module}`=Go module 名（如 `clover-cq`）、
> `<ENGINE_PATH>`=`clover-server-engine` 的本地绝对路径。生成时替换。

## 0. 目标结构

```
clover-{name}/
├── tools/
│   ├── ai-skill/                   # ★ 项目级 skill（本项目特有约定；模板 = scaffold/project-skill.md）
│   │   └── SKILL.md                #   建项目时必须生成；动本项目代码前必须先读它
│   └── verify.ps1                  # ★ 开工四件套①：交付闸门（模板 = 全局 skill 的 reference/verify-template.md）
├── server/                        # ★【仅"形态=联机"时才有】单机项目整个不生成（见 patterns/game-demo.md §1.5）
│   ├── configs/all/server.yaml     # 引擎配置（字段见 §2.3）
│   ├── game/
│   │   ├── datadef/                # 数据 schema
│   │   ├── def/                    # 消息号 msg.go / reply.go / push.go
│   │   └── logic/                  # handler（init 挂载）
│   ├── main.go
│   ├── go.mod
│   └── go.sum
├── client/                         # ★ 由 unity-cli 创建，不要手写目录树
│   └── （Unity 工程：Packages/ Assets/ ProjectSettings/）
└── 策划/                            # 策划产物（配表与策划案分开，别混）
    ├── 数值文档/                    # ★ 配表：源表 *.txt（AI 写）+ -pack 出的 *.xlsx
    ├── 策划案/                      # 玩法 / 数值 / 需求转写的策划文档（*.md）
    ├── 验收表.md                    # ★ 交付闸门（§2：A 的系统清单逐行；开工时建）
    ├── 对照表.md                    # ★ §0.5 的执行形态：元素 | 原版值(出处) | 我们的值 | 差值
    └── 基线图/                      # ★ 开工四件套③：参考物那一侧的截图 + 场景清单（写外观代码前必须有）
```

**禁止**：手写 `client/Assets/Scripts/...` 目录树来"假装"是 Unity 工程（缺 `.meta`/`ProjectSettings`/`Packages` 就不是 Unity 工程，Unity 打不开）。

### 0.0 开工四件套（来自全局 skill §0.7 —— **缺哪个，就不许进哪个阶段**）

| 件 | 何时就位 | 怎么来 |
|---|---|---|
| ① `tools/verify.ps1` 闸门 | **建项目时** | 从全局 skill 的 `reference/verify-template.md` 复制，按项目改路径 |
| ② 强制层（让闸门在"必经点"自动跑） | **建项目时** | 优先**通用层**：git `pre-commit`/`pre-push` 或 CI 跑 `tools/verify.ps1`（**与宿主无关**）；有钩子的宿主再加钩子 —— 适配表见 `reference/deterministic-gates.md` 第五节 |
| ③ `策划/基线图/` + 场景清单 | **写第一行外观/UI 代码之前** | 采集参考物那一侧：固定分辨率 / 冻结动画 / 固定相机位姿（流程见 `reference/visual-loop.md`） |
| ④ diff 闭环器 | **写第一行外观/UI 代码之前** | 通用引擎；换引擎只换"采集器"（同上） |

⛔ ①② 缺 ⇒ **不许写任何代码**；③④ 缺 ⇒ **只许做逻辑/数据，不许碰外观/UI**。

### 0.1 项目级 skill：`<项目根>/tools/ai-skill/`（★ 建项目时**必须**生成）

在项目根创建 **`tools/ai-skill/` 目录**，按 **`scaffold/project-skill.md`** 的结构生成 **4 本必填分册**
（`SKILL.md` / `conventions.md` / `registry.md` / `constraints.md`），删掉占位内容、按项目实情填：

```text
<项目根>/tools/ai-skill/
├── SKILL.md            # ★ 入口/索引（必填，<100 行）：项目一句话 + 各册索引 + 硬规则
├── conventions.md      # ★ 本项目约定（必填）：消息号段分配、命名、目录边界
├── registry.md         # ★ 设施登记簿（必填，会最长）：消息号 / handler / 面板 / 管理器 / 通用函数 / 配表
├── constraints.md      # ★ 约束与风险（必填）：引擎/平台固有约束 + 静默失败风险
├── patterns/           # ☆ 本项目特有写法（复杂项目才拆）
└── examples/           # ☆ 可照抄的实现范例（复杂项目才拆）
```

**是复合 skill，不是单文件**（与全局 `clover-engine` skill 同构）。四本必填分册的完整模板、以及"复杂项目怎么继续拆"的判据，见 **`scaffold/project-skill.md` §0-§5**。

它是**项目级 skill**，与全局 `clover-engine` skill 分工：

| | 全局 `clover-engine` skill | 本项目 `tools/ai-skill/` |
|---|---|---|
| 范围 | 所有 Clover 项目通用（引擎 API / 范式 / 通用约定） | **只有本项目** |
| 内容 | `patterns/**`、`reference/**`、`scaffold/**` | 本项目的消息号表、handler 表、面板表、配表、约束、范例 |
| 优先级 | **规则层（`SKILL.md` §0~§7）不可被覆盖**；其余通用写法可被项目取代 | 只在「全局 skill 没写 / 明说可自选」的地方优先（技术选型、命名、目录细分、消息号分段） |

**规则**：
- ⛔ **项目级 skill 只能"加严"，不许"放宽"全局 skill 的规则层** ——
  凡全局标了 `§0~§7` / `⛔` / **硬规则** / **硬闸门** 的，项目约定一律不得冲突；
  冲突时：**照全局规则层做 → 把冲突的项目文档改掉 → 真觉得规则不合理就回报用户改 skill**。
  ⛔ **不许**用「以本项目为准」把硬规则放过（实测形态：项目级 skill 把 §1.8 禁止的 `client/_dev/`
  写成了"本项目探针位置"，子 agent 照章执行 ⇒ 该目录堆了**上百个文件**）。完整层级见 `SKILL.md` §1.10。
- 只记**本项目特有**的东西；通用规则**不许**抄进来（抄了必然两边漂移）。
- 每次新增**对外可见**的设施（消息号 / handler / 面板 / 管理器 / 通用函数 / 配表），**回来补一行** ——
  这份文件的价值全在"保持更新"上，过期比没有更糟。
- 后续任何 AI（或人）**动本项目代码前，先读 `tools/ai-skill/SKILL.md`**。

> 复杂项目还要另建 `docs/步骤文档.md` 与 `docs/agents/agent-NN-*.md`，见 `patterns/multi-agent.md`。

---

## 1. 服务端模板

### 1.1 `server/go.mod`

```
module {module}

go 1.25.0

require github.com/qw576483/clover-server-engine v0.1.0
```

> 引擎是标准 Go module：`go get github.com/qw576483/clover-server-engine@latest` 即可，**默认不需要 `replace`**。
> 引擎 `go.mod` 声明 `go 1.25.0`，新工程保持一致。
> 只有**要改引擎源码**时才加一行指到本地克隆（**必须绝对路径**，相对路径跨盘符/跨目录易失效，
> 曾导致 `replacement directory does not exist`）：
> `replace github.com/qw576483/clover-server-engine => <引擎本地绝对路径>`。
> 建完先 `go mod tidy` 再 `go build`。

### 1.2 `server/main.go`

```go
package main

import (
	"flag"
	"log"
	"os"

	"github.com/qw576483/clover-server-engine/pkg/app"

	_ "{module}/game/logic" // 触发 logic 包 init() 完成挂载
)

func main() {
	cfgPath := flag.String("config", "configs/all", "path to config dir or file")
	flag.Parse()

	if err := app.Run(*cfgPath); err != nil {
		log.Printf("app start failed: %v", err)
		os.Exit(1)
	}
}
```

- `app.Run` 接收**目录或单文件**均可：传目录会扫描该目录下所有 `*.yaml`（不递归，按文件名排序合并）。
- `configs/all` 只是约定默认值，**不是引擎硬编码**。

### 1.3 `server/configs/all/server.yaml`

⚠️ **字段为平铺结构，不是嵌套**（如 `gateway.listen_tcp`，而非 `gateway.tcp.addr`）。以下为最小可用集，已显式写出所有"空值会静默降级"的字段，以及**硬必填**的 `auth.*`（缺失直接 panic / 启动失败）：

```yaml
server_type: "all"            # 必需：game|gateway|all|master|log|auth，缺失直接报错

gateway:
  listen_ws:  "127.0.0.1:8001"  # WebSocket；空=不启用
  ws_path: "/ws"
  ws_allow_all_origins: true    # 测试工具建议 true
  listen_tcp: "127.0.0.1:8002"  # TCP；空=不启用
  listen_udp: "127.0.0.1:8003"  # UDP(QUIC+裸UDP 共享)；空=不启用
  enable_wt: true               # WebTransport 复用 listen_ws 的端口号(UDP侧)，无独立地址字段
  max_frame_size: 1048576
  reconnect_grace: 30s          # 引擎默认 0=关闭；此处为模板示例值
  disconnect_grace: 10s         # 引擎默认 0=立即触发；此处为模板示例值
  # tls_cert / tls_key 必须成对配置；不配即明文
  # tcp_tls_disabled: false     # 默认 false：配了证书则 TCP 口也走 TLS（TCP/WS 下限 1.2）
  #                             # 客户端 GameConfig.UseTls 必须与之相反；设 true 保留明文 TCP 会打启动告警
  # queue_cap: 1000             # >0 启用等候队列（限流/满载时排队而非直接拒）
  # queue_release_per_sec: 50   # 排队放行速率；开排队后客户端会收到 EMsgQueuePosition（前面还有 N 人）

logic:
  listen_addr: "127.0.0.1:8011" # 必需：网关上游，空则网关拨不通逻辑服
  http_listen: ""
  heartbeat: 30s
  frame_timeout: 30s
  reconnect_grace: 30s

# 账号服：登录链路的必经依赖（all 角色会一并启动账号服）
# 登录链路：客户端 HTTP 调 {verify_addr}/auth/login 换 token → 长连接发 EMsgLogin{token}
#           → 游戏服 POST {verify_addr}/auth/verify 换 owner
auth:
  listen: "127.0.0.1:8051"             # 账号服 HTTP 监听（auth / all 角色必填）
  jwt_secret: "dev-only-change-me"     # JWT 签名密钥（HS256）；生产务必用强随机串
  token_ttl: 2h                        # 签发 token 有效期；<=0 用内置默认 2h
  verify_addr: "http://127.0.0.1:8051" # 游戏服校验凭证的账号服地址（game/gateway/all 必填，缺失即 panic）

data:
  tier: "TierRedisMySQL"        # 禁止 TierMemory（会被 loadConfig 拒绝）
  auto_create_table: true
  table: "data"
  redis_key_prefix: "clover:data"
  redis:
    addr: "127.0.0.1:6379"
    pass: ""
    db: 0
  mysql:
    host: "127.0.0.1"
    port: 3306
    user: "root"
    pass: ""
    db_name: "clover"
    charset: "utf8mb4"
    parse_time: true

# 以下为顶层字段，不在任何子结构下
master_listen_addr: "127.0.0.1:8021"
master_addr: "127.0.0.1:8021"
log_listen_addr: "127.0.0.1:8031"
log_addr: "127.0.0.1:8031"        # 仅当 etcd 发现不到 log 实例时作为兜底
# log_backend: "mysql"            # 日志落盘后端；空=内置 mysql。填 app.RegisterLogBackend 注册的名字可换后端
# log_backend_config: {}          # 自定义后端的参数（原样交给后端工厂）

nats:
  addr: ""                      # 顶层，不在 data 下；空=不启用（连不上仅 warn 降级）
nats_subject: "clover.notify"

log:                            # 注意是 log，不是 logging
  level: "debug"
  format: "console"
  dir: "./logs"
  stdout: true

admin:
  disable: false
  listen_addr: "127.0.0.1:8041" # 空=零值回落固定 127.0.0.1:8041（不是随机端口）；关闭用 disable: true
  shutdown_timeout: 5s

misc:
  timezone: "Asia/Shanghai"

etcd:
  endpoints: []                 # 空=不启用服务发现

# master_health / master_session_token / reliable 三段整段省略即可，零值回落默认
```

**约束**：
- `data.tier` 禁止 `TierMemory`；合法值 `TierRedis` / `TierRedisMySQL` / `TierSnapshot`。
- 监听地址缺失**不报错而是静默降级**（admin 空串=固定回落 `127.0.0.1:8041`，其余监听多为「不启用」），所以必须显式写。
- viper 只认 `mapstructure` tag 自定义字段需两个 tag 都写。

### 1.4 `server/game/def/msg.go`（消息号）

```go
package def

// 业务消息号必须 >= 10001（引擎占 [1,10000]，启动期校验，误用 panic）
//
// 分段按团队约定 reference/conventions.md §1，**不要自创起点**：
//   C2S  : 1000101 起（本模块首号 1000101；下一个模块 1000201 …）
//   Push : 3002001 起
//   回包 : **不定义消息号常量** —— 引擎按 requestID 配对，回包帧的 msgID 恒为 0，
//          只定义回包**结构体**（历史上那批 2000101+ 常量全仓零引用，
//          且会让人误以为回包有协议号）。
const (
	MsgXxx = 1000101 // C2S

	PushXxx = 3002001 // Push
)

type XxxRequest struct {
	Name string `json:"name"`
}

// XxxReply 回包体：回包不占消息号。
type XxxReply struct {
	OK  bool   `json:"ok"`
	Err string `json:"err,omitempty"`
}
```

### 1.5 `server/game/logic/logic.go`（挂载 + handler）

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

// ★ handler 签名是 event.Ctx（值接口），不是 *event.Ctx
func (l *gameLogic) onXxx(c event.Ctx) error {
	var req def.XxxRequest
	if err := c.BindMsg(&req); err != nil { // ★ BindMsg，不是 Bind
		l.g.Alert(c, &proto.EAlertNotify{Title: "错误", Content: "参数错误"})
		return nil
	}

	// 业务逻辑…

	l.g.Reply(c, def.XxxReply{OK: true}) // ★ 回包用 g.Reply
	return nil
}
```

**API 铁律**（与 `patterns/handler.md` 一致，照抄不会编不过）：
- 签名 `func(c event.Ctx) error`；import [`clover-server-engine/pkg/transport/event`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/transport/event.md)。
- 取请求体 `c.BindMsg(&req)`；回包 `g.Reply(c, v)`；弹窗 `g.Alert(c, &proto.EAlertNotify{Title, Content})`。
- `c.MarkReplied(body []byte)` 收 **[]byte**，一般不用它，用 `g.Reply` 即可。
- 返回非 nil → 框架自动回错误包；返回 nil 且未回包 → 不回包（fire-and-forget）。

---

## 2. 客户端模板（★ 必须用 unity-cli 创建工程）

### 2.0 动手前：先确认 360 已关闭（硬闸门，不过就别建）

**360（及同类国产杀软）会拦截 Unity 的安装目录与辅助进程。** 实测症状：
`Unity.Licensing.Client` 进程存在但通道永远拒绝连接（其内部在崩），编辑器于是无限重连：

```
[Licensing::IpcConnector] Connection to channel LicenseClient-XXX refused
[Licensing::Module] Timed-out after 60.00s, waiting for channel
Another instance of Unity.Licensing.Client is already running.
```

**每轮重连烧 60 秒且会一直循环** —— 主观感受就是"创建工程跑一小时没动静"。
**这不是慢，是根本起不来；360 开着时工程创建不出来。**

做法：
1. **开工前先问用户**：「360 关了吗？」
2. 用户说没关 → **别创建工程**，让他先关，关完再来。
3. **不确定 = 按没关处理**，先问，不许赌。
4. 已经卡住了才发现 → 别硬等，回到第 1 步去问（`reference/unity-cli.md` §9 的排查会指向同一个原因）。

### 2.1 创建 Unity 工程（唯一正确入口）

```bash
# 1) 自检 CLI（没有就自动装，见 reference/unity-cli.md）
unity --version
unity editors list --format json     # 确认有 Unity 6 (6000.x)

# 2) 创建工程（先查真实模板 id，不要猜）
unity templates list --editor 6000.0.47f1 --format json
unity projects create "client" \
  --path "clover-{项目名}" \
  --editor-version 6000.0.47f1 \
  --template com.unity.template.3d
# ⇒ 工程根 = clover-{项目名}/client/（目录名固定 "client"，Hub 显示名也是 client）
```

> `clover-client-unity-engine` 包声明 `unity: 2022.3`，Unity 6 可打开并升级，
> 而 AI 自动化操作强制走 Unity 6，故创建即选 6000.x。

### 2.1.1 建完必须让用户「看得到」工程（硬约束）

> ⚠️ **本节与 §2.1.2 是「收尾动作」，必须放在本文档所有"写文件"步骤全部做完之后** ——
> 也就是 §2.2（manifest，**含 P-0 的 `com.unity.pipeline`**）+ §2.3（asmdef）+ §2.3.5（Def）
> + §2.4（config）+ 你自己写的全部代码**都落地了**，
> **才让用户去「添加项目到 Hub + 打开编辑器」**。
>
> **为什么**：用户打开编辑器是一次**昂贵且不可打断**的操作（首次导入素材+编译动辄几分钟）。
> 你在交给用户之后再补一个包/改一次 manifest，用户就得**再重启一次**。
> 实测代价：用户为此重启了三次。

`unity projects create "client" --path <项目根>` 建出来的是 `<项目根>/client`，
而 **Hub 列表显示的标题 = 最后一级目录名 = `client`**；用户机器上常有好几个同名 `client` 
（实测：`clover-cq/client`、`Atlantic/Project/client`…），用户**根本认不出**哪个是新的。

建完立刻：

```bash
unity projects add "<项目根>/client"     # 登记进 Hub（create 不保证已登记）
unity projects pin "<项目根>/client"     # 置顶到 Favorites
unity projects list --format json        # 复核 isFavorite=true
```

交付说明必须给出「Hub 里的名字 + 绝对路径 + 一条 `unity open "<绝对路径>/client"`」。
**"建在磁盘上"不等于交付，用户看不见就是没交付。**

> 给用户的**具体点击步骤**（添加 → 从磁盘添加项目 → 选中 `client` 文件夹本身）见 §2.1.2 的话术块。
> 两节要一起给：这一节解决"看得见"，§2.1.2 解决"由用户打开"，缺一个用户都卡住。

### 2.1.2 建完**交给用户打开**；用户不开就不继续（★★★ 硬闸门）

**这一条是为了保护"后面只有一条能走的路"不被自己堵死。**

唯一正确的节奏：

```
① AI：创建 client/ + 写好全部代码 + 写好工程生成器（Editor 脚本）
        ↓
② AI：★【在让用户「添加项目到 Hub、打开编辑器」之前】先把 com.unity.pipeline 写进 manifest
        （见 §2.2 的 P-0；不做这步 = 用户白开一次、还得再重启）
        ↓
③ AI：才让用户去「添加项目到 Hub + 打开编辑器」，并给出「可直接照做的 Hub 步骤」（见下）
        ↓
④ 用户：打开（这一步只能用户做，AI 做不了）—— 首次启动即带 Pipeline 服务
        ↓
⑤ AI：驱动那个**活着的编辑器**（unity status / unity command / editor_play）
       —— 编译、跑工程生成器、进 Play、截图自审
```

**② 必须给出的具体步骤话术**（**不许只说"请打开工程"** —— 用户不知道点哪，
更不知道要选 `client` 而不是它的上一级）：

> 请把这个工程加到 Unity Hub 并打开（首次导入素材会慢几分钟，属正常）：
> 1. 打开 **Unity Hub** → 左侧「**项目 / Projects**」页
> 2. 点右上角「**添加 / Add**」→「**从磁盘添加项目… / Add project from disk**」
> 3. 文件夹选择框里定位到 **`<项目根绝对路径>`**，**选中里面的 `client` 文件夹本身**
>    ——⚠️ **选 `client`，不是它的上一级**（`client` 才是 Unity 工程根：含 `Assets/`、`Packages/`、`ProjectSettings/`）
> 4. 点「**选择文件夹 / Select Folder**」
> 5. 点列表里的 `client` 打开 → 等导入 + 编译完
> 6. **打开后回我一声**，我继续
>
> 路径（可直接复制）：`<项目根绝对路径>/client`

> 第 2~4 步也可用命令行代劳：`unity projects add "<项目根>/client"` + `unity projects pin "<项目根>/client"`。
> **第 5 步（真的打开）只能用户做**，且必须等用户确认后再继续。

**硬性禁止**：

- ❌ **创建完工程后自己跑 `unity run` / `unity test` / `Unity.exe -batchmode` 去"顺便"编译验证。**
  首次导入几百上千张素材 + 全量脚本编译会把**一次往返拖到十几分钟**；且授权/杀软任一环节
  出问题就是 2.0 那种**静默卡死**，时间全烧在等它返回上。
- ❌ 用户没打开编辑器时"先跳过实测，把代码交了"。
- ❌ **用户不开编辑器还继续往下做。** 正确动作是**停下**，明确说清"我需要你打开 `client/` 文件夹"，
  然后**不继续**。
- ❌ 用"我自己批处理一下也行"绕过 ③。

**为什么必须这样**：批处理模式还有个硬限制 —— `-executeMethod` **执行完立刻退出，进不了播放模式**，
所以"跑一遍看画面"在批处理下**根本做不了**；而用户开着编辑器时 `unity run` 又会直接报
「项目已在运行中的编辑器中打开」。**活编辑器是唯一能实测的路径，且只能由用户开启。**

> **没有例外。** 即使用户说「你就自己跑，别烦我」，也**不做** ——
> 把原因讲清楚：批处理下 `-executeMethod` 执行完立刻退出、**进不了播放模式**，
> 自己跑既慢又验不了画面。
> **用户不打开编辑器 = 停在这里，不许继续往下做。**

### 2.2 接入 Clover 引擎包

> ### ★★ P-0：`com.unity.pipeline` 必须在这里就加进去
> ### （**在让用户「添加项目到 Hub、打开编辑器」之前** —— 不是之后）
>
> **为什么必须在这一步做**：Pipeline 的 HTTP 服务**只在编辑器启动 / 域重载时拉起**。
> 如果用户**已经打开了**工程、你才想起来要加这个包 —— 对一个跑着的编辑器**毫无作用**：
> `unity pipeline list --json` 的 `instances` 恒为 `[]`、`unity status` 永远是空表，
> **用户必须再重启一次** Unity 才连得上。
>
> **实测代价**：因为这条没写进模板，用户被迫重启了 **3 次**。
>
> **做法**（二选一，**都必须在让用户去添加项目 / 打开编辑器之前做完**）：
>
> | 方式 | 命令 / 写法 |
> |---|---|
> | ① 手写 manifest（推荐，零额外步骤） | 在下面的 `dependencies` 里加上 `"com.unity.pipeline": "0.7.0-exp.1"` |
> | ② CLI | `unity pipeline install --project-path "<项目根>/client"` |
>
> **交出去之前自检**：`Select-String -Path client/Packages/manifest.json -Pattern 'com.unity.pipeline'`
> 有命中才让用户打开。没有命中就别催他开 —— 开了也白开。

编辑 `client/Packages/manifest.json`。**只加引擎包一行是不够的** —— 模板自带的包清单里
**没有 Unity 内置模块**，而引擎用到它们；缺了会在编译引擎时炸：

```
error CS1069: The type name 'RuntimeAnimatorController' could not be found in the namespace
'UnityEngine'. ... Enable the built in package 'Animation' ... to fix this error.
```

因此 `dependencies` 要写成「引擎包 + 测试框架 + 引擎所需内置模块」：

```json
{
  "dependencies": {
    "com.clover.unity-engine": "file:<ENGINE_CLIENT_PATH>",
    "com.unity.feature.development": "1.0.2",
    "com.unity.pipeline": "0.7.0-exp.1",
    "com.unity.test-framework": "1.4.5",
    "com.unity.ugui": "2.0.0",
    "com.unity.modules.animation": "1.0.0",
    "com.unity.modules.assetbundle": "1.0.0",
    "com.unity.modules.audio": "1.0.0",
    "com.unity.modules.director": "1.0.0",
    "com.unity.modules.imageconversion": "1.0.0",
    "com.unity.modules.imgui": "1.0.0",
    "com.unity.modules.jsonserialize": "1.0.0",
    "com.unity.modules.physics": "1.0.0",
    "com.unity.modules.physics2d": "1.0.0",
    "com.unity.modules.screencapture": "1.0.0",
    "com.unity.modules.ui": "1.0.0",
    "com.unity.modules.uielements": "1.0.0",
    "com.unity.modules.unitywebrequest": "1.0.0",
    "com.unity.modules.unitywebrequestassetbundle": "1.0.0",
    "com.unity.modules.unitywebrequesttexture": "1.0.0",
    "com.unity.modules.unitywebrequestwww": "1.0.0",
    "com.unity.modules.video": "1.0.0",
    "com.unity.modules.xr": "1.0.0"
  },
  "testables": ["com.clover.unity-engine"]
}
```

- `ENGINE_CLIENT_PATH` = `clover-client-unity-engine` 的本地绝对路径（正斜杠）。
- 内置模块的**权威名单**以本机编辑器为准：`<Editor>/Data/Resources/PackageManager/BuiltInPackages/`
  下列出的 `com.unity.modules.*`；上表是引擎实际用到的子集，可再按需增补。
- `testables` 让引擎包自带的 `Tests/`（EditMode + PlayMode）被编译进测试程序集。
  **不加的症状是「静默的」**：`unity test` 正常退出、报告 `testcasecount="0"`，一个测试都不跑。

### 2.3 业务程序集 `client/Assets/Scripts/{Name}.asmdef`

> 位置是 `Assets/Scripts/`（覆盖 `Scripts/` 下的 Core / Network / Room / Table / UI 全部业务代码），
> 与 `SKILL.md` 的目录骨架、`patterns/client/config.md` 的固定路径（`Assets/Scripts/Core/ClientConfig.cs`）一致。
> 放在 `Assets/{Name}/` 会导致 `Assets/Scripts/` 下的业务脚本落到默认的 `Assembly-CSharp`，**反而不受约束**。


```json
{
  "name": "{Name}",
  "rootNamespace": "{Name}",
  "references": [
    "CloverEngine.Core",
    "CloverEngine.Network",
    "CloverEngine.Data",
    "CloverEngine.Resource",
    "CloverEngine.Presentation"
  ],
  "includePlatforms": [],
  "excludePlatforms": [],
  "allowUnsafeCode": false,
  "overrideReferences": false,
  "precompiledReferences": [],
  "autoReferenced": true,
  "defineConstraints": [],
  "versionDefines": [],
  "noEngineReferences": false
}
```

### 2.3.5 业务消息号与协议定义（必须先建，再写任何网络代码）

**目录结构**（在 `client/Assets/Scripts/Def/` 下）：

```
Def/
├── MsgDef.cs      # 业务消息号常量（唯一定义处，禁止散落在 GameManager/UI 脚本里）
└── ProtoDef.cs    # 协议结构体（Request / Reply / Notify DTO）
```

**`client/Assets/Scripts/Def/MsgDef.cs`**

```c#
namespace {Name}.Def   // ← 换成实际项目名（如 CQ.Def），不要复用 CloverEngine 命名空间
{
    /// <summary>
    /// 业务消息号，与 server/game/def/{msg,reply,push}.go 一一对应，改动必须同步。
    /// 引擎消息号见 CloverEngine.EMsg（[1,10000]），业务必须 >= 10001。
    /// </summary>
    public static class MsgDef
    {
        // ---- C2S（1000101 起；引擎硬约束只有「> 10000」）----
        public const uint Xxx = 1000101;   // 示例请求

        // ---- 回包：**不定义消息号常量**（回包帧 msgID 恒为 0，用 Call<XxxReply> 配对）----

        // ---- 推送（3002001 起）----
        public const uint XxxNotify = 3002001;
    }
}
```

**`client/Assets/Scripts/Def/ProtoDef.cs`**

```c#
namespace {Name}.Def
{
    public class XxxRequest  { public string name; }
    public class XxxReply    { public bool ok; public string err; }
    public class XxxNotify   { public int value; }
}
```

**使用**：

```c#
using {Name}.Def;   // 引擎消息号仍用 CloverEngine.EMsg，业务用 MsgDef，两者不混

Game.Net.Send(MsgDef.Xxx, new XxxRequest { name = "test" });
var reply = await Game.Net.Call<XxxReply>(MsgDef.Xxx, new XxxRequest { name = "test" });
Game.OnMsg(MsgDef.XxxNotify, ctx => { var n = ctx.Bind<XxxNotify>(); /* ... */ });
```

**铁律**：
- 消息号**只能**定义在 `Def/MsgDef.cs`；**禁止**在 `GameManager.cs` / UI / 业务脚本里写 `const uint` 消息号或裸字面量（如 `Game.Net.Send(10001, ...)` 一律算错）。
- 协议结构体**只能**定义在 `Def/ProtoDef.cs`。
- 客户端 `MsgDef` 与服务端 `server/game/def/` **逐条对齐**，改一端必须同步另一端。

### 2.4 客户端配置 `client/Assets/Configs/config.json`

```json
{
  "server": {
    "addr": "127.0.0.1:8002",
    "udp_addr": "127.0.0.1:8003",
    "tls": true,
    "auth_addr": "http://127.0.0.1:8051",
    "call_timeout": 10,
    "max_reconnect_count": 5
  }
}
```

> `tls` 必须与服务端 `gateway.tcp_tls_disabled` **相反**：服务端配了 `tls_cert` 后 TCP 口也走 TLS（默认 `false`），
> 所以新工程直接写 `true`；写成 `false` 连上去的表现是「连上就断」。证书只走系统信任链（无跳过校验开关）。

> **端口必须与 `gateway` 段严格对齐**：`addr` = `gateway.listen_tcp`（**8002**），
> `udp_addr` = `gateway.listen_udp`（**8003**）。
>`auth_addr` = 账号服 HTTP 地址（`server.yaml` 的 `auth.listen`，**8051**）。**必填**：
> 引擎只有一种登录模式（账号服校验），客户端必须先 HTTP 换 token 再 `EMsgLogin{token}`，
> 留空则登录 / 注册必失败。详见 `patterns/auth-server.md`。
>⚠️ **绝对不要填 8001**。8001 是 `gateway.listen_ws`（WebSocket），是给浏览器 / WebGL 客户端的。
> Unity 原生客户端走的是**裸 TCP**（引擎 `Connection.cs` 用 `TcpClient` + `NetworkStream`），
> 连到 8001 会出现这种极难定位的现象：
> **TCP 能连上（HTTP 监听器接受了连接）→ 立刻被断开 → 自动重连 → 重连耗尽报「连接被踢出」
> → 之后所有 `Call`（登录/建房/同步）全部超时。**
> 客户端日志特征：`[Clover][Network] reliable link connected (…)` 紧接着 `reliable link lost (…)` / `reconnecting in …s`
> （链路日志由 `Game.Logger` 以 `Network` tag 输出，可在 `logs/` 文件日志里检索）。

**必须连同加载器一起生成**：`client/Assets/Scripts/Core/ClientConfig.cs`（范式见
`patterns/client/config.md`，可直接照抄）。**只建 `Configs/` 目录或只放 json、不写加载器 = 未完成** ——
那样业务会继续把地址/账号/密码/超时写死在代码里（历史事故：同一份超时还在两处各写一遍）。

### 2.5 纯单机项目（不接服务端）

用户明确要求「单机 / 纯本地 / 不要服务器」时：**不生成 `server/`**，
也不生成 `config.json` 的 `server` 段（或标注为未使用）。客户端只要最小两步：

```csharp
using CloverEngine;
using UnityEngine;

public class GameMain : MonoBehaviour
{
    void Start()
    {
        // 1. 启动引擎。Game.Launch 不做任何网络连接——单机不需要 ServerAddr
        Game.Launch(new GameConfig
        {
            LogDir = "logs",
        });

        // 2. 挂载输入 + 建立 EventSystem。有 UI / 键鼠操作就必须有，且必须在构建 UI 之前
        CloverInput.Init();

        // 3. 其它本地模块按需挂载（都不是必需）
        // CloverRes.Init("<资源根目录>");          // 资源
        // CloverData.InitDataTable("<表目录>");    // 配表

        // 4. 构建 UI / 开始游戏
        //    ★ 「一键出游戏」交付必须有启动链路：这里不是"直接进战斗场景"，
        //      而是交给流程模块进「启动画面 → 主菜单 → 创角/选角 → 读条进图」。
        //      写法见 patterns/client/app-flow.md；分层落点 Module/Flow/ + UI/ + App/Bootstrap.cs。
        BuildUi();
    }
}
```

**关键事实**（均在引擎源码里确认过）：

| 事实 | 说明 |
|---|---|
| `Game.Launch` **不联网** | 只初始化 Logger / Dispatcher / Event / Timer / Fsm / Setting + 创建引擎驱动器；不会去连 `ServerAddr` |
| `CloverNet.Init` 是**可选**的 | 只有联机项目才调；单机完全不调 |
| 单机下 `Game.Net == null`，**不会崩** | 引擎内部一律 `Net?.Tick()` / `Net?.Disconnect()`，空安全；但业务侧别去调 `Game.Net.*` |
| `CloverInput.Init()` **单机也必须有** | 输入与 UI 事件系统跟服务器无关 |
| `CloverData` / `CloverRes` 也是可选的 | 不用配表 / 资源就不调 |

**交付要求**：单机任务的交付说明首行必须标注「单机版（用户指定，不含服务端）」，
其余交付标准（打开即跑、资源欠缺清单）不变。

**不许做的事**：为"省事"把用户点名的**联服**玩法改成单机 —— 那是静默降级，见 `SKILL.md` 环境自检章节。

---

## 3. 初始化检查清单（建完逐条核对）

- [ ] **`.gitignore` 按 Unity 模板写好**（⚠️ 别等提交时才发现；建项目就写）：
      `client/{Library,Temp,obj,Logs,Build,Builds,UserSettings,setting}/`、`*.csproj`、`*.sln`、**`*.slnx`**、
      `.vsconfig`、`.vs/`、`.idea/`、`.ai-tmp/`、`原版资源/`。
      **实测代价**：某项目第一次准备提交时 `client/Library` 已经 **1769 MB / 26821 文件** ——
      忽略规则少一行，仓库就多一个 GB 级缓存（且**忽略只挡提交、不会让磁盘变小**）。
      判据：`git --git-dir=<tmp> --work-tree=<项目> status --porcelain -uall` 里
      `Library/` / `Logs/` / `Temp/` / `.ai-tmp/` / `原版资源/` 的**计数全为 0**。
      ⚠️ 这条**只能验"项目自己的 `.gitignore` 够不够"**（临时 git 目录的 work-tree = 项目根，
      **看不见工作区级 `.gitignore`**）。**必须再补一次真实仓库判定**，否则会假绿：

      ```powershell
      # ① 项目是否被上层 .gitignore 整目录吞掉（实测踩过：git add 报 "paths are ignored"）
      git -C <工作区根> check-ignore -v -- <项目根相对路径>
      # ② 真实仓库里"会被提交的文件数 / 危险项计数"（pathspec 限定本工程，别用 -A）
      git -C <工作区根> add -n -- <项目根相对路径> | Measure-Object | Select-Object -ExpandProperty Count
      git -C <工作区根> status --porcelain -uall -- <项目根相对路径>
      ```

      **实测代价（2026-09-20，`clover-project-diablo2`）**：临时目录判据给出"危险项全 0、
      24788 条待提交"，看着完美；真实 `git add` 却直接
      `The following paths are ignored by one of your .gitignore files: clover-project-diablo2`
      —— 工作区级 `.gitignore` 早有 `/clover-project-diablo2/`（**有意排除**：该工程 8.7 GB）。
      ⇒ **"不建仓也能验"的说法要收紧：项目内忽略规则可以离线验，`是否真的入得了仓`必须问真实仓库。**
- [ ] **`.gitattributes` 存在**：二进制类型（`*.png/*.wav/*.unity/*.prefab`）+ 需要随交付给原版素材时 `*.zip filter=lfs`（`reference/asset-sources.md` §8.2）。

```
□ server/go.mod 的 replace 用绝对路径，且 go mod tidy 无错
□ server/configs/all/server.yaml 字段为平铺结构（对照 §1.3）
□ data.tier 不是 TierMemory
□ 所有监听地址显式配置（server_type/gateway/logic/master/log/admin）
□ 消息号 >= 10001
□ handler 签名 func(c event.Ctx) error，import 为 pkg/transport/event
□ client 由 unity projects create 生成（有 Packages/ + ProjectSettings/）
□ manifest.json 已加 com.clover.unity-engine 本地依赖
□ client/Assets/Configs/config.json + Core/ClientConfig.cs 均已生成，业务代码无硬编码地址/账号/密码/超时
□ server.addr 填的是网关 TCP 口（8002），不是 WS 口（8001）
□ server.tls 与服务端 gateway.tcp_tls_disabled 相反（服务端默认 false → 客户端 true）
□ import 路径已替换为实际 module 名，无 {module} 占位符残留
□ 服务器环境已自检（env.exe info 就绪）；未就绪时未擅自降级为离线/单机版
□ 用户点名的功能全部真实实现（无"暂未实现 / 框架已预留"）
□ 客户端场景已创建并保存、已入 Build Settings、业务脚本已挂载（打开工程点 Play 即跑）
□ unity console tail 无编译错误
□ 引用了外部资源 → client/资源欠缺清单.md 已生成
□ 业务消息号已集中在 client/Assets/Scripts/Def/MsgDef.cs（脚本里无散落消息号字面量）
□ 客户端 Def 与服务端 game/def 消息号逐条对齐
□ <项目根>/tools/ai-skill/ 已生成（SKILL.md 按 scaffold/project-skill.md 填好：消息号 / handler / 面板 / 配表 / 约束）
□ **项目级 skill 没有放宽全局规则层**（grep `以本项目为准|优先于全局`，逐条判"加严"；见 SKILL.md §1.10）
□ 策划/数值文档/ 与 策划/策划案/ 两个目录已建（配表与策划案不混放）
□ **交付前跑完 SKILL.md §1.11 的 7 条机械自检**（能脚本化就做成 `tools/verify.ps1`，照 `reference/verify-template.md`），并把原始输出贴进回报
□ `策划/验收表.md` **每行带「类别」列**（`数值类` / `表现类`）；`表现类` 的行能在**联络图索引表**里查到格号（`SKILL.md` §2 硬性判定第 3 条）
□ 交付前的证据**只采一次**（一张或数张**联络图**，见 `reference/visual-loop.md` 第八节）；⛔ 没有逐项截图、没有逐张让 AI 读图
□ `策划/验收表.md` 带「允许的差异」+「机械自检记录」两节；**汇总数字 = 表体统计**
□ 项目里**不存在**交接/进度类文档（`docs/交接-*.md` / `NEXT.md` / `docs/进度*.md`）
□ 一次性产物只在 `<项目根>/.ai-tmp/`（⛔ 无 `client/_dev/`、无项目根散落脚本），已写进 `.gitignore`
```

## 4. 编译与运行

```bash
cd server && go mod tidy && go build -o {name}.exe .     # 服务端
unity build ./client --editor-version 6000.0.47f1 --target StandaloneWindows64  # 客户端（可选）
```

起服前先确认本地依赖（etcd/nats/redis/mysql）已就绪，排障见 `reference/server-env.md`。
