// Package gen 把 TableDef 生成为 TSV 与 Go/C# 代码。
//
// 设计要点（详见 README）：
//   - base 产物（base_*.go / base_*.cs）与 tsv（tsv/ / Tsv/）每次覆盖；
//   - 上层产物（*.go / *.cs）首次生成后不再覆盖，供业务填加载钩子；
//   - 复合类型序列化约定：map 对用 |、kv 用 ;（兼容 :）；slice 元素用 ;；vector3 用 ; 或 ,。
package gen

import (
	"strings"
	"unicode"
)

// 转为 Go 导出标识符（大驼峰）。
func ToGoIdent(s string) string {
	fields := splitIdent(s)
	var b strings.Builder
	for _, f := range fields {
		if f == "" {
			continue
		}
		r := []rune(f)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	out := b.String()
	if out == "" {
		return "Field"
	}
	return out
}

// 首字母小写（用于接收者/局部变量）。
func ToLowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// 按非字母数字拆分（保留驼峰内部字母数字）。
func splitIdent(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// 从表名解析客户端/服务器后缀，返回逻辑名与后缀。
//
// 命名约定（与策划共识）：
//   "demo_cs" -> ("demo", "cs")  // 客户端 + 服务器都要
//   "demo_c"  -> ("demo", "c")   // 仅客户端
//   "demo_s"  -> ("demo", "s")   // 仅服务器
//   "demo"    -> ("", "")        // 无后缀，该表不生成
//   "demo_"   -> ("", "")        // 仅尾下划线也无后缀，不生成
//
// ok=false 表示该表名没有合法的 _c / _s / _cs 后缀，应整体跳过（不生成任何侧）。
// 逻辑名（如 demo）用于生成文件名与 Go/C# 标识符，后缀不参与命名。
func ParseTableName(name string) (logical, side string, ok bool) {
	i := strings.LastIndex(name, "_")
	if i < 0 {
		return "", "", false
	}
	suf := strings.ToLower(name[i+1:])
	switch suf {
	case "cs", "c", "s":
		return name[:i], suf, true
	}
	return "", "", false
}


