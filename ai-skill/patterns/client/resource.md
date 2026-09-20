# 资源加载模板

> `IResourceManager` 的**加载/释放** API 是 6 个：`LoadAsset<T>(path, cb)`、
> `LoadAsset<T>(path, progress, cb)`、`Release(path)`、`UnloadAll()`、`Preload(paths, onDone, progress)`、
> `TryGet<T>(path)`（**同步**取已驻留资源；不触发加载、不阻塞、纯读——要"立刻拿到"就先 `Preload` 再 `TryGet`）。
> **热更**另有 5 个：`CheckUpdate` / `DownloadUpdate` / `ClearDownloaded` / `Version` / `UpdateState` 
> （见模板 6）。加载/释放全部是**回调式**（不是 await）。
> **没有** `LoadAsync` / `LoadSync` / `Unload` / `UnloadUnused` / `LoadSceneAsync` 等。

## 模板 1：加载资源（回调式）

```typescript C#
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class ResourceLoader : MonoBehaviour
{
    [SerializeField] private Image _icon;

    public void LoadPrefab()
    {
        Game.Res.LoadAsset<GameObject>("Prefabs/Enemy", prefab =>
        {
            if (prefab == null) return;
            var obj = Instantiate(prefab);      // 战斗/列表路径请改用 Game.Pool.Spawn
            Game.Logger?.Info("Res", $"加载成功: {prefab.name}");
        });
    }

    public void LoadSprite()
    {
        Game.Res.LoadAsset<Sprite>("Icons/Sword", sprite =>
        {
            if (sprite != null) _icon.sprite = sprite;
        });
    }
}
```

## 模板 2：带进度加载 / 批量预加载

```typescript C#
using System.Collections.Generic;
using CloverEngine;
using UnityEngine;

public class PreloadManager : MonoBehaviour
{
    public void LoadWithProgress()
    {
        Game.Res.LoadAsset<GameObject>("Prefabs/Boss",
            p => Game.Logger?.Info("Res", $"加载进度 {p:P0}"),
            prefab => Game.Logger?.Info("Res", prefab != null ? "完成" : "失败"));
    }

    public void PreloadBattleAssets()
    {
        var paths = new List<string> { "Prefabs/Enemy", "Prefabs/Bullet", "Prefabs/Effect" };
        Game.Res.Preload(paths,
            onDone: () => Game.Logger?.Info("Res", "全部预加载完成"),
            progress: p => Game.Logger?.Info("Res", $"{p:P0}"));
    }
}
```

> 命中缓存时回调会**立即（同帧）**触发，但签名仍是回调式；没有同步返回值的版本。

## 模板 3：加载配置表

```typescript C#
using CloverEngine;
using UnityEngine;

public class ConfigLoader : MonoBehaviour
{
    public void LoadItemConfig()
    {
        var config = Game.Table.Get<ItemConfig>(1001);
        if (config != null)
        {
            Game.Logger?.Info("Res", $"物品名称: {config.Name}");
            Game.Logger?.Info("Res", $"物品描述: {config.Description}");
        }
    }

    public void LoadAllConfigs()
    {
        foreach (var item in Game.Table.GetAll<ItemConfig>())
        {
            Game.Logger?.Info("Res", $"物品: {item.Id} - {item.Name}");
        }
    }
}
```

## 模板 4：资源释放

```typescript C#
using CloverEngine;
using UnityEngine;

public class ResourceUnloadManager : MonoBehaviour
{
    // 减少某个资源的引用计数（只降计数、不立即卸载；缓存淘汰由内存水位 + LRU 决定）；参数须与加载时一致
    public void ReleaseOne(string path) => Game.Res.Release(path);

    // 一次性卸载全部缓存资源（切场景 / 回登录时用）
    public void ReleaseAll() => Game.Res.UnloadAll();
}
```

> **没有** `UnloadUnused()`（按未使用程度自动回收）这个能力；需要精细控制就用 `Release` 的引用计数。

## 模板 5：场景加载带进度（用 `Game.Scene`，不是 `Game.Res`）

```typescript C#
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class LoadingManager : MonoBehaviour
{
    [SerializeField] private Slider _progressBar;
    [SerializeField] private TextMeshProUGUI _progressText;

    public void LoadScene(string sceneName)
    {
        _progressBar.value = 0f;

        // 参数 1 = 进度(0~1)，参数 2 = 完成回调
        Game.Scene.Load(sceneName,
            progress =>
            {
                _progressBar.value = progress;
                _progressText.text = $"{progress * 100:F0}%";
            },
            () => _progressText.text = "100%");
    }
}
```

> 场景加载自带门控（进度到 0.9 才真正激活场景），业务**禁止**直调 Unity 的 `SceneManager.LoadScene`。

## 模板 6：资源热更

引擎**已内置**热更链路（AssetBundle 后端 + 清单比对 + 断点续传下载）。三步：

