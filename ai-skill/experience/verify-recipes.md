# 自证与取证配方（AI 必须自己跑完这些，再写交付说明）

> 原则：**不许说"应该好了"**。每一条都要有可粘贴的证据（日志/数字/截图/测试结果）。
> 工具用法出处：`reference/unity-cli.md` §8.1。前提：Unity 6 + 工程已在编辑器里打开。

---

## 0. 四连（每次交付前都跑）

```bash
unity status --format json                      # ① 编辑器 ready（不是 Safe Mode）
unity command recompile_status --format json    # ② failed=false
unity command clear_console; unity command editor_play; sleep 45
unity command console --tail 400                # ③ 关键日志都在 & 零异常
unity command eval_file --file <工程>/client/verify-runtime.cs   # ④ 运行时自证
unity command capture_game_view --source screen --save_path Screenshots/play.png
unity command editor_stop                       # ⑤ 收工
```

## 1. 运行时自证脚本（`verify-*.cs` 放 `Assets/` **外**，Unity 不会编译它）

**模板 A：对象状态**（组件是否挂上/模型是否加载/动画是否在跑）

```csharp
var m = UnityEngine.Object.FindFirstObjectByType<CloverMmo1.Module.Player.PlayerMotor>();
var c = UnityEngine.Object.FindFirstObjectByType<CloverMmo1.Module.CameraRig.ThirdPersonCamera>();
var hud = UnityEngine.Object.FindFirstObjectByType<CloverMmo1.UI.HudPanel>();
var model = (m != null) ? m.transform.Find("Model") : null;
var an = (model != null) ? model.GetComponent<Animator>() : null;
string clip = "none"; float t = -1f;
if (an != null) { var ci = an.GetCurrentAnimatorClipInfo(0); if (ci.Length > 0) clip = ci[0].clip.name; t = an.GetCurrentAnimatorStateInfo(0).normalizedTime; }
float d = (m != null && c != null) ? UnityEngine.Vector3.Distance(c.transform.position, m.transform.position) : -1f;
UnityEngine.Debug.Log("[Verify] motor=" + (m != null) + " camTarget=" + (c != null && c.Target != null)
    + " camDist=" + d.ToString("F2") + " hud=" + (hud != null) + " model=" + (model != null)
    + " clip=" + clip + " clipTime=" + t.ToString("F2"));
return "ok";
```

**模板 B：本地碰撞**（与服务端同规则？防穿墙？）

```csharp
// 地图：数据与空间事实在**引擎**（Game.Map，读服务端同一份字节）；
// 解算（半径采样 / 分轴滑墙 / 扫掠细分）在**业务模块**（这里是 MapModule，薄门面无状态）。
var map = new CloverMmo1.Module.Map.MapModule();
bool ok = false; map.Load(b => ok = b);      // 已加载/资源缓存命中时会同步回调

float x = 6.5f, z = 10.5f; int blocked = 0;
for (int i = 0; i < 100; i++) {
    var r = map.Resolve(new UnityEngine.Vector3(x,0,z), new UnityEngine.Vector3(x+0.1f,0,z), 0.35f);
    if (r.x <= x + 0.0001f) blocked++;
    x = r.x;
}
var big = map.Resolve(new UnityEngine.Vector3(6.5f,0,10.5f), new UnityEngine.Vector3(14f,0,10.5f), 0.35f);
UnityEngine.Debug.Log("[VerifyMove] loaded=" + ok + " 小步终点 x=" + x.ToString("F2") + " 被挡=" + blocked + "；一次 7.5m 位移结果=" + big.x.ToString("F2"));
return "ok";
```

**模板 C：服务端校正链路**（故意发一条非法移动，看是否收到校正并回放）

```csharp
CloverEngine.Game.Net.Send(CloverMmo1.Def.MsgDef.Move,
    new CloverMmo1.Def.MoveRequest { x = 10.5f, y = 0f, z = 10.5f, seq = 8888 });
UnityEngine.Debug.Log("[VerifyNet] 已发送非法移动（走进建筑）seq=8888");
return "ok";
// 等 2~3 秒后再跑模板 A：应看到 CorrectionCount>0，客户端日志有「应用服务端校正 ack=8888 …」
```

