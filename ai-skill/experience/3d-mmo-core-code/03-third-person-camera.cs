// 分层落点：Module/CameraRig（纯表现，不依赖任何业务模块）
// 核心演示：第三人称相机的**遮挡避障**（这是"镜头穿墙/看不到角色"的正解）
// 看什么：① 多高度探针取最小 ② **被挡允许拉到很近（别把 MinDistance 当硬下限）** ③ 重叠兜底 ④ 收缩快恢复慢
using UnityEngine;

namespace YourGame.Module.CameraRig
{
    public class ThirdPersonCamera : MonoBehaviour
    {
        public Transform Target;
        public Vector3 PivotOffset = new Vector3(0, 1.35f, 0);
        public float Distance = 5.5f, MinDistance = 0.2f, Pitch = 16f, Yaw, Sensitivity = 0.14f;
        public float CollisionRadius = 0.28f, HideTargetBelow = 0.85f, MinHeightAboveGround = 0.35f;
        public LayerMask BlockMask = ~0;

        static readonly float[] ProbeHeights = { 0f, 0.5f, -0.5f, 1.0f };  // 墙沿/门框要多个高度才不漏
        float _curDist; Vector3 _vel; Renderer[] _rends;

        void LateUpdate()
        {
            if (Target == null) return;
            if (_rends == null) _rends = Target.GetComponentsInChildren<Renderer>();

            var input = CloverEngine.Game.Input;
            if (input != null && !input.IsLocked && input.GetMouseButton(1))   // 右键拖拽转视角
            {
                var d = input.MouseDelta;
                Yaw += d.x * Sensitivity; Pitch = Mathf.Clamp(Pitch - d.y * Sensitivity, -25f, 75f);
            }

            Vector3 pivot = Target.position + PivotOffset;
            Vector3 dirBack = -(Quaternion.Euler(Pitch, Yaw, 0) * Vector3.forward);

            // ① 多高度探针 → 最小可用距离；② 允许拉到很近（**不要** clamp 到保护距离，否则机位留在墙里）
            float want = Distance;
            foreach (var h in ProbeHeights)
                if (Physics.SphereCast(pivot + Vector3.up * h, CollisionRadius, dirBack, out var hit, Distance, BlockMask, QueryTriggerInteraction.Ignore))
                    want = Mathf.Min(want, hit.distance - 0.12f);
            want = Mathf.Clamp(want, MinDistance, Distance);

            // ④ 收缩快 / 恢复慢
            float k = want < _curDist ? 1f - Mathf.Exp(-Time.deltaTime * 30f) : 1f - Mathf.Exp(-Time.deltaTime * 6f);
            _curDist = Mathf.Lerp(_curDist, want, k);

            // ③ 机位仍与几何体重叠 → 继续拉近
            Vector3 pos = pivot + dirBack * _curDist;
            for (int g = 0; g < 8 && Physics.CheckSphere(pos, CollisionRadius * 0.9f, BlockMask, QueryTriggerInteraction.Ignore); g++)
                pos = pivot + dirBack * (_curDist = Mathf.Max(MinDistance, _curDist - 0.18f));
            pos.y = Mathf.Max(pos.y, MinHeightAboveGround);                    // 不钻地

            transform.position = Vector3.SmoothDamp(transform.position, pos, ref _vel, 0.05f);
            transform.rotation = Quaternion.LookRotation(pivot - transform.position, Vector3.up);

            bool hide = _curDist < HideTargetBelow;                            // 贴脸隐藏角色（否则糊屏）
            foreach (var r in _rends) if (r != null) r.enabled = !hide;
        }
    }
}
