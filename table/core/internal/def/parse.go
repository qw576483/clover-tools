package def

import (
	"fmt"
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
			continue // 字段名为空 -> 整列无效
		}
		// 字段名去重：策划表偶有两列同名，自动加 _N 后缀避免代码/表头冲突。
		if n, ok := seen[name]; ok {
			seen[name]++
			name = fmt.Sprintf("%s_%d", name, n+1)
		} else {
			seen[name] = 1
		}
		ct, ok := ParseType(cellAt(typeRow, c))
		if !ok {
			continue // 类型无效 -> 整列无效
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
