# UI 开发模板

> API 以 [`clover-client-unity-engine/Runtime/**`](https://github.com/qw576483/clover-client-unity-engine/blob/main/Runtime/**.md) 为准。表现域模块随 `Game.Launch` **自动挂载**，
> 业务无需初始化即可直接用 `Game.UI` / `Game.Scene` / `Game.Sound` 等
> （**例外：`Game.Res` 需业务显式 `CloverRes.Init(root)`**——不挂则 `Game.Res` 为 null、资源静默加载失败）。
> 面板预制体放在 **`Resources/UI/{面板类名}`**（`Game.UI.Open<T>()` 按类名加载）。

## UI 布局规范

**默认分辨率适配**：1920x1080（横屏），UI 布局以最小可用为原则。

**占位 UI 规则**：
- 使用纯色块（灰色/红色）作为占位图，不依赖外部资源
- UI 布局必须完整可交互，不能只是空壳
- 所有 UI 元素必须有明确的尺寸和位置
- 占位图片尺寸：64x64 像素（默认），如用户指定其他尺寸则按用户要求

## 模板 1：基础面板（继承 `UIPanel`，只需 override 关心的）

`UIPanel` 基类已经把 `PanelName`（默认取类名）/ `Layer`（默认 Normal）/ `Root` / `OnClose` / `OnUpdate` 
全部实现好，业务通常只写 `OnOpen`：

```typescript C#
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class LoginPanel : UIPanel
{
    [SerializeField] private InputField _accountInput;
    [SerializeField] private InputField _passwordInput;
    [SerializeField] private Button _loginButton;

    public override void OnOpen(object param)
    {
        _loginButton.onClick.RemoveAllListeners();
        _loginButton.onClick.AddListener(OnLoginClick);
    }

    private async void OnLoginClick()
    {
        var account = _accountInput.text;
        var password = _passwordInput.text;
        if (string.IsNullOrEmpty(account) || string.IsNullOrEmpty(password))
        {
            Game.Logger?.Warn("UI", "请输入账号和密码");
            return;
        }

        try
        {
            // 账号密码只发给账号服（HTTP）换 token；ELoginRequest 只有 token 字段。
            var token = await CloverAuth.LoginAsync(account, password);

            var reply = await Game.Net.Call<ELoginReply>(EMsg.Login, new ELoginRequest
            {
                token = token,          // ★ 字段名与服务端 json tag 对齐
            });
            if (!reply.success)
            {
                Game.Logger?.Warn("UI", $"登录失败: {reply.err}");
                return;
            }

            // ★ 登记会话（断线恢复前提），再切场景
            Game.Net.SetupSession(account, null);   // 凭证由全量同步下发（登录回包 session_key 是通道加密密钥，不是凭证）
            Game.UI.Close<LoginPanel>();
            Game.Scene.Load("MainCity");
        }
        catch (CloverCallException ex)
        {
            // 服务端业务错误（EMsg.Error 回包），错误描述在 ex.ServerError
            Game.Logger?.Warn("UI", $"登录失败: {ex.ServerError}");
        }
        catch (System.TimeoutException)
        {
            Game.Logger?.Error("UI", "登录超时");
        }
    }
}
```

> `Layer` 默认 `Normal`。需要弹窗遮罩设为 `UILayer.Popup`，需要最顶层设 `UILayer.Top`/`System`。

## 模板 2：打开 / 关闭 / 取值

```typescript C#
using CloverEngine;

public static class UIFlow
{
    public static void ShowLogin()  => Game.UI.Open<LoginPanel>();

    public static void CloseLogin() => Game.UI.Close<LoginPanel>();
    public static void CloseAll()   => Game.UI.CloseAll();

    // 带参数打开：参数通过 OnOpen(param) 传进去
    public static void ShowItemInfo(ItemData item) => Game.UI.Open<ItemInfoPanel>(item);

    public static void RefreshIfOpen()
    {
        var panel = Game.UI.Get<ItemInfoPanel>();   // ★ 不需要传名字
        if (panel != null) { /* 刷新 */ }

        if (Game.UI.IsOpen<LoginPanel>()) { /* ... */ }
    }

    // 订阅面板开关（做红点、埋点等）
    public static void Bind()
    {
        Game.UI.OnPanelOpened(name => Game.Logger?.Info("UI", $"opened: {name}"));
        Game.UI.OnPanelClosed(name => Game.Logger?.Info("UI", $"closed: {name}"));
    }
}
```

## 模板 3：面板动画等待（用 Timer，不是 await 一个不存在的 API）

```typescript C#
using CloverEngine;
using UnityEngine;

public class AnimatedPanel : UIPanel
{
    [SerializeField] private Animator _animator;

    private const float AnimTime = 0.3f;

    public override void OnOpen(object param)
    {
        gameObject.SetActive(true);
        _animator.SetTrigger("Show");
    }

    public override void OnClose()
    {
        _animator.SetTrigger("Hide");
        // ★ 定时器返回 long id（不是 IDisposable，也没有 FromSeconds）
        Game.Timer.After(AnimTime, () => gameObject.SetActive(false));
    }
}
```

> 需要「关闭时先播动画再真正关」时，别在 `OnClose` 里去动 UIManager；
> 由调用方用 `Game.Timer.After(...)` 延后 `Game.UI.Close<T>()` 更简单。

## 模板 4：列表 UI（走对象池 + 回调式资源加载）

```typescript C#
using System.Collections.Generic;
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class ItemListPanel : UIPanel
{
    [SerializeField] private RectTransform _content;

    private readonly List<GameObject> _slots = new();

    public override void OnOpen(object param)
    {
        var items = param as List<ItemData>;
        if (items == null) return;
        ShowItems(items);
    }

    private void ShowItems(List<ItemData> items)
    {
        foreach (var go in _slots) Game.Pool.Despawn(go);
        _slots.Clear();

        foreach (var item in items)
        {
            // ★ 池化获取，不要裸 Instantiate（战斗/列表路径尤其禁止）
            var slotGo = Game.Pool.Spawn("UI/ItemSlot", _content, "item_list");
            _slots.Add(slotGo);
            slotGo.GetComponent<ItemSlotView>()?.Init(item);
        }
    }

    public override void OnClose()
    {
        foreach (var go in _slots) Game.Pool.Despawn(go);
        _slots.Clear();
    }
}

public class ItemSlotView : MonoBehaviour
{
    [SerializeField] private Image _icon;
    [SerializeField] private TextMeshProUGUI _nameText;
    [SerializeField] private TextMeshProUGUI _countText;

    public void Init(ItemData item)
    {
        _nameText.text = item.Name;
        _countText.text = $"x{item.Count}";

        // ★ 资源加载是回调式，没有同步/await 版本
        Game.Res.LoadAsset<Sprite>($"Icons/{item.IconId}", sp =>
        {
            if (sp != null) _icon.sprite = sp;
        });
    }
}
```

## 模板 5：面板事件监听（记得在 OnClose 里注销）

```typescript C#
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class ShopPanel : UIPanel
{
    [SerializeField] private TextMeshProUGUI _goldText;

    public override void OnOpen(object param)
    {
        Game.Event.On<BuySuccessInfo>("Shop.BuySuccess", OnBuySuccess);
        Game.Event.On<long>("Player.GoldChange", OnGoldChange);
    }

    public override void OnClose()
    {
        Game.Event.Off<BuySuccessInfo>("Shop.BuySuccess", OnBuySuccess);
        Game.Event.Off<long>("Player.GoldChange", OnGoldChange);
    }

    private void OnBuySuccess(BuySuccessInfo info)
    {
        Game.Logger?.Info("UI", $"购买成功: {info.ItemName}");
    }

    private void OnGoldChange(long gold)
    {
        _goldText.text = gold.ToString();
    }
}
```

> `Game.Event.On` 返回 `void`，**没有订阅句柄可以 Dispose**；注销一律用对应的 `Off`。
> 用泛型 `On<T>` 时，`Off<T>` 必须传**同一个方法引用**才能正确移除。

## 模板 6：确认弹窗（把回调当 param 传进去）

```typescript C#
using System;
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class ConfirmDialog : UIPanel
{
    public class Args
    {
        public string Title;
        public string Message;
        public Action OnConfirm;
        public Action OnCancel;
    }

    [SerializeField] private TextMeshProUGUI _titleText;
    [SerializeField] private TextMeshProUGUI _messageText;
    [SerializeField] private Button _confirmButton;
    [SerializeField] private Button _cancelButton;

    private Args _args;

    public override string PanelName => "ConfirmDialog";
    public override UILayer Layer => UILayer.Popup;   // 弹窗：自动加遮罩 + 互斥

    public override void OnOpen(object param)
    {
        _args = param as Args;
        if (_args == null) return;

        _titleText.text = _args.Title;
        _messageText.text = _args.Message;

        _confirmButton.onClick.RemoveAllListeners();
        _cancelButton.onClick.RemoveAllListeners();
        _confirmButton.onClick.AddListener(() => { _args.OnConfirm?.Invoke(); Game.UI.Close<ConfirmDialog>(); });
        _cancelButton.onClick.AddListener(() => { _args.OnCancel?.Invoke(); Game.UI.Close<ConfirmDialog>(); });
    }

    public override void OnClose()
    {
        _args = null;
        _confirmButton.onClick.RemoveAllListeners();
        _cancelButton.onClick.RemoveAllListeners();
    }

    // 使用示例
    public static void ShowDeleteConfirm(int itemId, Action onDeleted)
    {
        Game.UI.Open<ConfirmDialog>(new Args
        {
            Title = "确认删除",
            Message = "确定要删除这个物品吗？",
            OnConfirm = onDeleted,
            OnCancel = () => Game.Logger?.Info("UI", "取消删除"),
        });
    }
}
```

## API 命名对照

> 左列是**错误写法**——其它框架的习惯写法，照抄会编译不过；右列才是 Clover 的真实 API。

| 别处常见写法（错误写法） | Clover 写法 |
|---|---|
| `Game.UI.Show("LoginPanel")` | `Game.UI.Open<LoginPanel>()` |
| `Game.UI.Hide("LoginPanel")` | `Game.UI.Close<LoginPanel>()` |
| `Game.UI.Get<LoginPanel>("LoginPanel")` | `Game.UI.Get<LoginPanel>()` |
| `Game.Scene.LoadScene("MainCity")` | `Game.Scene.Load("MainCity")` |
| `await Game.Timer.FromSeconds(0.3f)` | `Game.Timer.After(0.3f, cb)` |
| `await Game.Res.LoadAsync<T>(path)` | `Game.Res.LoadAsset<T>(path, cb => {...})` |
| `Game.Res.LoadSync<T>(path)` | 不存在同步**加载**；要"立刻拿到"先 `Preload` 预加载、再 `TryGet<T>(path)` 同步取；其余走 `LoadAsset` 回调 |
| `Game.Res.Unload(path)` | `Game.Res.Release(path)` |
| `ex.Error` | `ex.ServerError`（`CloverCallException` 的错误描述属性名） |
| 面板 `: MonoBehaviour` 却用 `Game.UI.Open<T>` | 面板必须实现 `IUIPanel`（推荐直接继承 `UIPanel`） |
| 裸 `Instantiate` 做列表项 | `Game.Pool.Spawn(key, parent, group)` |
