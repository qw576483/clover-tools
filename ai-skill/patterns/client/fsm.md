# 状态机模板

## 模板 1：游戏流程状态机

```typescript C#
using CloverEngine;
using UnityEngine;

public class GameFlowManager : MonoBehaviour
{
    private IFsm _gameFlow;

    void Start()
    {
        _gameFlow = Game.Fsm;   // ★ Fsm 是 internal，不要 new；直接用 Game.Fsm
        
        // 注册游戏流程状态
        RegisterLaunchState();
        RegisterLoginState();
        RegisterMainCityState();
        RegisterBattleState();
        
        // 启动流程
        _gameFlow.Force("launch");
    }

    private void RegisterLaunchState()
    {
        _gameFlow.RegisterState("launch",
            onEnter: async () =>
            {
                Game.Logger?.Info("FSM", "进入启动状态");
                // 切换到登录状态
                _gameFlow.Force("login");
            }
        );
    }

    private void RegisterLoginState()
    {
        _gameFlow.RegisterState("login",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入登录状态");
                Game.UI.Open<LoginPanel>();
            },
            onExit: () =>
            {
                Game.UI.Close<LoginPanel>();
            }
        );
    }

    private void RegisterMainCityState()
    {
        _gameFlow.RegisterState("mainCity",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入主城状态");
                Game.Scene.Load("MainCity");
            },
            onExit: () =>
            {
                // 清理主城资源
            }
        );
    }

    private void RegisterBattleState()
    {
        _gameFlow.RegisterState("battle",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入战斗状态");
                Game.Scene.Load("Battle");
            },
            onExit: () =>
            {
                // 清理战斗资源
            }
        );
    }

    public void EnterMainCity()
    {
        _gameFlow.Force("mainCity");
    }

    public void EnterBattle()
    {
        _gameFlow.Force("battle");
    }
}
```

## 模板 2：角色状态机

```typescript C#
using CloverEngine;
using UnityEngine;

public class CharacterFSM : MonoBehaviour
{
    private IFsm _fsm;

    void Start()
    {
        _fsm = Game.Fsm;
        
        // 注册角色状态
        RegisterIdleState();
        RegisterWalkState();
        RegisterAttackState();
        RegisterDieState();
        
        // 初始状态
        _fsm.Force("idle");
    }

    private void RegisterIdleState()
    {
        _fsm.RegisterState("idle",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入待机状态");
                // 播放待机动画
            },
            onTick: (dt) =>
            {
                // 检查是否需要切换状态
                if (HasInput())
                {
                    _fsm.Force("walk");
                }
            }
        );
    }

    private void RegisterWalkState()
    {
        _fsm.RegisterState("walk",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入行走状态");
                // 播放行走动画
            },
            onTick: (dt) =>
            {
                // 更新移动逻辑
                
                if (!HasInput())
                {
                    _fsm.Force("idle");
                }
            },
            onExit: () =>
            {
                // 停止移动
            }
        );
    }

    private void RegisterAttackState()
    {
        _fsm.RegisterState("attack",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入攻击状态");
                // 播放攻击动画
            },
            onExit: () =>
            {
                // 攻击结束
            }
        );
    }

    private void RegisterDieState()
    {
        _fsm.RegisterState("die",
            onEnter: () =>
            {
                Game.Logger?.Info("FSM", "进入死亡状态");
                // 播放死亡动画
                // 禁用碰撞
            }
        );
    }

    private bool HasInput()
    {
        // 检测输入：必须走 Game.Input（后端无关），禁止直连 UnityEngine.Input，
        // 否则 Active Input Handling 不含「旧输入」时会整帧抛异常 → 键鼠全灭。
        // 详见 reference/client-conventions.md §13。
        return Game.Input.GetAxis("Horizontal") != 0 || Game.Input.GetAxis("Vertical") != 0;
    }

    public void Attack()
    {
        if (_fsm.Current == "idle" || _fsm.Current == "walk")
        {
            _fsm.Force("attack");
        }
    }

    public void Die()
    {
        _fsm.Force("die");
    }
}
```

## 模板 3：UI 状态机

```typescript C#
using CloverEngine;
using UnityEngine;

public class UIStateManager : MonoBehaviour
{
    private IFsm _uiState;

    void Start()
    {
        _uiState = Game.Fsm;
        
        // 注册 UI 状态
        RegisterMainMenuState();
        RegisterShopState();
        RegisterInventoryState();
        RegisterSettingsState();
        
        // 初始状态
        _uiState.Force("mainMenu");
    }

    private void RegisterMainMenuState()
    {
        _uiState.RegisterState("mainMenu",
            onEnter: () =>
            {
                Game.UI.Open<MainMenuPanel>();
            },
            onExit: () =>
            {
                Game.UI.Close<MainMenuPanel>();
            }
        );
    }

    private void RegisterShopState()
    {
        _uiState.RegisterState("shop",
            onEnter: () =>
            {
                Game.UI.Open<ShopPanel>();
            },
            onExit: () =>
            {
                Game.UI.Close<ShopPanel>();
            }
        );
    }

    private void RegisterInventoryState()
    {
        _uiState.RegisterState("inventory",
            onEnter: () =>
            {
                Game.UI.Open<InventoryPanel>();
            },
            onExit: () =>
            {
                Game.UI.Close<InventoryPanel>();
            }
        );
    }

    private void RegisterSettingsState()
    {
        _uiState.RegisterState("settings",
            onEnter: () =>
            {
                Game.UI.Open<SettingsPanel>();
            },
            onExit: () =>
            {
                Game.UI.Close<SettingsPanel>();
            }
        );
    }

    public void OpenShop()
    {
        _uiState.Force("shop");
    }

    public void OpenInventory()
    {
        _uiState.Force("inventory");
    }

    public void OpenSettings()
    {
        _uiState.Force("settings");
    }

    public void BackToMainMenu()
    {
        _uiState.Force("mainMenu");
    }
}
```

## 模板 4：带过渡动画的状态机

```typescript C#
using CloverEngine;
using UnityEngine;

public class AnimatedFSM : MonoBehaviour
{
    private IFsm _fsm;
    private Animator _animator;

    void Start()
    {
        _fsm = Game.Fsm;
        _animator = GetComponent<Animator>();
        
        RegisterStates();
        _fsm.Force("idle");
    }

    private void RegisterStates()
    {
        _fsm.RegisterState("idle",
            onEnter: () =>
            {
                _animator.SetTrigger("Idle");
            }
        );

        _fsm.RegisterState("walk",
            onEnter: () =>
            {
                _animator.SetTrigger("Walk");
            }
        );

        _fsm.RegisterState("run",
            onEnter: () =>
            {
                _animator.SetTrigger("Run");
            }
        );
    }

    public void SetMovement(float speed)
    {
        if (speed < 0.1f)
        {
            _fsm.Force("idle");
        }
        else if (speed < 0.5f)
        {
            _fsm.Force("walk");
        }
        else
        {
            _fsm.Force("run");
        }
    }
}
```
