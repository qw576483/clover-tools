// 分层落点：Module/Map（静态工具类，无业务依赖；其它模块只用它的公开方法）
// 核心演示：客户端本地碰撞 —— **空间事实来自引擎，手感规则属于业务**
// 看什么：
//   ① 数据与格换算**不要自己写**：引擎 `Game.Map` 读的是服务端加载的**同一份字节**，
//      格换算（floor 取整）/ 越界 / 位图方向由引擎保证两端口径一致（契约见
//      [`clover-server-engine/pkg/domain/mmo/mapdata/README.md`](https://github.com/qw576483/clover-server-engine/blob/main/pkg/domain/mmo/mapdata/README.md)）；
//   ② 半径 8 向采样（避免"半个身子插进墙里"）
//   ③ 分轴滑墙（撞墙贴墙走，而不是一头撞停）
//   ④ 扫掠细分（每段 ≤0.25m，只检查终点会"穿墙"）
// ★ ②③④ 刻意留在业务：客户端引擎 clover-client-unity-engine-index.md（WorldSync「移动预测」行）「引擎不做本地预测」——
//   用多大半径、几点采样、撞墙是停还是滑，都是玩法手感，不是引擎该定的。
using UnityEngine;
using CloverEngine;   // Game.Map = 逻辑地图（服务端权威地图在客户端的只读投影）

namespace YourGame.Module.Map
{
    public static class MapCollision
    {
        /// <summary>Resources 下的地图数据路径（**不带扩展名**；文件是 `MapData/xxx.bytes`）。</summary>
        public const string ResPath = "MapData/map-city";

        public static bool Loaded => Game.Map != null && Game.Map.Loaded;

        /// <summary>异步加载地图数据（走 `Game.Res`，不直调 `Resources` 原生 API）。</summary>
        public static void Load(System.Action<bool> onDone = null)
        {
            if (Game.Map == null)
            {
                // 非预期分支：引擎地图模块没挂上（Launch 未跑完）→ 必须留痕
                Game.Logger.Error("Map", "引擎地图模块未挂载（Game.Map 为空），本地碰撞不可用", null);
                onDone?.Invoke(false);
                return;
            }
            Game.Map.LoadFromResource(ResPath, onDone);
        }

        /// <summary>
        /// 空间事实：**问引擎**（`Game.Map.WalkableAt`）—— 与服务端 `mapdata.WalkableAt`
        /// 同一份位图、同一套 floor 取整口径。
        /// ★ 不要自己写位图解码与格换算：那正是"两端各自看着都对、合起来就是不对"的经典来源。
        /// </summary>
        public static bool IsWalkable(float x, float z)
            => Game.Map == null || !Game.Map.Loaded || Game.Map.WalkableAt(x, z);
        //   ↑ 未加载不阻挡：地图缺失是导出/配置问题，不该表现成"玩家被锁死在原地"

        /// <summary>中心 + 半径 8 向采样：避免"半个身子插进墙里"。</summary>
        public static bool CanStand(Vector3 p, float r)
        {
            if (!IsWalkable(p.x, p.z)) return false;
            const float k = 0.70710678f;
            return IsWalkable(p.x + r, p.z) && IsWalkable(p.x - r, p.z)
                && IsWalkable(p.x, p.z + r) && IsWalkable(p.x, p.z - r)
                && IsWalkable(p.x + r * k, p.z + r * k) && IsWalkable(p.x - r * k, p.z - r * k)
                && IsWalkable(p.x + r * k, p.z - r * k) && IsWalkable(p.x - r * k, p.z + r * k);
        }

        /// <summary>
        /// ① 分轴解算（先 X 后 Z）→ 撞墙**贴墙滑行**（不是一头撞停）；
        /// ② 扫掠细分（每段 ≤0.25m）→ 只检查终点会**穿墙**（实测：一次 7.5m 位移从建筑左跳到右）。
        /// </summary>
        public static Vector3 Resolve(Vector3 from, Vector3 to, float r)
        {
            if (!Loaded) return to;

            // ★ 解除卡死：脚下本来就"站不住"时（出生点/传送点落在几何体夹角里、地图更新后变挤），
            //   绝不能把玩家锁死 —— 否则表现就是「按 WASD 只在原地跑动画，位置一动不动」。
            //   规则：脚下不合法时放行移动（目标点必须可走），让玩家自己走出去；
            //   "出生点不合法"这件事由**服务端加载地图时做净空校验**兜住（根治）。
            if (!CanStand(from, r)) return IsWalkable(to.x, to.z) ? to : from;

            var delta = to - from; delta.y = 0f;
            int steps = Mathf.Max(1, Mathf.CeilToInt(delta.magnitude / 0.25f));
            var step = delta / steps;

            var p = from;
            for (int i = 0; i < steps; i++)
            {
                var tx = new Vector3(p.x + step.x, from.y, p.z);
                if (CanStand(tx, r)) p.x = tx.x;
                var tz = new Vector3(p.x, from.y, p.z + step.z);
                if (CanStand(tz, r)) p.z = tz.z;
            }
            return new Vector3(p.x, to.y, p.z);
        }
    }
}
