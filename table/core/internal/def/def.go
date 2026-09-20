// Package def 把策划表的一个工作表解析为类型安全的 TableDef。
//
// 约定的表头布局（与策划共识，详见 README）：
//   - 第 1 行：字段名（英文）；字段名为空 -> 该列无效
//   - 第 2 行：类型声明；非可用类型 -> 该列无效
//   - 第 3 行：cs 标记（c=客户端 / s=服务器 / cs=都要 / 空=按配置默认）
//   - 第 4 行：注释说明
//   - 第 5 行起：数据；第一列为主键
package def

// 列数据类型。
type ColumnType int

const (
	TypeInt ColumnType = iota
	TypeInt32
	TypeInt64
	TypeFloat32
	TypeFloat64
	TypeString
	TypeMapIntInt
	TypeMapIntString
	TypeSliceInt
	TypeSliceString
	TypeVector3
)

// 数据使用侧。
type Side uint8

const (
	SideServer Side = 1 << iota
	SideClient
)

// 服务器+客户端。
const SideBoth = SideServer | SideClient

// 一列定义。
type Column struct {
	Name    string
	Type    ColumnType
	Side    Side
	Comment string
	OrigCol int // 原表列号，用于从数据行取数
	Index   int // 在父 TableDef.Columns 中的位置，用于从 Rows 投影取数
}

// 一个工作表解析结果。
type TableDef struct {
	Name       string     // 工作表名
	SourceFile string     // 来源 xls/xlsx 文件名（不含目录）
	Columns    []Column   // 有效列（保持原表列顺序）
	Rows       [][]string // 数据行（按 Columns 投影后的字符串，未解析）
	PkIndex    int        // 主键列在 Columns 中的索引（恒为 0，第一列）
	PkIsString bool       // 主键是否为 string 类型
}

// 返回仅属于指定侧的列（保持顺序）。
func (t *TableDef) ColumnsForSide(side Side) []Column {
	var out []Column
	for _, c := range t.Columns {
		if c.Side&side != 0 {
			out = append(out, c)
		}
	}
	return out
}
