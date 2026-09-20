# 原生插件（P/Invoke）互操作：崩溃定位法 + msquic 实测坑

> 适用场景：Unity 里自己绑一个原生库（msquic / WebRTC / 三方 SDK）。
> **结论先写**：这类代码的失败形式是"**进程直接消失**"而不是异常 —— 必须换一套排查方法，
> 拿"读日志 + 看数字"那套去查只会空转（本项目白烧了 3 次编辑器崩溃）。

## 1. 心智模型：为什么必须换一套

| | 托管代码的 bug | 原生互操作的 bug |
|---|---|---|
| 表现 | 异常 / 报错日志 | **进程消失**（编辑器被强杀、真机闪退） |
| 有无托管堆栈 | 有 | **没有** |
| 复现成本 | 秒级 | 起编辑器 + 进 Play（分钟级） |
| 有效手段 | 读日志 | **控制台试验台 + 步骤标记 + 退出码** |

⇒ 第一原则：**让绑定能脱离 Unity 独立跑**（见 §2）。做不到这一点，"试一次 3 分钟"会把排查拖死。

## 2. 手法：控制台试验台

（本项目落地在 `clover-client-unity-engine/Tools~/quic-harness/`，可直接照搬结构。）

1. **直接链接真实源码**：csproj 用 `<Compile Include="../../Runtime/Network/Quic/*.cs" />`
   链接引擎里的真实文件，只补**最小的 Unity 替身**（`Game.Logger` / 枚举 / 契约接口 / 大端读写）。   
   ⇒ 验证过的就是 Unity 里跑的那份，**不存在"两份代码漂移"**；顺带把绑定约束成"不依赖 UnityEngine"。
2. **逐步 + 每步先打标记**：崩溃没有堆栈，**最后打印出来的标记 = 崩在下一步**。
3. **日志每条 `Flush()`**：进程被原生断言打死时 **stdout 缓冲会整体丢失**，
   不 flush 就会被"假的最后一行"骗（本项目第一次就是这么被骗的）。
4. **模式二分**：把可疑维度抽成 `plain / send / login / dgram / multi`，各跑一遍 ⇒ 判别项立刻显现。
   本项目就是靠 `plain`（零收发）**永不崩**、其余（收过数据）**全崩**，   
   才推出"触发条件是**收到过流数据**"。
5. **诊断开关做成环境变量**（`CLOVER_QUIC_NOFREE` 故意不释放缓冲 / `CLOVER_QUIC_SKIP_SHUTDOWN` 故意跳过某步 /
   `CLOVER_QUIC_TRACE=1` 逐调用跟踪）：用"故意不这么做"来**反证因果**，比读源码猜快得多。
6. **退出码就是证据**：崩溃时是 `0xC0000420` 这类 NTSTATUS，正常是 `0` —— 一眼判定，还能进 CI。

## 2.5 ★ IL2CPP 下的四条硬约束（编辑器对、出包错 —— 真实事故）

> **共同点：这四条在编辑器（Mono）里全部正常，只有出 IL2CPP 包才现形。**所以"编辑器里跑通了"对原生互操作**不构成验收**：至少要有一次 IL2CPP 包的实测。

**① 交给原生的回调必须是静态方法 + `[MonoPInvokeCallback]`**
- 症状：`IL2CPP does not support marshaling delegates that point to instance methods to native code`
  （在**连接时才抛**，日志里只看到"连接失败"）。
- 做法：`[MonoPInvokeCallback(typeof(NativeXxxCallback))] static uint OnXxxStatic(IntPtr h, IntPtr context, ...)`，
  在 `context` 里传 `GCHandle.ToIntPtr(GCHandle.Alloc(this))`，静态回调里 `GCHandle.FromIntPtr(context).Target as T` 反查实例；  句柄关闭后 `Free()`（解析失败要静默返回，不能让异常穿过原生边界）。
- 只支持 **静态**方法——实例方法委托在 Mono 下能用，IL2CPP 下直接失败。

**② 能力判定必须"主动探测"，不能读"探测结果缓存"**
- 症状：**真机/新进程里那条线路被静默排除**，规划里只剩降级线路，而且**一条相关日志都没有**。
- 根因：`Capabilities.IsXxxUsable => Runtime.IsAvailable && ...`，而 `IsAvailable` 是"探测结果缓存"，
  **未探测时恒 false** ⇒ 只有"同进程内先有人探测过"（比如编辑器里跑过测试）才会启用。
- 做法：能力判定里调用**主动探测**入口（`TryEnsureReady(out reason)`，内部加锁 + 缓存结果只探一次），
  并把排除原因打一次日志（**能力缺失要可见，不静默降级**）。回归用例：断言"能力判定结果 == 显式探测结果"。

**③ 运行期配置文件必须真的进包**
- 症状：真机包用**代码默认值**跑（例如 `udp_addr` 为空 ⇒ 那条线路永不启用），**没有任何报错**（默认值都合法）。
- 根因：配置读取写成 `Path.Combine(Application.dataPath, "Configs/xxx.json")`：
  编辑器里 `dataPath` = `<工程>/Assets`（读得到），**玩家包里 = `<exe>_Data`**（读不到）。
