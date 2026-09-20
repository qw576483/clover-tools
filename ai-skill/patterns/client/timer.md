# 定时器模板

## 模板 1：延迟执行

```typescript C#
using CloverEngine;
using UnityEngine;

public class DelayExample : MonoBehaviour
{
    public void StartDelayedAction()
    {
        // 3秒后执行
        Game.Timer.After(3f, () =>
        {
            Game.Logger?.Info("Timer", "3秒后执行");
        });
    }
}
```

## 模板 2：循环执行

```typescript C#
using CloverEngine;
using UnityEngine;

public class RepeatExample : MonoBehaviour
{
    private long _timerId;   // ★ Timer.Every 返回 long id，不是 IDisposable

    public void StartRepeatAction()
    {
        // 每秒执行一次
        _timerId = Game.Timer.Every(1f, () =>
        {
            Game.Logger?.Info("Timer", "每秒执行");
        });
    }

    public void StopRepeatAction()
    {
        Game.Timer.Stop(_timerId);
    }
}
```

## 模板 3：定时器取消

```typescript C#
using CloverEngine;
using UnityEngine;

public class TimerCancelExample : MonoBehaviour
{
    // 具名定时器：同名会先停掉旧的，之后可按名字停（比记 id 更适合业务）
    private const string TimerName = "my_repeat";

    public void StartTimer()
    {
        Game.Timer.EveryName(TimerName, 1f, () =>
        {
            Game.Logger?.Info("Timer", "循环执行");
        });
    }

    public void StopTimer()
    {
        Game.Timer.StopNamed(TimerName);
    }

    void OnDestroy()
    {
        StopTimer();
    }
}
```

## 模板 4：帧率控制

```typescript C#
using CloverEngine;
using UnityEngine;

public class FrameRateExample : MonoBehaviour
{
    void Start()
    {
        // 设置目标帧率
        Application.targetFrameRate = 60;
        
        // 使用 Timer.After 等待帧
        Game.Timer.After(0f, () =>
        {
            // 下一帧执行
            Game.Logger?.Info("Timer", "下一帧执行");
        });
    }
}
```

## 模板 5：倒计时

```typescript C#
using CloverEngine;
using UnityEngine;
using UnityEngine.UI;

public class CountdownTimer : MonoBehaviour
{
    [SerializeField] private TextMeshProUGUI _timerText;
    
    private float _remainingTime;

    public void StartCountdown(float duration)
    {
        _remainingTime = duration;

        // 每 0.1 秒更新一次；用 EveryName + StopNamed 才能在回调里自我取消
        Game.Timer.EveryName("countdown", 0.1f, () =>
        {
            _remainingTime -= 0.1f;

            if (_remainingTime <= 0)
            {
                _remainingTime = 0;
                Game.Timer.StopNamed("countdown");
                OnCountdownEnd();
            }

            UpdateTimerDisplay();
        });
    }

    private void UpdateTimerDisplay()
    {
        int minutes = Mathf.FloorToInt(_remainingTime / 60);
        int seconds = Mathf.FloorToInt(_remainingTime % 60);
        _timerText.text = $"{minutes:00}:{seconds:00}";
    }

    private void OnCountdownEnd()
    {
        Game.Logger?.Info("Timer", "倒计时结束");
    }
}
```

## 模板 6：定时刷新

```typescript C#
using CloverEngine;
using UnityEngine;

public class AutoRefreshExample : MonoBehaviour
{
    void Start()
    {
        // 每 5 秒刷新一次；用具名定时器，后续想停可按名 StopNamed
        Game.Timer.EveryName("auto_refresh", 5f, () =>
        {
            RefreshData();
        });
    }

    private void RefreshData()
    {
        Game.Logger?.Info("Timer", "刷新数据");
        // 刷新UI或数据
    }
}
```
