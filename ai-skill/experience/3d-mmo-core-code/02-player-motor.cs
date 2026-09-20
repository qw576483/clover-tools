// 分层落点：Module/Player（对外只暴露一个门面；**不直接碰 Game.Net**，
//           发送能力由 App 注入 Func<uint,float,float,float,bool> SendMove —— 见 architecture.md 依赖规则）
// 核心演示：本地预测 + 输入序号 + ack 回放 + 停发闸门
// 看什么：① 本地先动（手感）② 20Hz 带 seq 上行 ③ 收到校正「置权威值→丢弃已确认→**回放未确认**」
//         ④ 未进图/断线 `SendEnabled=false`（否则被网关按未鉴权拒绝并刷屏）
using System.Collections.Generic;
using UnityEngine;

namespace YourGame.Module.Player
{
    public class PlayerMotor : MonoBehaviour
    {
        public float RunSpeed = 6f, Radius = 0.35f;
        public Transform CameraRef;
        public bool SendEnabled;                                  // ★ 由 Net 模块的进图态驱动
        public System.Func<uint, float, float, float, bool> SendMove;  // 注入，不直连网络
        public System.Action<float> SetAnimSpeed;

        struct Pending { public uint Seq; public Vector3 Delta; }   // 未确认输入窗口
        readonly List<Pending> _pending = new List<Pending>();
        Vector3 _pos, _accum; uint _seq; float _sendAccum;
        const float SendInterval = 0.05f; const int MaxPending = 8;  // ≈ 一个 RTT

        /// <summary>服务端校正：置权威值 → 丢弃 ≤ack → **回放剩余**（直接覆盖 = 橡皮带）。</summary>
        public void ApplyCorrection(uint ackSeq, Vector3 authPos)
        {
            _pos = authPos;
            _pending.RemoveAll(p => p.Seq <= ackSeq);
            foreach (var p in _pending) _pos = Module.Map.MapCollision.Resolve(_pos, _pos + p.Delta, Radius);
            transform.position = _pos;
            Debug.Log($"[Motor] 应用服务端校正 ack={ackSeq} 回放={_pending.Count}");
        }

        public void Teleport(Vector3 p) { _pos = p; _accum = Vector3.zero; _pending.Clear(); transform.position = p; }

        void Update()
        {
            float dt = Time.deltaTime;
            var mv = CloverEngine.Game.Input.State.MoveDirection;      // ★ 引擎输入（后端无关）
            Vector3 dir = Vector3.zero;
            if (mv.sqrMagnitude > 0.001f)
            {
                var f = CameraRef.forward; f.y = 0; var r = CameraRef.right; r.y = 0;
                dir = (f.normalized * mv.y + r.normalized * mv.x).normalized;
                transform.rotation = Quaternion.Euler(0, Mathf.Atan2(dir.x, dir.z) * Mathf.Rad2Deg, 0);
            }

            // ① 本地立即移动 + 本地碰撞（与服务端同规则）
            var want = _pos + dir * (RunSpeed * dt);
            var resolved = Module.Map.MapCollision.Resolve(_pos, want, Radius);
            _accum += resolved - _pos; _pos = resolved; transform.position = _pos;

            // ② 20Hz 上行（带 seq），未确认输入进窗口；★ 停发闸门
            _sendAccum += dt;
            if (_sendAccum >= SendInterval)
            {
                _sendAccum = 0f;
                if (!SendEnabled) { _accum = Vector3.zero; _pending.Clear(); SetAnimSpeed?.Invoke(dir.sqrMagnitude > 0.001f ? 1f : 0f); return; }
                _seq++;
                _pending.Add(new Pending { Seq = _seq, Delta = _accum });
                _accum = Vector3.zero;
                while (_pending.Count > MaxPending) _pending.RemoveAt(0);   // 接受时不回包 → 靠窗口排空
                SendMove?.Invoke(_seq, _pos.x, _pos.y, _pos.z);
            }
            SetAnimSpeed?.Invoke(dir.sqrMagnitude > 0.001f ? 1f : 0f);
        }
    }
}