- 做法（二选一）：① 放 `StreamingAssets`（跨平台标准位置，Android 内需 `UnityWebRequest`）；
  ② 出包后把配置**拷进** `<exe>_Data/Configs/`（本项目 `PlayerBuilder.CopyRuntimeConfigs` 就这么做，  好处是**只保留一份配置文件**、不产生两份漂移）。**验收**：包里必须能 `line plan` 出目标线路，而不是只剩默认线路。

**④ 出包前必须关掉在跑的 Player**
- 症状：IL2CPP 构建报 `Building .../global-metadata.dat failed`（还会夹带一堆无关字段告警，容易被误导）。
- 根因：正在运行的 Player **映射着 `global-metadata.dat`**，构建写不进去。
- 做法：构建脚本/流程里先杀 Player 进程再出包。

## 3. msquic 实测坑（按危害排序，全部真踩过）

1. **`StreamReceiveComplete` 只能用于"挂起式（PEND）"收包**。
   同步消费（回调里把数据拷进托管内存后返回 `SUCCESS`）就**不要再调它**；   
   调了会把 msquic 的**收包记账写坏**，代价不是立刻报错，而是**之后关闭连接时原生断言崩溃**   
   （`Faulting module: msquic.dll`，`0xC0000420` = STATUS_ASSERTION_FAILURE）。   
   **迷惑点**：只连不发的用例永远不崩，一发过/收过数据的就崩 ⇒ 别按"关闭流程"去查，先查收包。
2. **回调内先还缓冲、再解析**：顺序必须是「拷贝 → 交还缓冲 → 解析」。
   反了就是"解析判定异常 → 关流句柄 → 再对已关闭句柄调 API" = use-after-free。
3. **缓冲生命周期**：`StreamSend` / `DatagramSend` 的**数据字节**必须活到
   `SEND_COMPLETE` / 发送状态进入 `FINAL`（描述符数组可以自己分配，数据缓冲不行）。
4. **托管宿主会卸载**：Unity 退出 Play / 重编译会**卸载脚本域**，此时还活着的连接会让 msquic
   回调进**已卸载的托管代码** ⇒ 崩溃。必须三件套：   
   ① 活连接**登记在册**；② `AssemblyReloadEvents.beforeAssemblyReload`（Editor）+ `Application.quitting`    
   里**同步**关掉全部连接（`ConnectionClose` 之后 msquic 保证不再回调）；   
   ③ **测试的 `TearDown` 必须兜底** —— 断言失败时方法体末尾的 `Disconnect()` 根本不会执行。
5. **测试里"失败"本身就是风险**：E2E 用例一旦断言失败抛异常，清理被跳过 → 活连接留到域卸载 → 崩溃。
   ⇒ 真连外部服务的用例**默认跳过**（环境变量开启，例如 `CLOVER_QUIC_E2E=1`），并让 `TearDown` 无条件兜底。

## 4. "编辑器起不来"的先看日志（**处方见 `unity-cli.md` §9.2，别自己造**）

**先看它自己的日志**，按这个顺序：

1. `%LOCALAPPDATA%\Unity\Editor\Editor.log` 与 `Editor-prev.log`（**每次启动轮转**，prev 是上一次）；
2. **工程内的服务日志**：`<project>/Logs/Editor-UDS.log`、`Logs/UDS-Service.log`
   （Unity 6 的 **Unity Data Store**；`Failed to open database: 5` 的 `5` 是错误码，不是行号）——   
   **一手原因就在这两个文件里**，比编辑器自己的日志更靠前；
3. `%LOCALAPPDATA%\Temp\Unity\Editor\Crashes\Crash_*\`（原生崩溃转储，含 `Editor.log` 副本）；
4. **Windows 事件日志 `Application Error`** —— **故障模块名 + 异常码**是最硬的一手证据：
   `msquic.dll` + `0xC0000420`（断言）/ `0xC0000005`（访问违例）/ `KERNELBASE.dll` + `0x40000015`（abort）。

**已知诱因与处方（`unity-cli.md` §9.2，按代价从低到高）**：

| 诱因 | 识别特征 | 处方 |
|---|---|---|
| **中途强杀 Editor**（超时/`Stop-Process`；**原生崩溃也算强杀**） | `UDSInterface::UDSInterface` → `InitializeAssetDatabaseV2` 栈；`Failed to open database` | **把 `Library` 整体挪走**（改名即可）让它重建 |
| **安全软件（360 等）拦截** | `Failed to open database: 5`（`5` 是错误码）+ `信号量已存在` + `Failed to connect to existing shared system data` | 加信任区 / 临时关主动防御 / 用 **Hub GUI 手工打开一次**（GUI 路径对 UDS 不敏感） |
| 工程或日志放在**点开头**的目录 | `.xxx is not a valid directory name` | 挪到正常路径（且**机器级共享状态已被弄坏，挪回去不等于恢复**） |

> 本轮实测补充（**别重复我的弯路**）：只挪 `Library/DataStore` **不足以**恢复（我挪了两遍都没用）；
> 真正做到的是「**清干净所有 Unity 进程 + 按 §9.2 处置 + 重开**」。
> 我一度判断"只能重启系统"——**结论过强，已被证伪**，`§9.2` 的三条才是正解。
