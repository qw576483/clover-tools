# offline-hosts —— 离线自检宿主 harness（模板 + 脚手架 + 一键回归）

把「**Unity 托管 DLL + 项目源码直编到 .NET、秒级跑真断言、不启编辑器**」这套基建做成通用件。

- 出处：`clover-project-diablo2/tools/probes/hosts/`（13 个宿主 + `run_all_hosts.ps1` + `Directory.Build.props` + `shim/`）。
- 上浮时改掉的两处**项目耦合**：
  1. **不再写死相对深度**。源工程的 13 个 csproj 把 `..\..\..\..\client\...` 与 Unity 版本写成字面量（54 行 `HintPath`）⇒
     换机器 / 挪目录 / 换编辑器小版本，13 个宿主一起失效。本 harness 改为**有界向上查找**（0~8 层）+ 显式覆盖。
  2. **一键入口不再手工维护宿主名单**。源工程的 `run_all_hosts.ps1` 曾硬编码 12 个名字、而盘上有 14 个宿主 ⇒
     两个宿主**从来没被一键入口跑过**。本 harness 按 `*.csproj` 自动发现。

## 文件

| 文件 | 作用 |
|---|---|
| `New-OfflineHost.ps1` | 脚手架：造一个宿主目录（`props` + `csproj` + `Program.cs`），并打印解析出的项目/引擎/编辑器路径 |
| `Invoke-OfflineHosts.ps1` | 一键回归：自动发现 `*.csproj` → 逐个子进程 `dotnet run` → 汇总 `TOTAL_HOSTS / FAILED` |
| `clover-paths.ps1` | 共享路径解析：`Find-CloverProjectRoot` / `Find-CloverUnityDir` / `Find-CloverEngineRuntime` / `Resolve-UnityEditorRoot` |
| `templates/Directory.Build.props` | 宿主根共用：向上解析 Unity 工程目录、引擎 Runtime 目录、编辑器目录；**无固定相对深度** |
| `templates/Host.csproj.template` | 宿主 csproj 模板（白名单式 `Compile` + 托管 DLL `Reference`） |
| `templates/Program.cs.template` | 断言宿主程序模板（`[ OK ]` / `[FAIL]` + `CHECK ok= fail=` + 退出码） |
| `templates/shim/README.md` | **引擎门面替身**怎么写（三条硬规则 + 复用口径） |

## 用法

```powershell
# 1) 造一个宿主（Name 建议 <模块>Check）
powershell -NoProfile -ExecutionPolicy Bypass -File New-OfflineHost.ps1 `
    -Name CoreCheck -HostsRoot <宿主根目录> -ProjectRoot <项目根>

# 2) 改 <宿主根>\CoreCheck\CoreCheck.csproj：每个源码族加一行
#      <Compile Include="$(CloverAssetsDir)\Scripts\Core\*.cs" />
#    改 <宿主根>\CoreCheck\Program.cs：写断言 Check(name, condition, detail)

