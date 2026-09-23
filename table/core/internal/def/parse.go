package def

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"core/internal/sheet"
)

// 把类型字符串解析为 ColumnType，返回是否成功。
// 支持别名：map[int,int]->map[int]int 等，提升策划容错。
func ParseType(s string) (ColumnType, bool) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "int":
		return TypeInt, true
	case "int32":
		return TypeInt32, true
	case "int64":
		return TypeInt64, true
	case "float32":
		return TypeFloat32, true
	case "float64":
		return TypeFloat64, true
	case "string":
		return TypeString, true
	case "map[int]int", "map[int,int]", "mapkv[int][int]", "mapkv[int,int]":
		return TypeMapIntInt, true
	case "map[int]string", "map[int,string]", "mapkv[int][string]", "mapkv[int,string]":
		return TypeMapIntString, true
	case "[]int", "int[]", "slice[int]":
		return TypeSliceInt, true
	case "[]string", "string[]", "slice[string]":
		return TypeSliceString, true
	case "[]float32", "float32[]", "slice[float32]", "[]float", "float[]", "slice[float]":
		return TypeSliceFloat, true
	case "vector3", "vec3":
		return TypeVector3, true
	default:
		return 0, false
	}
}

// 解析 cs 标记。
func ParseSide(s string, emptyDefault Side) Side {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "c", "client":
		return SideClient
	case "s", "server":
		return SideServer
	case "cs", "sc", "both", "all":
		return SideBoth
	case "":
		return emptyDefault
	default:
		// 未知标记按「都要」处理，避免静默丢列
		return SideBoth
	}
}

func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

// splitters 是一个单元格列表类字段的分隔符。';' 与 map 的 kv 分隔符，必须和运行时解析器
// （gen/go.go 的 parseSlice* / parseMap*、gen/cs.go 的同名方法）保持一致。
const (
	sliceSep = ";"
	mapPair  = "|"
)

// reFloat 是"规范十进制"的形式定义。**三条车道必须完全一致**：
//   - Go 打表期校验（本文件）；
//   - 源侧闸门 tools/verify.ps1 的 $reFloat；
//   - 客户端运行时 C# TableParsers.ToFloat（InvariantCulture + NumberStyles.Float）。
//
// 为什么用正则而不是 strconv.ParseFloat：后者还接受 "inf" / "NaN" / "0x1p-2"（十六进制浮点），
// 而 C# 的 float.TryParse 不接受 ⇒ 那几类写法会在打表期放行、在客户端静默变 0，正是 bug#6 的同类。
// 用正则把定义收紧成"只接受十进制"，三条车道就一致了。已全量核对本仓 + diablo2 源表：
// 非规范写法 0 处（脚本 .ai-tmp/test/scan-strict.py 的口径）。
// 注：本正则同时排除 ".5" / "5."（两端运行时其实都能解析），是**刻意收窄**取无歧义的形式定义，
// 属宁严勿宽；真被拦下改写 "0.5" 即可，不是静默变 0。
var reFloat = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// ValidateCell 按列类型校验一个单元格文本（bug#6）。
//
// 口径（与生成代码的运行时解析器严格对齐）：
//   - **空串一律放行**：语义是 0 / 空容器。策划大量合法留空（本仓实测 2058 格），
//     任何"严格"若把空值判错，就是在惩罚正确数据。
//   - 非空且不合法 ⇒ 返回非 nil error。调用方（Parse）负责把文件名/行号/列名/原文拼进错误。
//   - 列表类（[]int / []float32 / map*）**不限制元素个数**：不同 rank 长度本来就可以不同，
//     长度不符不是缺陷（"1;2" 合法）。但**空项**（"1;;2"、尾随 ";"）是缺陷。
//   - vector3 例外：它是定形类型，必须恰好 3 个分量（运行时对多出的分量静默丢弃、缺失补 0）。
//   - string / []string 不做内容约束（自由文本）。
//
// 这里做校验而不是运行时：打表期报错能挡住"产物已生成、游戏里才静默变 0"，见报告
// table-parse-strictness-impact.md 的判别性说明。
func ValidateCell(t ColumnType, raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil // 空值放行
	}
	switch t {
	case TypeString, TypeSliceString:
		return nil
	case TypeInt, TypeInt64:
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return fmt.Errorf("整数列含非整数")
		}
	case TypeInt32:
		// 必须按 32 位判定，和运行时 parseInt32（strconv.ParseInt(s, 10, 32)）同口径。
		// 若这里用 64 位，`3000000000` 会被判合法 → 生成期放行 → 运行时 ParseInt 失败
		// → 静默变 0，正是本闸门存在的意义（这一格是 2058/26672 之外的越界漏洞）。
		if _, err := strconv.ParseInt(s, 10, 32); err != nil {
			return fmt.Errorf("int32 列含越界值或非整数（超出 int32 范围）")
		}
	case TypeFloat32, TypeFloat64:
		if !reFloat.MatchString(s) {
			return fmt.Errorf("浮点列含非数字（只接受十进制写法，如 19.5 / 1e-3；inf/nan/十六进制浮点会被客户端解析成 0）")
		}
	case TypeSliceInt:
		for _, e := range strings.Split(s, sliceSep) {
			e = strings.TrimSpace(e)
			if e == "" {
				return fmt.Errorf("[]int 含空项（连续或尾随 %q）", sliceSep)
			}
			if _, err := strconv.ParseInt(e, 10, 64); err != nil {
				return fmt.Errorf("[]int 含非整数项 %q", e)
			}
		}
	case TypeSliceFloat:
		for _, e := range strings.Split(s, sliceSep) {
			e = strings.TrimSpace(e)
			if e == "" {
				return fmt.Errorf("[]float32 含空项（连续或尾随 %q）", sliceSep)
			}
			if !reFloat.MatchString(e) {
				return fmt.Errorf("[]float32 含非数字项 %q（只接受十进制写法）", e)
			}
		}
	case TypeMapIntInt, TypeMapIntString:
		for _, pair := range strings.Split(s, mapPair) {
			p := strings.TrimSpace(pair)
			if p == "" {
				return fmt.Errorf("map 含空项（连续或尾随 %q）", mapPair)
			}
			kv := strings.SplitN(p, ";", 2)
			if len(kv) != 2 {
				kv = strings.SplitN(p, ":", 2)
			}
			if len(kv) != 2 {
				return fmt.Errorf("map 项缺少 kv 分隔符（%q 或 %q）：%q", ";", ":", p)
			}
			if _, err := strconv.ParseInt(strings.TrimSpace(kv[0]), 10, 64); err != nil {
				return fmt.Errorf("map 键非整数 %q", strings.TrimSpace(kv[0]))
			}
			if t == TypeMapIntInt {
				if _, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64); err != nil {
					return fmt.Errorf("map 值非整数 %q", strings.TrimSpace(kv[1]))
				}
			}
		}
	case TypeVector3:
		sep := ";"
		if !strings.Contains(s, ";") && strings.Contains(s, ",") {
			sep = ","
		}
		parts := strings.Split(s, sep)
		if len(parts) != 3 {
			return fmt.Errorf("vector3 分量数为 %d（必须恰好 3）", len(parts))
		}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				return fmt.Errorf("vector3 含空分量")
			}
			if !reFloat.MatchString(p) {
				return fmt.Errorf("vector3 含非数字分量 %q（只接受十进制写法）", p)
			}
		}
	}
	return nil
}