**模板 D：相机遮挡**（把角色挪到障碍旁、相机朝墙侧，读距离）

```csharp
var m = UnityEngine.Object.FindFirstObjectByType<CloverMmo1.Module.Player.PlayerMotor>();
var c = UnityEngine.Object.FindFirstObjectByType<CloverMmo1.Module.CameraRig.ThirdPersonCamera>();
m.SendEnabled = false;                                   // 测试期间别真发移动
m.Teleport(new UnityEngine.Vector3(7.5f, 0f, 10.5f));    // 障碍旁边
c.Yaw = 270f;                                            // 让相机朝向"墙那一侧"
UnityEngine.Debug.Log("[VerifyCam] 已就位，等一帧读距离");
return "ok";
// 2 秒后再跑一次只读距离：被挡应远小于 5.5（实测 1.47）
```

## 2. 服务端探针（把不可见的逻辑变成可断言的事实）

- 加一条**探针消息**（本项目 `MsgGridProbe`）：给点返回「可走性 + 命中的碰撞体 id + 地图是否加载」；
- 用途：验地图管线、验建筑真的挡路、验客户端与服务端判定一致；
- 断言两个方向：**道路=true 且不在碰撞体**、**建筑=false 且在碰撞体**（只断言一边会被"全阻挡/全可走"蒙过）。

## 3. 截图取证（含 HUD）

```bash
# 必须在 Play 模式；save_path 必须在工程目录内（相对"作者根 Assets/"）
unity command capture_game_view --source screen --width 1600 --height 900 --save_path Screenshots/play.png
```

**客观校验（不要"看着像"）**：PNG 体积 + 颜色多样性（空白画面压不出体积、颜色数极少）。

```powershell
Add-Type -AssemblyName System.Drawing; $bmp=[System.Drawing.Bitmap]::FromFile($png)
$colors=@{}; for($y=0;$y -lt $bmp.Height;$y+=8){for($x=0;$x -lt $bmp.Width;$x+=8){$c=$bmp.GetPixel($x,$y);$colors["$($c.R),$($c.G),$($c.B)"]=1}}
"不同颜色数=$($colors.Count)"; $bmp.Dispose()
# 判据：>200 种 = 有真实贴图/HUD；个位数 = 基本空白
```

## 4. 读日志（console 输出是一行 TSV，必须先落盘再正则）

```powershell
unity command console --tail 300 --no-pager --no-banner 2>&1 | Out-File -Encoding utf8 console.txt
$t=[IO.File]::ReadAllText('console.txt',[Text.Encoding]::UTF8)
[regex]::Matches($t,'\[(Boot|Motor|Cam|HUD|Combat)\][^"\\]{0,180}') | ForEach-Object { $_.Value } | Select-Object -Last 20
```

## 5. 回归测试（既有套件必须跑过）

```bash
unity command run_tests --mode PlayMode --filter <命名空间>.<用例类> --timeout 300 --async_tests --format json
# 轮询
unity command test_status --format json      # 看 summary.passed / failed
```
> 引擎改动后也要跑：`go build ./... && go vet ./... && go test ./internal/... -run <复现用例> -v -timeout 90s`

## 6. 交付说明必须包含的证据清单

| 证据 | 具体形式 |
|---|---|
| 编译 | `recompile_status` → `failed=false`（客户端）/ `go build` 退出码 0（服务端） |
| 运行 | 关键日志逐条贴出（进图/装配/动画/碰撞/校正…）+ **零异常** |
| 自证 | 模板 A/B/C/D 的输出（数字） |
| 画面 | 截图路径 + 颜色多样性数字 |
| 回归 | PlayMode 用例结果（passed/failed）+ 引擎用例结果 |
| 引擎改动 | 若有：E 编号 + 改动文件 + 回归方式 |
| 未完成 | 诚实列出（不许把没做的说成"已预留"） |
