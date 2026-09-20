# 范式：新建客户端 Unity 工程（用 Unity CLI，禁止 mkdir）

## 何时使用

- 用户要「新建一个游戏 / 新起一个项目 / 从零做一个 demo」，且需要 Unity 客户端时
- 发现 `client/` 只有 `Assets/Configs`、`Assets/Scripts/...` 这些**空目录**，
  却没有 `Packages/`、`ProjectSettings/` → 这是假工程，按本文推倒重来

## 为什么禁止 `mkdir`

手工建目录的产物是**空壳**：

| 缺失 | 后果 |
|---|---|
| `ProjectSettings/ProjectVersion.txt` | Unity 打不开，不是合法工程 |
| `Packages/manifest.json` | 加不了包依赖，引不进 clover 引擎 |
| `.sln` / `.csproj` | 无 IDE 补全、无编译检查，写 C# 全盲写 |
| `.asmdef` | `CloverEngine.*` 引用不到，`Game.Net` / `EMsg` 全部编译错误 |

**结论：在里面写多少业务代码都是白写，必须先有真工程。**

## 步骤

### 1. 前置：CLI + Unity 6（硬门槛）

```bash
unity --version
unity editors list --format json
```

- 没有 `unity` 命令 → 自动装（Windows）：
  ```powershell
  $env:UNITY_CLI_CHANNEL='beta'; irm https://public-cdn.cloud.unity3d.com/hub/prod/cli/install.ps1 | iex
  ```
  装完**新开终端**再继续。
- **列表里必须有 Unity 6（6000.x）**。没有 → `unity install --version 6000.0.x`，
  装完重新 `unity editors list --format json` 确认。  
  **不是 6000.x 就不许往下走**，不要拿 2022.x / 2023.x 凑合。

### 2. 建工程

```bash
# 2.1 列出该编辑器真实提供的模板，不要猜 id
unity templates list --editor 6000.0.x --format json

# 2.2 创建。第一个位置参数 = 工程名，--path = 父目录
#     目标：clover-{项目名}/client/{Assets,Packages,ProjectSettings}
#     ⇒ 工程名填 client，--path 填项目根目录
unity projects create "client" --path "clover-{项目名}" \
  --editor-version 6000.0.x --template com.unity.template.2d
```

模板选择：纯 UI / 2D 玩法 → `com.unity.template.2d`；3D → `com.unity.template.3d`；
**以 2.1 实际列出的 id 为准**。

> 命令是 `unity projects create`（`projects` 复数），**不是** `unity create project`。

### 3. 引入 clover 客户端引擎包

在 `client/Packages/manifest.json` 的 `dependencies` 中加入：

```json
"com.clover.unity-engine": "file:../../clover-client-unity-engine"
```

（路径按实际仓库相对位置调整；若引擎包已发布到私有 UPM 源，改用对应的 registry 地址 + 版本号。）

### 4. 建业务程序集定义

`client/Assets/Scripts/{项目名}.asmdef`：

```json
{
    "name": "{项目名}",
    "rootNamespace": "{项目名}",
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

### 5. 建脚本目录（此时才建，且是有工程之后）

```
client/Assets/Scripts/Def/       # ★ 业务消息号 + 协议（MsgDef.cs / ProtoDef.cs；写任何网络代码前必建）
client/Assets/Scripts/Core/      # 常量 / 事件 / 场景名 / 资源路径（全项目唯一来源）
client/Assets/Scripts/Module/    # 一个系统一个模块（Module.X，如 Module.Room / Module.Game）
client/Assets/Scripts/UI/        # 面板（一个文件一个 MonoBehaviour）
client/Assets/Scripts/App/       # Bootstrap（全项目唯一手动挂载的脚本，≤ 200 行）
```

### 6. 验收（**以 `reference/unity-cli.md` 的「验收五条」为准**；本节的"四条"是它的旧副本）

```bash
cat client/ProjectSettings/ProjectVersion.txt   # 1. m_EditorVersion 必须是 6000.x
cat client/Packages/manifest.json                # 2. 必须含 com.clover.unity-engine
cat client/Assets/Scripts/{项目名}.asmdef        # 3. references 含 5 个 CloverEngine.*
ls client/*.slnx                                 # 4. 解决方案文件存在（Unity 6 生成 .slnx，不是 .sln——别因没看到 .sln 就判失败）
```

| # | 验收项 | 不通过怎么办 |
|---|---|---|
| 1 | `ProjectVersion.txt` 是 6000.x | 整份删掉 → `unity install --version 6000.0.x` → 重新 create。**不要手改版本号蒙混** |
| 2 | `manifest.json` 含 clover 引擎包 | 补依赖；`file:` 路径写错会解析失败，用绝对路径兜底 |
| 3 | asmdef references 齐全 | 补 5 个 `CloverEngine.*`，否则 `Game.Net`/`EMsg` 编译不过 |
| 4 | 解决方案文件生成（Unity 6 是 **`.slnx`**；旧版才是 `.sln`） | 用 Unity 打开一次工程，或 `unity` 触发一次编译刷新。⚠️ 判据以 `reference/unity-cli.md` 的「验收五条」为准 |

**四条全过才允许开始写业务 C#。**

## 常见问题

- **工程名/路径搞反**：`--path` 是**父目录**不是工程目录。
  要 `clover-x/client/` 是工程根，就得 `create "client" --path "clover-x"`。
- **新建的 shell 找不到 `unity`**：装完 CLI 必须新开终端（PATH 未刷新）；
  或用完整路径 `$env:LOCALAPPDATA\Unity\bin\unity.exe`。
- **把源码树当工程模板**：只有 `Assets/` 而没有 `Packages/`、`ProjectSettings/` 的目录只是**源码树**，
  不是工程模板，**不要**照着它 mkdir。
- **在 Editor 打开着的时候改磁盘文件**：先让 Editor 保存/关闭再改。