// 把工作表解析为 TableDef。emptyDefault 为第 3 行 cs 为空时的默认侧。
func Parse(sh *sheet.Sheet, sourceFile string, emptyDefault Side) (*TableDef, error) {
	if len(sh.Rows) < 5 {
		return nil, fmt.Errorf("def: 工作表 %q 行数不足（至少需 4 行表头 + 1 行数据）", sh.Name)
	}
	nameRow := sh.Rows[0]
	typeRow := sh.Rows[1]
	sideRow := sh.Rows[2]
	commentRow := sh.Rows[3]

	var cols []Column
	maxC := len(nameRow)
	for _, r := range [][]string{typeRow, sideRow, commentRow} {
		if len(r) > maxC {
			maxC = len(r)
		}
	}

	seen := map[string]int{}
	for c := 0; c < maxC; c++ {
		name := strings.TrimSpace(cellAt(nameRow, c))
		if name == "" {
			continue // 字段名为空 -> 整列无效（策划约定：右侧空列）
		}
		// 字段名去重：策划表偶有两列同名，自动加 _N 后缀避免代码/表头冲突。
		if n, ok := seen[name]; ok {
			seen[name]++
			name = fmt.Sprintf("%s_%d", name, n+1)
		} else {
			seen[name] = 1
		}
		rawType := cellAt(typeRow, c)
		ct, ok := ParseType(rawType)
		if !ok {
			// ⛔ 不再静默丢列（旧行为：continue）。列有名字却给不出合法类型，是表头缺陷，
			// 而静默丢列会让"少了一列"一直不可见 —— 产出的结构体/tsv 都悄悄少一列。
			return nil, fmt.Errorf("def: %s!%s 第 %d 列 %q 的类型声明非法：%q（应为 int/int32/int64/float32/float64/string/map[int]int/map[int]string/[]int/[]float32/[]string/vector3）",
				sourceFile, sh.Name, c+1, name, strings.TrimSpace(rawType))
		}
		side := ParseSide(cellAt(sideRow, c), emptyDefault)
		comment := strings.TrimSpace(cellAt(commentRow, c))
		cols = append(cols, Column{
			Name:    name,
			Type:    ct,
			Side:    side,
			Comment: comment,
			OrigCol: c,
			Index:   len(cols),
		})
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("def: 工作表 %q 无有效列", sh.Name)
	}

	pkIsString := cols[0].Type == TypeString

	var rows [][]string
	for r := 4; r < len(sh.Rows); r++ {
		src := sh.Rows[r]
		proj := make([]string, len(cols))
		allEmpty := true
		for i, col := range cols {
			v := strings.TrimSpace(cellAt(src, col.OrigCol))
			proj[i] = v
			if v != "" {
				allEmpty = false
			}
		}
		if allEmpty {
			continue
		}
		// 主键列为空 -> 该行无效（策划约定：第一行没值则整行无效，常用于表尾说明文字）。
		if strings.TrimSpace(proj[0]) == "" {
			continue
		}
		// 第一列以 # 开头 -> 注释行，跳过
		if strings.HasPrefix(proj[0], "#") {
			continue
		}
		// 逐格按列类型严格校验（bug#6）：空值放行，非空不合法即报错。
		// 这一步是"静默吞错"的总闸门：以前 tsv 会带着非法值一路走到游戏运行时才变成 0。
		for i, col := range cols {
			if err := ValidateCell(col.Type, proj[i]); err != nil {
				return nil, fmt.Errorf("def: %s!%s 第 %d 行 列 %q(%s): %v  原文=%q",
					sourceFile, sh.Name, r+1, col.Name, col.Type.String(), err, proj[i])
			}
		}
		rows = append(rows, proj)
	}

	return &TableDef{
		Name:       sh.Name,
		SourceFile: sourceFile,
		Columns:    cols,
		Rows:       rows,
		PkIndex:    0,
		PkIsString: pkIsString,
	}, nil
}