# 3) 跑全部宿主（含新造的）
powershell -NoProfile -ExecutionPolicy Bypass -File Invoke-OfflineHosts.ps1 -HostsRoot <宿主根目录>
```

| 参数（`New-OfflineHost.ps1`） | 默认 | 说明 |
|---|---|---|
| `-Name` | 必填 | 宿主名，字母开头，`[A-Za-z][A-Za-z0-9_.]*` |
| `-HostsRoot` | `<脚本目录>\hosts` | 宿主根目录；`Directory.Build.props` 放这一层（MSBuild 自动向上继承） |
| `-ProjectRoot` | 自动 | 从 `-HostsRoot` 向上找 `client\Assets` / `Assets` |
| `-EngineRoot` | 自动 | 向上找 `<引擎目录名>\Runtime` 或 `Packages\<引擎目录名>\Runtime` |
| `-TargetFramework` | `net10.0` | 写进 csproj 的 `TargetFramework` |
| `-UnityEditorRoot` | 自动 | 依次：参数 → `UNITY_EDITOR_ROOT` → Hub 下**版本名最高且真的有托管 DLL**的那一个 |
| `-Force` | 关 | 覆盖已存在的 `csproj` / `Program.cs` |

| 参数（`Invoke-OfflineHosts.ps1`） | 默认 | 说明 |
|---|---|---|
| `-HostsRoot` | 必填 | 递归找 `*.csproj`（排除 `bin\` / `obj\`） |
| `-Filter` | 空 | 按文件名/基名过滤，如 `*check.csproj` |
| `-TimeoutSec` | `300` | 单宿主超时（超时记为 `TIMEOUT` 并计入 FAILED） |
| `-UnityEditorRoot` | 自动 | 同 `New-OfflineHost.ps1`；解析到就以 `-p:UnityEditorRoot=` 传给每个宿主 |
| `-Quiet` | 关 | 只打印每行的状态，不打尾部输出行 |

退出码：`0` = 全过、`1` = 至少一个宿主失败、`2` = 参数/发现失败（宿主根不存在、一个 `csproj` 都没找到）。

## 判据形态（一条命令 + 关键原始输出）

```text
===== offline self-check hosts =====
hosts-root   = <...>
discovered   = 14 host(s)
unity-editor = C:\Program Files\Unity\Hub\Editor\6000.6.0f1\Editor
timeout      = 300 s per host
[1/14] CoreCheck            exit=0  PASS  (6.2 s)
         | CHECK ok=12 fail=0
         | ALL PASS
[2/14] ItemCheck            exit=1  FAIL  (4.8 s)
         | [FAIL] MoveItem: expected 3 got 2
         | CHECK ok=7 fail=1
TOTAL_HOSTS=14 FAILED=1
failed: ItemCheck
```

判定读法（硬性）：

- 判据 = **`TOTAL_HOSTS=N FAILED=0`**（退出码 0）。`FAILED>0` ⇒ 逐个看该宿主的 `[FAIL]` 行。
- ⛔ `dotnet run` **成功**只代表宿主自身没崩；宿主**没有**覆盖到某个源码文件时它照样绿
  （源工程实测：白名单漏了一个文件 ⇒ 断言只能从源码文本里抠数）。
  所以裁源码范围时要核对 csproj 的 `Compile` 清单，⛔ 别把"宿主绿了"当成"这部分被验过了"。
- ⛔ 宿主只证明**类型 / 纯逻辑 / 文本契约**。表现类（画面 / 动画 / 时序观感）必须另行进 Play 取证。

## 路径解析口径

优先级（第一个有值的生效）：

1. 命令行 `-p:CloverUnityDir=<dir>` / `-p:CloverEngineRuntimeDir=<dir>` / `-p:UnityEditorRoot=<dir>`
2. 环境变量 `CLOVER_UNITY_DIR` / `CLOVER_ENGINE_ROOT` / `UNITY_EDITOR_ROOT`（Hub 根可另给 `UNITY_HUB_ROOT`）
3. **有界向上查找**：从 `Directory.Build.props` 所在目录起 `_Cd0.._Cd8`（0~8 层），
   找 `client\Assets`（或 `Assets`）、找 `<引擎目录名>\Runtime`（或 `Packages\<引擎目录名>\Runtime`）
4. Unity 编辑器：Hub 下**版本名降序**第一个真的含
   `Data\Managed\UnityEngine\UnityEngine.CoreModule.dll` 的版本目录

找不到时**不报 MSBuild 的解析墙**，而是给一句人话 + 列出 Hub 下已装的版本（见 `props` 里 `CloverCheckPaths` 目标）。

## 已知边界

- 宿主只能离线跑；**进 Play 的端到端**不在这里（那是 `unity-cli` + 驱动脚本的事）。
- `dotnet run` 是**增量**的：无改动时可能什么都不打印。判据看**退出码 + `CHECK` 行**，⛔ 不要看"有没有输出"。
- 跨仓库（引擎 + 项目）改动时，若宿主引用的是 `Library\ScriptAssemblies\*.dll` 这类**编辑器产物**，
  必须先在编辑器里重编，否则**假通过**（拿旧 DLL 编过已被删掉的 API）。参考 `clover-ai-skill/reference/fast-compile-loop.md` 坑 4/6。
