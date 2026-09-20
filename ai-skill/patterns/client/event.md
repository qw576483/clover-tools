# 事件总线模板

## 模板 1：基础事件注册和发布

```typescript C#
using CloverEngine;
using UnityEngine;

public class EventExample : MonoBehaviour
{
    void Start()
    {
        // 注册监听（带参事件 → 显式 On<T>；传方法组时必须把 T 写明，否则无法推断）
        Game.Event.On<LevelUpData>("Player.LevelUp", OnLevelUp);
        
        // 发布事件
        Game.Event.Emit("Player.LevelUp", new LevelUpData
        {
            NewLevel = 10,
            OldLevel = 9
        });
    }

    private void OnLevelUp(LevelUpData data)
    {
        Game.Logger?.Info("Event", $"升级: {data.OldLevel} -> {data.NewLevel}");
    }

    void OnDestroy()
    {
        // 取消监听（同样要写明泛型参数）
        Game.Event.Off<LevelUpData>("Player.LevelUp", OnLevelUp);
    }
}
```

## 模板 2：一次性监听

```typescript C#
using CloverEngine;
using UnityEngine;

public class OneTimeEvent : MonoBehaviour
{
    void Start()
    {
        // 一次性监听（触发一次后自动移除；无参事件 → 处理器必须零参）
        Game.Event.Once("Game.FirstLogin", () =>
        {
            Game.Logger?.Info("Event", "首次登录");
            ShowWelcomeGift();
        });
    }

    private void ShowWelcomeGift()
    {
        Game.Logger?.Info("Event", "显示欢迎礼包");
    }
}
```

## 模板 3：带命名空间的事件

```typescript C#
using CloverEngine;
using UnityEngine;

public class NamespacedEvents : MonoBehaviour
{
    void Start()
    {
        // 使用命名空间格式：Module.EventName
        Game.Event.On("Net.OnConnected", OnConnected);
        Game.Event.On("Net.OnDisconnected", OnDisconnected);
        Game.Event.On<int>("Player.GoldChange", OnGoldChange);              // 带参事件 → 显式 On<T>
        Game.Event.On<BuySuccessInfo>("Shop.BuySuccess", OnBuySuccess);
    }

    // Net.OnConnected / Net.OnDisconnected 均为**无参发布** → 处理器必须零参（绑单参委托运行期永不回调）
    private void OnConnected()
    {
        Game.Logger?.Info("Event", "连接成功");
    }

    private void OnDisconnected()
    {
        Game.Logger?.Warn("Event", "连接断开");
    }

    private void OnGoldChange(int gold)
    {
        Game.Logger?.Info("Event", $"金币变化: {gold}");
    }

    private void OnBuySuccess(BuySuccessInfo info)
    {
        Game.Logger?.Info("Event", $"购买成功: {info.ItemName}");
    }
}
```

## 模板 4：事件数据类

```typescript C#
// 定义事件数据
public class LevelUpData
{
    public int OldLevel;
    public int NewLevel;
}

public class BuySuccessInfo
{
    public int ItemId;
    public string ItemName;
    public int Count;
    public int Cost;
}

// 使用事件数据
public class EventWithData : MonoBehaviour
{
    void Start()
    {
        Game.Event.On<LevelUpData>("Player.LevelUp", data =>
        {
            var info = data as LevelUpData;
            Game.Logger?.Info("Event", $"升级: {info.OldLevel} -> {info.NewLevel}");
        });
    }

    public void TriggerLevelUp(int oldLevel, int newLevel)
    {
        Game.Event.Emit("Player.LevelUp", new LevelUpData
        {
            OldLevel = oldLevel,
            NewLevel = newLevel
        });
    }
}
```

## 模板 5：网络事件监听

```typescript C#
using CloverEngine;
using UnityEngine;

public class NetworkEvent监听 : MonoBehaviour
{
    void Start()
    {
        // 监听网络生命周期事件（无参发布的事件 → 处理器必须零参；带参事件 → On<T> 显式给类型）
        Game.Event.On("Net.OnConnected", () =>
        {
            Game.Logger?.Info("Event", "已连接到服务器");
            OnConnected();
        });

        Game.Event.On("Net.OnDisconnected", () =>
        {
            Game.Logger?.Warn("Event", "连接断开，正在重连...");
            OnDisconnected();
        });

        Game.Event.On<EResumeSessionReply>("Net.OnResumed", data =>
        {
            Game.Logger?.Info("Event", "会话已恢复");
            OnResumed();
        });

        Game.Event.On("Net.OnKicked", () =>
        {
            Game.Logger?.Warn("Event", "被踢出，需要重新登录");
            OnKicked();
        });
    }

    private void OnConnected()
    {
        // 连接成功后的处理
    }

    private void OnDisconnected()
    {
        // 断开连接后的处理
    }

    private void OnResumed()
    {
        // 会话恢复后的处理
    }

    private void OnKicked()
    {
        // 被踢出后的处理
    }
}
```

## 模板 6：批量取消监听

```typescript C#
using CloverEngine;
using UnityEngine;
using System.Collections.Generic;

public class BatchEventHandler : MonoBehaviour
{
    void Start()
    {
        // 注册：On 返回 void，没有可 Dispose 的句柄
        Game.Event.On("Event1", Handler1);
        Game.Event.On("Event2", Handler2);
        Game.Event.On("Event3", Handler3);
    }

    void OnDestroy()
    {
        // 注销：逐个 Off（传同一个方法引用），或一次性 OffAll
        Game.Event.Off("Event1", Handler1);
        Game.Event.Off("Event2", Handler2);
        Game.Event.Off("Event3", Handler3);

        // 更省事的写法（本组件只关心这几条事件时可用）：
        // Game.Event.OffAll("Event1");
        // Game.Event.OffAll();          // 清掉全部事件的全部监听
    }

    // 不带载荷 → 用 On(name, Action)，处理器必须无参
    private void Handler1() { }
    private void Handler2() { }
    private void Handler3() { }
}
```
