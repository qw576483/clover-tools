# 掉帧 / 卡顿 排查配方（`experience/perf-triage.md`）

> 触发词：用户说"怎么这么卡""有 1 帧吗""掉帧""跑不动"。
> **一句话铁律：先证伪环境，再查代码。** 环境没排除之前，任何"代码层性能红旗"都是猜。
> 本文件是 2026-09-20 `clover-project-cs16` 的真实翻车复盘 —— **跑满 6 次 Play 才定位到根因，而根因在第一条命令里**。

---

## 0. 反例：当时是怎么绕远的（⛔ 照着别犯）

用户的投诉就是一句"你这游戏怎么这么卡，有 1 帧吗"。当时的路径：

| 次序 | 我做了什么 | 结果 | 代价 |
|---|---|---|---|
| 1 | 派子 agent 扫**代码层性能红旗**（每帧全层射线 / HUD 每帧重建 / 每帧 `Camera.main` / `Animator.HasState` / 字符串分配） | 列了一堆"可疑项"，全部是真的、但都不是主因 | 一次大范围探索 |
| 2 | 进 Play 采 `Time.unscaledDeltaTime` + marker | `PlayerLoop` 300~560ms，而 `Camera.Render` 0.6ms、`BehaviourUpdate` 1.6ms ⇒ **数字自相矛盾**，无法解释 | 1 次 Play |
| 3 | 怀疑"编辑器失焦节流" | Edit 模式 tick P50 **20.7ms**、Play 模式 P50 **637ms** ⇒ 排除节流，确认 Play 真慢 | 1 次 Play |
| 4 | 怀疑日志（`Debug.Log` 抓堆栈 + Console） | `logEnabled=false` 后 45.4→4.9ms（**9×**）⇒ 差点收工去改日志 | 1 次 Play |
| 5 | 但比赛中静音日志后仍 **3.5 fps** | 日志是"地板"不是"天花板"，假设被推翻 | 1 次 Play |
| 6 | `FrameTimingManager` 拆 CPU/GPU | **`gpuFrameTime = 255.66ms`**、`cpuRenderThread = 5.80ms` ⇒ 锁定 GPU | 1 次 Play |
| 7 | **才去看渲染设备** | `SystemInfo.graphicsDeviceName = Microsoft Basic Render Driver` ⇒ **WARP 软件渲染**，根因到手 | 10 秒 |

**教训**：第 7 步如果放在第 0 步，整个排查 1 条命令、10 秒结束。前 6 步全是"在一个连 3D 加速都没有的机器上分析代码性能"。

---

## 1. 正确顺序（4 步，别跳）

### 第 0 步（**最先做，一条命令**）：这台机器现在到底在用哪个渲染设备？

```powershell
# A. Unity 视角（进 Play 后 eval_file，或 Play 中任意探针里打印）
UnityEngine.SystemInfo.graphicsDeviceName      # => 期望 "AMD Radeon RX 5700 XT" / "NVIDIA ..." / "Intel ..."
UnityEngine.SystemInfo.graphicsDeviceType
UnityEngine.SystemInfo.graphicsMemorySize

# B. 系统视角（不依赖 Unity，随时可跑）
Get-CimInstance Win32_VideoController | Select-Object Name,Status,ConfigManagerErrorCode
Get-PnpDevice -Class Display | Select-Object Status,FriendlyName,InstanceId

# C. Unity 自己的启动日志（最硬的证据，编辑器启动时写一次）
Select-String -Path client/Logs/Editor*.log -Pattern 'D3D12 Device Filter|Renderer:'
```

**判红条件（任一条命中 ⇒ 停，先修环境）**：

| 现象 | 含义 |
|---|---|
| `graphicsDeviceName` = `Microsoft Basic Render Driver` | D3D12 只剩 WARP = **纯 CPU 软件光栅化** |
| dxdiag 里 `Card name` = `Microsoft 基本显示适配器` | 同上，显卡没被用上（`Driver Model: WDDM 1.3` 是兜底驱动的特征） |
| `Get-PnpDevice` 里独显 `Status=Error` | 查 `Problem`：`CM_PROB_DISABLED (Code 22)` = 被禁用；`Code 10/43` = 驱动启动失败 |
| 设备 `InstallDate` 之后有 `Kernel-PnP` **Event 411（had a problem starting）** | 驱动装完就没起来过 |

```powershell
# 追"什么时候坏的 / 谁干的"
Get-PnpDevice -InstanceId <id> | Format-List Status,Problem,ProblemDescription
Get-WinEvent -LogName 'Microsoft-Windows-Kernel-PnP/Configuration' -MaxEvents 3000 |
  Where-Object { $_.Message -match '<显卡 ID 片段>' } |
  Select-Object TimeCreated,Id,LevelDisplayName,Message
Get-PnpDeviceProperty -InstanceId <id> -KeyName DEVPKEY_Device_InstallDate,DEVPKEY_Device_LastArrivalDate,DEVPKEY_Device_DriverVersion
```
`ConfigFlags = 1`（`HKLM\SYSTEM\CurrentControlSet\Enum\<instanceId>`）= 禁用标志被置上。
**⛔ 不要据此推断"是用户手动禁的"** —— 驱动安装失败后的回滚、投屏/虚拟显示器软件（如向日葵 `OrayIddDriver`）、AMD `Crash Defender` 服务都会写这个标志。
**⛔ 也不要说"启用了就好"**：`Enable-PnpDevice` 返回 OK、`ConfigFlags` 归零之后，显示适配器在本会话内**常常仍报 Code 22**，必须**重启**重新枚举；重启后若变 `Code 10/43`，那是驱动本身的问题 ⇒ 重装驱动。

