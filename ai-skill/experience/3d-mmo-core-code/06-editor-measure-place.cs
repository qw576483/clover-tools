// 分层落点：Assets/Editor（工具层，只依赖 Def/Core；不进运行时模块）
// 核心演示：用第三方素材铺场景的"量测优先"三件事
// 看什么：① 实测包围盒反算缩放（各包单位不一致）② 按包围盒贴地 ③ **主动补 BoxCollider**
//         （FBX 默认不带碰撞体 ⇒ 不补就烘焙出"0 障碍物"的空地图；烘焙日志会自证 `计为障碍=0`）；对位一律传**格中心**
using UnityEditor;
using UnityEngine;

namespace YourGame.EditorTools
{
    public static class EditorMeasurePlace
    {
        /// <summary>格左上角 → 格**中心**（传角点会让整图偏移半格，实测踩过）。</summary>
        public static Vector3 TileCenter(int x, int z, float tile) => new Vector3(x + tile * 0.5f, 0f, z + tile * 0.5f);

        public static GameObject Place(string dir, string name, Vector3 posXZ, Transform parent,
                                       float scaleRef, bool byFootprint, bool alignTop, bool withCollider)
        {
            var prefab = AssetDatabase.LoadAssetAtPath<GameObject>($"{dir}/{name}.fbx");
            if (prefab == null) { Debug.LogWarning("[Forge] 缺模型: " + name); return null; }

            var go = (GameObject)PrefabUtility.InstantiatePrefab(prefab, parent);
            for (int i = 0; i < 2; i++)                       // ① 迭代一次修正缩放
            {
                var b0 = Measure(go);
                float cur = byFootprint ? Mathf.Max(b0.size.x, b0.size.z) : b0.size.y;
                if (cur <= 0.0001f) break;
                float k = scaleRef / cur;
                if (Mathf.Abs(k - 1f) < 0.01f) break;
                go.transform.localScale *= k;
            }

            var b = Measure(go);                              // ② 对格心 + 贴地（地面砖顶面=0，其余物底=0）
            float y = alignTop ? -b.max.y : -b.min.y;
            go.transform.position += new Vector3(posXZ.x - b.center.x, y, posXZ.z - b.center.z);

            if (withCollider && go.GetComponent<Collider>() == null)   // ③ FBX 不带碰撞体 → 自己补
            {
                var ls = go.transform.lossyScale; var bb = Measure(go);
                var col = go.AddComponent<BoxCollider>();
                col.center = go.transform.InverseTransformPoint(bb.center);
                col.size = new Vector3(bb.size.x / Mathf.Abs(ls.x), bb.size.y / Mathf.Abs(ls.y), bb.size.z / Mathf.Abs(ls.z));
            }
            return go;
        }

        static Bounds Measure(GameObject go)   // 实测：合并所有子 Renderer 的世界包围盒
        {
            var rs = go.GetComponentsInChildren<Renderer>();
            if (rs.Length == 0) return new Bounds(go.transform.position, Vector3.one);
            var b = rs[0].bounds;
            for (int i = 1; i < rs.Length; i++) b.Encapsulate(rs[i].bounds);
            return b;
        }
    }
}