```typescript C#
using System.Collections.Generic;
using CloverEngine;
using UnityEngine;

public class HotUpdateFlow : MonoBehaviour
{
    // ① 启动前先把资源模块挂起来（Game.Launch 之后）
    //    CloverRes.Init(new ResourceModuleConfig { ManifestUrl = "https://cdn/.../manifest.json" });
    //    不配 ManifestUrl = 不启用热更，走 Unity 内置 Resources（老行为）

    // ② 检查差异（只比对，不下载）
    public void Check()
    {
        Game.Res.CheckUpdate(info =>
        {
            if (!info.Success) { Game.Logger?.Error("Res", info.Error); return; }
            if (!info.HasUpdate)
            {
                Game.Logger?.Info("Res", $"已是最新：{info.LocalVersion}");
                return;
            }

            Game.Logger?.Info("Res",
                $"{info.LocalVersion} -> {info.Remote.Version}，需下载 {info.FilesToDownload.Count} 个文件 / {info.TotalBytes / 1024}KB" +
                (info.Force ? "（强制更新）" : string.Empty));

            // ③ 下载（断点续传，进度回调会被高频调用）
            Game.Res.DownloadUpdate(info,
                p => Game.UI?.ShowLoading($"更新中 {p.Progress:P0} {p.BytesPerSecond / 1024f:F0}KB/s"),
                (ok, error) =>
                {
                    Game.UI?.HideLoading();
                    if (!ok) { Game.UI?.Toast("更新失败：" + error); return; }

                    // 首次安装：下载完成后引擎**自动**切到 AssetBundle，立即可用；
                    // 版本升级：新内容在**下次启动**生效（引导玩家重启即可，不做运行中热切换）
                    Game.UI?.Toast("更新完成");
                });
        });
    }
}
```

**约定与边界**（写业务前务必知道）：

- **配置热更**同一条链路：配表这类裸文件也进清单（`raw: true`），下载后落在
  `Game.Res.ContentDir` 下，把 `CloverData.InitDataTable(dir)` 指向它就完成了配表热更。
- **不做运行中热切换**：正在被使用的资源在脚下被替换会引出难查的空引用。
  首次安装（本地还没有任何内容）会在下载完成后自动从 Resources 兜底切到 AssetBundle；  
  版本升级一律下次启动生效。
- **只下载不落地清单**：下载全部校验通过后引擎才写本地 `manifest.json` 与版本号，
  所以中途失败不会留下「以为已经更新了」的坏状态。
- **断点续传**：半成品是 `*.part`，`Range` 请求；服务端不支持 Range 时自动整包重下。
- **失败重试**由引擎按 `ResourceModuleConfig.MaxRetries` 自动进行，业务不用自己写重试。
- 硬修复（怀疑本地包损坏）：`Game.Res.ClearDownloaded()` 后重新走一遍检查与下载。

---

## 资源欠缺清单（client 根目录，强制交付物）

**固定路径：`client/资源欠缺清单.md`**（client 工程根目录，全工程唯一，不许放别处、不许只写在聊天里）。

**什么时候写**：只要这次实现**引用了**外部资源（Sprite/图片、音效 BGM、动画 Clip、模型、字体、
美术预制体…），收尾就必须新建或更新它。**占位资源同样要登记**（当前占位 = 纯色块 / 空实现 / 内置几何体）。
纯逻辑、零外部资源的功能不用建。

**更新规则**：文件已存在时**追加/更新行**，不要覆盖别人已替换完成的条目；替换完成的把状态改 ✅。

**文档模板**（新建时照抄，每行都要能"照着做"：给目标路径 + 尺寸 + 影响的代码位置）：

```markdown
# 资源欠缺清单

> 本文件由 AI 维护。AI 不制作美术/音效/动画/模型，全部登记于此，替换后把状态改成 ✅。
> **最低可玩**：不替换任何资源，demo 也能跑通。
> 更新时间：{YYYY-MM-DD}

| # | 资源名 | 类型 | 尺寸/规格要求 | 当前占位 | 需提供 | 替换方法（目标路径） | 影响范围（文件/接口） | 状态 |
|---|---|---|---|---|---|---|---|---|
| 1 | 出拳卡面 | 图 | 64x64 像素 | placeholder_64x64.png（纯色块） | 3 张 PNG | 替换 `Assets/Resources/Placeholder/placeholder_64x64.png` | `UIRpsPanel.cs` 加载路径 | ⬜ 待替换 |
| 2 | 胜负音效 | 音 | 无 | 空实现（打 `[SOUND]` 日志） | win.wav / lose.wav | 放入 `Assets/Resources/Sound/SFX/` 后 `Game.Sound.PlaySFX("win")` | `SfxService.cs` | ⬜ 待替换 |
| 3 | 牌型动画 | 动画 | 无 | 打日志 + Text | 2 个 AnimatorClip | 挂 Animator 并替换 clip | `CardView.cs` | ⬜ 待替换 |
| 4 | 角色模型 | 3D | 1x1x1 单位 | Unity 内置 Cube | FBX/OBJ | 替换 `Assets/Resources/Models/` 下文件 | `PlayerView.cs` | ⬜ 待替换 |
| 5 | 联服验证 | 环境 | - | 未验证（本地无服务器环境） | 装好 windows-env 后跑一遍 | 见 `reference/server-env.md` | 全流程 | ⬜ 待验证 |

## 说明
1. **尺寸要求**：uGUI 会拉伸占位图，但真实美术请按上表尺寸给，避免比例失调/清晰度不一致；
   Sliced/Tiled UI、游戏内 Sprite、图标头像等对尺寸敏感。
2. **推荐替换顺序**：先主视觉（卡面/角色）→ 再音效 → 动画 → 背景。
3. **替换不碰逻辑**：所有引用收敛在资源常量表，换文件或改一行即可，不用重写 handler。
4. **状态图例**：⬜ 待替换 · ✅ 已完成 · ⏸ 暂不需要。
```