### 第 1 步：CPU 还是 GPU？（`FrameTimingManager`，比 marker 靠谱）

```csharp
// 每帧调一次 CaptureFrameTimings，然后取最近若干帧做平均
UnityEngine.FrameTimingManager.CaptureFrameTimings();
var t = new UnityEngine.FrameTiming[1];
var got = UnityEngine.FrameTimingManager.GetLatestTimings(1, t);
// t[0].cpuFrameTime / gpuFrameTime / cpuMainThreadFrameTime /
//      cpuRenderThreadFrameTime / cpuMainThreadPresentWaitTime   （单位都是 ms）
```

| 读数 | 结论 |
|---|---|
| `gpuFrameTime` ≫ `cpuMainThreadFrameTime` | **GPU 瓶颈** ⇒ 查渲染（分辨率 / 阴影 / AA / 面数 / 贴图），或者干脆是软件渲染（回第 0 步） |
| `cpuMainThreadFrameTime` 大、`cpuRenderThreadFrameTime` 小 | 主线程在**等 GPU** 或卡在同步点 |
| 两者都小、帧时间仍很大 | 不在渲染 ⇒ 查脚本 / 物理 / 编辑器开销（第 2 步） |

**本次实测**：`gpuFrameTime 255.66ms` / `cpuRenderThread 5.80ms` / 实际 3.5 fps ⇒ 一眼 GPU。
**⚠️ `ProfilerRecorder` 的 `PlayerLoop` 会给出"总量很大但所有子项都很小"的自相矛盾结果**（本次 558ms vs 子项合计 5ms）—— 遇到这种矛盾，直接上 `FrameTimingManager`，别在 marker 名字上耗。

### 第 2 步：A/B 切场景（定位到"是不是渲染"）

按阶段采样平均帧时间，一次采完：

```
基线 → timeScale=0 → 关所有 Canvas → 关所有 Camera → 关所有 Renderer → 恢复
```

**⚠️ 两条本次踩过的坑**：
1. **在软件渲染下得到的 A/B 结论不可外推到正常机器**（本次"关相机 220→68ms"只在 WARP 下成立）。
2. **A/B 会破坏运行时状态**（关相机可能让流程退出比赛、场景卸载、后续读数全废）—— 要么每阶段之间恢复，要么**采样与恢复严格配对**，要么接受"这一轮只能拿一次数据"。

### 第 3 步：CPU 侧（脚本 / 物理 / 编辑器）

- 编辑器侧干扰项，**先量再改**：`Debug.unityLogger.logEnabled = false` 前后对比。本次实测 **45.4 → 4.9ms**（Console 条目只有 232 条也照样吃 40ms/帧）⇒ 这是"地板"，**不是**主因，⛔ 别急着去删日志。
- 排除"编辑器节流"这个假象：**同一台机器上比 Edit 模式的 `EditorApplication.update` tick 间隔 vs Play 模式的帧间隔**。本次 20.7ms（Edit）vs 637.8ms（Play）⇒ 节流被排除。
- 再看代码红旗（这一步才是子 agent 擅长的那份清单）：每帧 `~0` 全层 `Physics.RaycastNonAlloc`、每帧全量重刷 HUD/字符串插值、每帧 `Camera.main`、每帧 `Animator.HasState`、每帧日志。
  **⛔ 它们的量级通常是"每帧几百微秒~几毫秒"，只有在第 0~2 步都干净之后，才轮到它们**。

---

## 2. 判据表（"卡"这件事怎么算验过）

| 结论 | 必须同时给出的东西 |
|---|---|
| "这是环境问题" | 设备名（`SystemInfo.graphicsDeviceName` 或 dxdiag `Card name`）**+** 设备状态（`Win32_VideoController.Status` / `Problem`）**+** `Kernel-PnP` 时间线 |
| "这是游戏代码问题" | `FrameTimingManager` 的 CPU/GPU 拆分 **+** 关掉嫌疑模块后的 A/B 数字 **+** 渲染设备名（证明不是软件渲染） |
| "已经修好了" | 同一台机器、同一分辨率下的**前后两次帧时间**，且设备名已变成真实 GPU |

**⛔ 禁止**：只给帧率数字不给设备名；只给"我看着卡"；拿 A/B 的下降幅度当结论（却没有 A/B 之外的第二条独立证据）。

---

## 3. 预防（下次怎么不摔）

1. **进 Play 的第一件事是采环境基线**（设备名 + 帧时间 + 分辨率），而不是验证功能。把这三行存进 `.ai-tmp/test/`，后续所有性能数字都带着它。
2. **"卡"这个投诉进来时**，第 0 步的 A/B/C 三条命令先跑完再讨论代码。
3. **交付前自证**：`策划/验收表.md` 加一类 `性能类` 行，判据里**强制包含渲染设备名**。
4. 如果确实要在**没有独显加速的机器**上继续开发（比如远程/虚拟显示器环境），**在交付说明首行写明**"本机为软件渲染，性能数字不代表实机"，避免把环境限制说成"已完成优化"。
