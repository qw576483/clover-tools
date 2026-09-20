// 分层落点：Module/Net（**唯一允许直接调用 Game.Net 的模块**；其它模块要发消息就调它的方法）
// 核心演示：断线重连 / 重登 + 进图闸门（不接这些 = "断线后 20Hz 刷未鉴权日志 + 两个客户端同账号互相拽回"）
// 看什么：① 事件 → 动作一一对应 ② **登录重入保护** ③ `OnConnected` 只在"曾登录且本连接未绑定"时重登
using System;
using System.Threading.Tasks;
using CloverEngine;

namespace YourGame.Module.Net
{
    public class NetModule
    {
        public bool InMap { get; private set; }
        bool _sessionBound, _everLoggedIn, _loginInFlight;
        Func<Task> _loginFlow, _reEnterMap;

        public void Init(Func<Task> loginFlow, Func<Task> reEnterMap)
        {
            _loginFlow = loginFlow; _reEnterMap = reEnterMap;

            Game.Event.On("Net.OnConnected", () =>
            {
                // ★ 只有"曾登录过但本连接未绑定"才算重连；首次连接由 App 启动流程负责，
                //   否则会对同一账号**登录两次**（实测：旧连接绑定被顶掉却仍在发消息 → 互相拽回）
                if (!_sessionBound && _everLoggedIn) _ = LoginAsync();
            });
            Game.Event.On("Net.OnDisconnected", () => SetInMap(false));                 // 立刻停发
            Game.Event.On<EResumeSessionReply>("Net.OnResumed", _ => { _sessionBound = true; _ = ReEnterMapAsync(); });
            Game.Event.On<string>("Net.OnResumeFailed", r => _sessionBound = false);    // 等下次 OnConnected 重登
            Game.Event.On<EErrorReply>("Net.OnUnauthorized", e => { _sessionBound = false; SetInMap(false); });
            Game.Event.On("Net.OnKicked", () => { _sessionBound = false; SetInMap(false); });
        }

        /// <summary>重入保护：启动与重连两条路径都可能触发，必须只跑一次。</summary>
        public async Task LoginAsync()
        {
            if (_loginInFlight) return;
            _loginInFlight = true;
            try { if (_loginFlow != null) await _loginFlow(); _sessionBound = true; _everLoggedIn = true; }
            catch (Exception e) { Game.Logger?.Error("Net", "登录流程异常: " + e.Message, e); }
            finally { _loginInFlight = false; }
        }

        /// <summary>进图开关（唯一入口）：同时驱动 PlayerMotor.SendEnabled，防止未鉴权期发消息。</summary>
        public void SetInMap(bool inMap)
        {
            if (InMap == inMap) return;
            InMap = inMap;
            Game.Event.Emit(Core.Events.InMapChanged, inMap);   // PlayerMotor 订阅它
        }

        async Task ReEnterMapAsync()
        {
            await Task.Delay(300);                              // 等引擎写完恢复后的会话状态
            if (_reEnterMap != null) await _reEnterMap();
        }
    }
}
