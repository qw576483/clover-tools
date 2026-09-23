package gen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"core/internal/def"
)

// 对照表一条记录。
type MappingEntry struct {
	AbsPath string // 产物绝对路径
	Source  string // 来源 xls/xlsx 文件名
	Sheet   string // 工作表名
	Side    string // s=服务器 c=客户端
	Base    bool   // 是否 base 产物
	Kind    string // tsv / go / cs
}

// 服务器侧（go）输出配置。
//
// 生成的目录/包结构（符合 Go “包名=目录名” 约定）：
//
//	CodeDir/            <- 上层包 Pkg（默认 table）
//	  <logical>.go     <- 业务扩展层（嵌入 base，可编辑，不覆盖）
//	  registry.go      <- 业务 registry（覆盖，每次运行重新生成）
//	CodeDir/base/       <- base 包 BasePkg（默认 base）
//	  base_table.go    <- 共享运行时（覆盖）
//	  base_<logical>.go<- 强类型 Row/Table/Get/Load（覆盖）
//	  base_registry.go <- base registry（覆盖）
//
// tsv 是纯数据，不分 base/上层，直接输出一份权威文件到 TSVDir/<logical>.tsv（每次覆盖）。
type GoOutput struct {
	Pkg         string // 上层包名，默认 table（应与 CodeDir 目录名一致）
	BasePkg     string // base 包名，默认 base（应与 BaseCodeDir 目录名一致）
	BaseImport  string // base 包的完整导入路径（如 <module>/game/table/base），上层代码据此 import
	CodeDir     string // 上层代码目录（如 game/table）
	BaseCodeDir string // base 代码目录（如 game/table/base）
	TSVDir      string // tsv 数据目录（直接输出 <logical>.tsv，每次覆盖）
}

// 返回列的 Go 类型字符串。
func GoType(t def.ColumnType) string {
	switch t {
	case def.TypeInt:
		return "int"
	case def.TypeInt32:
		return "int32"
	case def.TypeInt64:
		return "int64"
	case def.TypeFloat32:
		return "float32"
	case def.TypeFloat64:
		return "float64"
	case def.TypeString:
		return "string"
	case def.TypeMapIntInt:
		return "map[int]int"
	case def.TypeMapIntString:
		return "map[int]string"
	case def.TypeSliceInt:
		return "[]int"
	case def.TypeSliceFloat:
		return "[]float32"
	case def.TypeSliceString:
		return "[]string"
	case def.TypeVector3:
		return "Vector3"
	default:
		return "string"
	}
}

// 单表生成元数据（供 registry 使用）。
type goTableMeta struct {
	Name       string // 逻辑名（剥侧后缀，用于 Go 标识符与结构体字段、以及 tsv 文件名）
	Tsv        string // tsv 文件名（含 .tsv），= 逻辑名 + ".tsv"（如 demo.tsv）
	BaseRow    string // BaseXxxRow
	BaseTable  string // BaseXxxTable
	UpperTable string // XxxTable
	PkGoType   string // int / string
}

// 生成服务器侧（go）全部文件。
// 仅处理含有服务器侧列的表（调用方已过滤）。
// 返回对照表条目。
func GoGenerate(out GoOutput, tables []*def.TableDef, overwriteNonBase bool) ([]MappingEntry, error) {
	if out.Pkg == "" {
		out.Pkg = "table"
	}
	if out.BasePkg == "" {
		out.BasePkg = "base"
	}
	if out.BaseCodeDir == "" {
		out.BaseCodeDir = filepath.Join(out.CodeDir, "base")
	}
	var entries []MappingEntry
	var metas []goTableMeta

	// 1) 运行时（base 子包，覆盖）
	rtPath := filepath.Join(out.BaseCodeDir, "base_table.go")
	if _, err := WriteFile(rtPath, goRuntime(out.BasePkg), true); err != nil {
		return nil, fmt.Errorf("gen: 写运行时失败: %w", err)
	}
	entries = append(entries, MappingEntry{AbsPath: rtPath, Source: "-", Sheet: "-", Side: "s", Base: true, Kind: "go"})

	// 2) 逐表生成 base + 上层；文件名/标识符剥离 _c/_s/_cs 后缀。
	for _, t := range tables {
		logical, _, ok := ParseTableName(t.Name)
		if !ok {
			logical = t.Name // 无后缀：生成器不负责过滤（由 main 门控），原样作为逻辑名
		}
		cols := t.ColumnsForSide(def.SideServer)
		if len(cols) == 0 {
			continue
		}
		meta := goTableMeta{
			Name:       logical,
			Tsv:        logical + ".tsv",
			BaseRow:    "Base" + ToGoIdent(logical) + "Row",
			BaseTable:  "Base" + ToGoIdent(logical) + "Table",
			UpperTable: ToGoIdent(logical) + "Table",
			PkGoType:   goPkType(t),
		}
		metas = append(metas, meta)

		// base 代码（base 子包，覆盖）
		baseCode := goBaseTable(out.BasePkg, cols, meta)
		basePath := filepath.Join(out.BaseCodeDir, "base_"+ToLowerFirst(ToGoIdent(logical))+".go")
		if _, err := WriteFile(basePath, baseCode, true); err != nil {
			return nil, fmt.Errorf("gen: 写 base 代码失败: %w", err)
		}
		entries = append(entries, MappingEntry{AbsPath: basePath, Source: t.SourceFile, Sheet: t.Name, Side: "s", Base: true, Kind: "go"})

		// tsv 数据（单文件，覆盖）
		if err := WriteTSV(out.TSVDir, meta.Tsv, t, def.SideServer); err != nil {
			return nil, fmt.Errorf("gen: 写服务器 tsv 失败: %w", err)
		}
		entries = append(entries,
			MappingEntry{AbsPath: filepath.Join(out.TSVDir, meta.Tsv), Source: t.SourceFile, Sheet: t.Name, Side: "s", Base: false, Kind: "tsv"},
		)

		// 上层代码（上层包，不覆盖）
		upperCode := goUpperTable(out.Pkg, out.BasePkg, out.BaseImport, meta)
		upperPath := filepath.Join(out.CodeDir, ToLowerFirst(ToGoIdent(logical))+".go")
		if _, err := WriteFile(upperPath, upperCode, false); err != nil {
			return nil, fmt.Errorf("gen: 写上层代码失败: %w", err)
		}
		entries = append(entries, MappingEntry{AbsPath: upperPath, Source: t.SourceFile, Sheet: t.Name, Side: "s", Base: false, Kind: "go"})
	}

	// 3) registry（base 覆盖 / 上层不覆盖）
	baseReg := goBaseRegistry(out.BasePkg, metas)
	baseRegPath := filepath.Join(out.BaseCodeDir, "base_registry.go")
	if _, err := WriteFile(baseRegPath, baseReg, true); err != nil {
		return nil, fmt.Errorf("gen: 写 base registry 失败: %w", err)
	}
	entries = append(entries, MappingEntry{AbsPath: baseRegPath, Source: "-", Sheet: "-", Side: "s", Base: true, Kind: "go"})

	upperReg := goUpperRegistry(out.Pkg, metas)
	upperRegPath := filepath.Join(out.CodeDir, "registry.go")
	if _, err := WriteFile(upperRegPath, upperReg, true); err != nil {
		return nil, fmt.Errorf("gen: 写上层 registry 失败: %w", err)
	}
	entries = append(entries, MappingEntry{AbsPath: upperRegPath, Source: "-", Sheet: "-", Side: "s", Base: false, Kind: "go"})

	return entries, nil
}

// 主键 Go 类型。
func goPkType(t *def.TableDef) string {
	if t.PkIsString {
		return "string"
	}
	return "int"
}

// 共享运行时（Vector3 + 复合类型解析 helper），base 覆盖。
func goRuntime(pkg string) string {
	return fmt.Sprintf(`// Code generated by core. DO NOT EDIT (base runtime, regenerated each run).
package %s

import (
	"strconv"
	"strings"
)

// Vector3 三维向量（策划用 ';' 或 ',' 分隔的三个浮点数）。
type Vector3 struct {
	X float32
	Y float32
	Z float32
}

// cell 安全取第 i 个单元格（越界返回空串）。
func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return rec[i]
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

func parseInt32(s string) int32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(v)
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseFloat32(s string) float32 {
	return float32(parseFloat64(s))
}

func parseFloat64(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseMapIntInt 解析 "k;v|k;v" 或 "k:v|k:v"（';' 与 ':' 均可作 kv 分隔）。
func parseMapIntInt(s string) map[int]int {
	out := map[int]int{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, pair := range strings.Split(s, "|") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, ";", 2)
		if len(kv) != 2 {
			kv = strings.SplitN(pair, ":", 2)
		}
		if len(kv) != 2 {
			continue
		}
		k := parseInt(kv[0])
		v := parseInt(kv[1])
		out[k] = v
	}
	return out
}

// parseMapIntString 解析 "k;v|k;v"（value 为原样字符串）。
func parseMapIntString(s string) map[int]string {
	out := map[int]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, pair := range strings.Split(s, "|") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, ";", 2)
		if len(kv) != 2 {
			kv = strings.SplitN(pair, ":", 2)
		}
		if len(kv) != 2 {
			continue
		}
		out[parseInt(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

// parseSliceInt 解析 "a;b;c"（';' 分隔）。
func parseSliceInt(s string) []int {
	out := []int{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, e := range strings.Split(s, ";") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		out = append(out, parseInt(e))
	}
	return out
}

// parseSliceFloat 解析 "a;b;c"（';' 分隔的浮点数，如冷却 22;19.5;17）。
func parseSliceFloat(s string) []float32 {
	out := []float32{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, e := range strings.Split(s, ";") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		out = append(out, parseFloat32(e))
	}
	return out
}

// parseSliceString 解析 "a;b;c"（';' 分隔，保留原样）。
func parseSliceString(s string) []string {
	out := []string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, e := range strings.Split(s, ";") {
		out = append(out, strings.TrimSpace(e))
	}
	return out
}

// parseVector3 解析 "x;y;z" 或 "x,y,z"，取前三个浮点。
func parseVector3(s string) Vector3 {
	var v Vector3
	s = strings.TrimSpace(s)
	if s == "" {
		return v
	}
	sep := ";"
	if !strings.Contains(s, ";") && strings.Contains(s, ",") {
		sep = ","
	}
	parts := strings.Split(s, sep)
	n := 0
	for _, p := range parts {
		f := parseFloat64(p)
		switch n {
		case 0:
			v.X = float32(f)
		case 1:
			v.Y = float32(f)
		case 2:
			v.Z = float32(f)
		default:
			return v
		}
		n++
	}
	return v
}
`, pkg)
}

// 生成 base_<name>.go：Row/Table/Get/Load，覆盖。
func goBaseTable(pkg string, cols []def.Column, m goTableMeta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by core. DO NOT EDIT (base, regenerated each run).\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	fmt.Fprintf(&b, "import (\n\t\"encoding/csv\"\n\t\"fmt\"\n\t\"strings\"\n)\n\n")

	// Row 结构
	fmt.Fprintf(&b, "// %s 表 %q 的一行（强类型）。\n", m.BaseRow, m.Name)
	fmt.Fprintf(&b, "type %s struct {\n", m.BaseRow)
	for _, c := range cols {
		comment := strings.TrimSpace(c.Comment)
		if comment != "" {
			fmt.Fprintf(&b, "\t%s %s // %s\n", ToGoIdent(c.Name), GoType(c.Type), comment)
		} else {
			fmt.Fprintf(&b, "\t%s %s\n", ToGoIdent(c.Name), GoType(c.Type))
		}
	}
	fmt.Fprintf(&b, "}\n\n")

	// Table 结构
	fmt.Fprintf(&b, "// %s 表 %q 的只读容器（按主键索引）。\n", m.BaseTable, m.Name)
	fmt.Fprintf(&b, "type %s struct {\n", m.BaseTable)
	fmt.Fprintf(&b, "\trows  []*%s\n", m.BaseRow)
	fmt.Fprintf(&b, "\tindex map[%s]*%s\n", m.PkGoType, m.BaseRow)
	fmt.Fprintf(&b, "}\n\n")

	// New
	fmt.Fprintf(&b, "// New%s 创建空表。\n", m.BaseTable)
	fmt.Fprintf(&b, "func New%s() *%s {\n", m.BaseTable, m.BaseTable)
	fmt.Fprintf(&b, "\treturn &%s{index: make(map[%s]*%s)}\n", m.BaseTable, m.PkGoType, m.BaseRow)
	fmt.Fprintf(&b, "}\n\n")

	// Get
	pkField := ToGoIdent(cols[0].Name)
	fmt.Fprintf(&b, "// Get 按主键取行（不存在返回 nil）。\n")
	fmt.Fprintf(&b, "func (t *%s) Get(id %s) *%s {\n", m.BaseTable, m.PkGoType, m.BaseRow)
	fmt.Fprintf(&b, "\treturn t.index[id]\n")
	fmt.Fprintf(&b, "}\n\n")

	// Len
	fmt.Fprintf(&b, "// Len 当前行数。\n")
	fmt.Fprintf(&b, "func (t *%s) Len() int { return len(t.rows) }\n\n", m.BaseTable)

	// Rows
	fmt.Fprintf(&b, "// Rows 返回全部行（只读视图）。\n")
	fmt.Fprintf(&b, "func (t *%s) Rows() []*%s { return t.rows }\n\n", m.BaseTable, m.BaseRow)

	// Load 从 tsv 文本加载（首行为列名表头，'\t' 分隔）。整体替换旧数据。
	// redis 等内存源拿到原始 tsv 文本直接传 content；file 源由 registry 读取后传入。
	fmt.Fprintf(&b, "// Load 从 tsv 文本加载（首行为列名表头，'\\t' 分隔）。整体替换旧数据。\n")
	fmt.Fprintf(&b, "// redis 等内存源拿到原始 tsv 文本直接传 content；file 源由 registry 读取后传入。\n")
	fmt.Fprintf(&b, "func (t *%s) Load(content string) error {\n", m.BaseTable)
	fmt.Fprintf(&b, "\tr := csv.NewReader(strings.NewReader(content))\n")
	fmt.Fprintf(&b, "\tr.Comma = '\\t'\n")
	fmt.Fprintf(&b, "\tr.FieldsPerRecord = -1\n")
	// ⛔ 必须为 false：Comma='\t' 时 TrimLeadingSpace 会按 unicode.IsSpace 吃掉前导制表符，
	// 每个空单元格吞掉其后的一个 '\t' ⇒ 该行其后字段整体左移且不报错（客户端 Split('\t') 不受影响 ⇒ 两端分叉）。
	fmt.Fprintf(&b, "\tr.TrimLeadingSpace = false\n")
	fmt.Fprintf(&b, "\trecs, err := r.ReadAll()\n")
	fmt.Fprintf(&b, "\tif err != nil {\n\t\treturn fmt.Errorf(\"%s: 解析 tsv 失败: %%w\", err)\n\t}\n", m.BaseTable)
	fmt.Fprintf(&b, "\tif len(recs) < 1 {\n\t\treturn fmt.Errorf(\"%s: tsv 无数据\")\n\t}\n", m.BaseTable)
	fmt.Fprintf(&b, "\t// 整体替换\n")
	fmt.Fprintf(&b, "\tt.rows = nil\n")
	fmt.Fprintf(&b, "\tt.index = make(map[%s]*%s)\n", m.PkGoType, m.BaseRow)
	fmt.Fprintf(&b, "\tfor _, rec := range recs[1:] {\n")
	fmt.Fprintf(&b, "\t\trow := &%s{}\n", m.BaseRow)
	for i, c := range cols {
		fld := ToGoIdent(c.Name)
		switch c.Type {
		case def.TypeInt:
			fmt.Fprintf(&b, "\t\trow.%s = parseInt(cell(rec, %d))\n", fld, i)
		case def.TypeInt32:
			fmt.Fprintf(&b, "\t\trow.%s = parseInt32(cell(rec, %d))\n", fld, i)
		case def.TypeInt64:
			fmt.Fprintf(&b, "\t\trow.%s = parseInt64(cell(rec, %d))\n", fld, i)
		case def.TypeFloat32:
			fmt.Fprintf(&b, "\t\trow.%s = parseFloat32(cell(rec, %d))\n", fld, i)
		case def.TypeFloat64:
			fmt.Fprintf(&b, "\t\trow.%s = parseFloat64(cell(rec, %d))\n", fld, i)
		case def.TypeString:
			fmt.Fprintf(&b, "\t\trow.%s = cell(rec, %d)\n", fld, i)
		case def.TypeMapIntInt:
			fmt.Fprintf(&b, "\t\trow.%s = parseMapIntInt(cell(rec, %d))\n", fld, i)
		case def.TypeMapIntString:
			fmt.Fprintf(&b, "\t\trow.%s = parseMapIntString(cell(rec, %d))\n", fld, i)
		case def.TypeSliceInt:
			fmt.Fprintf(&b, "\t\trow.%s = parseSliceInt(cell(rec, %d))\n", fld, i)
		case def.TypeSliceFloat:
			fmt.Fprintf(&b, "\t\trow.%s = parseSliceFloat(cell(rec, %d))\n", fld, i)
		case def.TypeSliceString:
			fmt.Fprintf(&b, "\t\trow.%s = parseSliceString(cell(rec, %d))\n", fld, i)
		case def.TypeVector3:
			fmt.Fprintf(&b, "\t\trow.%s = parseVector3(cell(rec, %d))\n", fld, i)
		}
	}
	fmt.Fprintf(&b, "\t\tt.rows = append(t.rows, row)\n")
	fmt.Fprintf(&b, "\t\tt.index[row.%s] = row\n", pkField)
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\treturn nil\n")
	fmt.Fprintf(&b, "}\n\n")

	// Clear 清空全部数据（redis / file 均无数据时调用）。
	fmt.Fprintf(&b, "// Clear 清空全部数据。\n")
	fmt.Fprintf(&b, "func (t *%s) Clear() {\n", m.BaseTable)
	fmt.Fprintf(&b, "\tt.rows = nil\n")
	fmt.Fprintf(&b, "\tt.index = make(map[%s]*%s)\n", m.PkGoType, m.BaseRow)
	fmt.Fprintf(&b, "}\n")

	return b.String()
}

// 生成 <name>.go：嵌入 base + 加载钩子，不覆盖。
// 上层包 import BaseImport（base 子包），类型引用统一加 BasePkg 前缀。
func goUpperTable(pkg, basePkg, baseImport string, m goTableMeta) string {
	return fmt.Sprintf(`// Code generated by core. EDITABLE (上层，首次生成后不再覆盖)。
// 业务可在此安全追加逻辑、覆写下方三个加载钩子。
package %[1]s

import %[5]q

// %[3]s 是 %[2]s 的业务扩展层（嵌入 base，自动获得 Row/Table/Get/Load 能力）。
type %[3]s struct {
	*%[4]s.%[2]s
}

// New%[3]s 构建业务表（底层为 base 表）。
func New%[3]s() *%[3]s {
	return &%[3]s{%[2]s: %[4]s.New%[2]s()}
}

// OnBeforeLoad 加载前钩子（按需覆写）。
func (t *%[3]s) OnBeforeLoad() {}

// OnLoadRow 逐行加载钩子（按需覆写）；row 为该行强类型视图。
func (t *%[3]s) OnLoadRow(row *%[4]s.%[6]s) {}

// OnAfterLoad 加载后钩子（按需覆写）。
func (t *%[3]s) OnAfterLoad() {}

// Load 从 tsv 文本加载，并在加载前/中/后触发钩子。
func (t *%[3]s) Load(content string) error {
	t.OnBeforeLoad()
	if err := t.%[2]s.Load(content); err != nil {
		return err
	}
	for _, row := range t.Rows() {
		t.OnLoadRow(row)
	}
	t.OnAfterLoad()
	return nil
}
`, pkg, m.BaseTable, m.UpperTable, basePkg, baseImport, m.BaseRow)
}

// 生成 base_registry.go（覆盖）：持有全部 base 表并提供 LoadAllBase。
// 仅负责纯加载；加载完成后的「广播 table.Loaded」由引擎（clover-server-engine/app）统一负责，
// 不在生成代码里耦合任何事件总线，保持可独立编译。
func goBaseRegistry(pkg string, metas []goTableMeta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by core. DO NOT EDIT (base registry, regenerated each run).\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	fmt.Fprintf(&b, "import (\n\t\"os\"\n\t\"path/filepath\"\n)\n\n")

	fmt.Fprintf(&b, "// BaseTables 持有全部 base 表实例（按名索引），便于一次性加载。\n")
	fmt.Fprintf(&b, "type BaseTables struct {\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\t%s *%s\n", ToGoIdent(m.Name), m.BaseTable)
	}
	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "// NewBaseTables 创建全部 base 表实例（空数据）。\n")
	fmt.Fprintf(&b, "func NewBaseTables() *BaseTables {\n")
	fmt.Fprintf(&b, "\treturn &BaseTables{\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\t\t%s: New%s(),\n", ToGoIdent(m.Name), m.BaseTable)
	}
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "}\n\n")

	// baseTable 统一读取接口：Load(content) / Len。所有 base 表均满足。
	fmt.Fprintf(&b, "// baseTable 所有 base 表共有的加载接口。\n")
	fmt.Fprintf(&b, "type baseTable interface {\n")
	fmt.Fprintf(&b, "\tLoad(content string) error\n")
	fmt.Fprintf(&b, "\tLen() int\n")
	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "// LoadAllBase 从 dir 加载全部 base 表（dir/<name>.tsv）。\n")
	fmt.Fprintf(&b, "func (t *BaseTables) LoadAllBase(dir string) error {\n")
	fmt.Fprintf(&b, "\tfor _, p := range []struct {\n")
	fmt.Fprintf(&b, "\t\ttbl  baseTable\n")
	fmt.Fprintf(&b, "\t\tname string\n")
	fmt.Fprintf(&b, "\t}{\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\t\t{t.%s, %q},\n", ToGoIdent(m.Name), m.Name)
	}
	fmt.Fprintf(&b, "\t} {\n")
	fmt.Fprintf(&b, "\t\tb, err := os.ReadFile(filepath.Join(dir, p.name+\".tsv\"))\n")
	fmt.Fprintf(&b, "\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif err := p.tbl.Load(string(b)); err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\treturn nil\n")
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// 生成 registry.go（覆盖，每次运行重新生成）：业务表索引 + LoadAll。
// 仅负责纯加载；广播 table.Loaded 由引擎统一负责（见 clover-server-engine/app.RegisterTable）。
// 注意：本文件每次运行都会被重新生成，请勿手写；每个表的自定义逻辑请写在对应的 <name>.go（上一层，可编辑）。
func goUpperRegistry(pkg string, metas []goTableMeta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by core. DO NOT EDIT (上层 registry，每次运行重新生成)。\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg)
	fmt.Fprintf(&b, "import (\n\t\"os\"\n\t\"path/filepath\"\n)\n\n")

	fmt.Fprintf(&b, "// Tables 持有全部业务表实例（按名索引）。\n")
	fmt.Fprintf(&b, "type Tables struct {\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\t%s *%s\n", ToGoIdent(m.Name), m.UpperTable)
	}
	fmt.Fprintf(&b, "}\n\n")

	// 全局单例：首次 NewTables 即被赋值，引擎加载该实例后即为已加载数据。
	fmt.Fprintf(&b, "// Default 全局配置表单例。首次调用 NewTables 时被赋值；引擎加载该实例后，\n")
	fmt.Fprintf(&b, "// 业务任意处可直接 table.Default.<表名>.Get(id) 读取，无需再 NewTables。\n")
	fmt.Fprintf(&b, "var Default *Tables\n\n")

	fmt.Fprintf(&b, "// NewTables 创建全部业务表实例，并设为全局单例 Default。\n")
	fmt.Fprintf(&b, "func NewTables() *Tables {\n")
	fmt.Fprintf(&b, "\tt := &Tables{\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\t\t%s: New%s(),\n", ToGoIdent(m.Name), m.UpperTable)
	}
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\tDefault = t\n")
	fmt.Fprintf(&b, "\treturn t\n")
	fmt.Fprintf(&b, "}\n\n")

	// OnLoadedOne 每加载完一张表触发（底层事件 table.LoadedOne 的钩子）。由业务接线到事件总线；nil 时不触发。
	fmt.Fprintf(&b, "var OnLoadedOne func(name string, count int)\n\n")

	// tsvTable 统一加载接口（base/上层表均满足：Load(content) / Clear / Len）。
	fmt.Fprintf(&b, "type tsvTable interface {\n")
	fmt.Fprintf(&b, "\tLoad(content string) error\n")
	fmt.Fprintf(&b, "\tClear()\n")
	fmt.Fprintf(&b, "\tLen() int\n")
	fmt.Fprintf(&b, "}\n\n")

	// loadTable 单表加载：读 file（<dir>/<name>.tsv）-> Load(content)；文件缺失则清空；完成后触发 OnLoadedOne。
	fmt.Fprintf(&b, "// loadTable 单表加载：读取 <dir>/<name>.tsv 并 Load(content)；文件缺失则清空该表；\n")
	fmt.Fprintf(&b, "// 完成后触发 OnLoadedOne 钩子（由引擎接线到 table.LoadedOne 事件）。\n")
	fmt.Fprintf(&b, "func loadTable(tbl tsvTable, dir, name string) error {\n")
	fmt.Fprintf(&b, "\tp := filepath.Join(dir, name+\".tsv\")\n")
	fmt.Fprintf(&b, "\tif _, err := os.Stat(p); err == nil {\n")
	fmt.Fprintf(&b, "\t\tb, rerr := os.ReadFile(p)\n")
	fmt.Fprintf(&b, "\t\tif rerr != nil {\n\t\t\treturn rerr\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif err := tbl.Load(string(b)); err != nil {\n\t\t\treturn err\n\t\t}\n")
	fmt.Fprintf(&b, "\t\tif OnLoadedOne != nil {\n\t\t\tOnLoadedOne(name, tbl.Len())\n\t\t}\n")
	fmt.Fprintf(&b, "\t\treturn nil\n\t}\n")
	fmt.Fprintf(&b, "\t// file 缺失：清空数据（不报错，便于后续覆盖式热更）。\n")
	fmt.Fprintf(&b, "\ttbl.Clear()\n")
	fmt.Fprintf(&b, "\tif OnLoadedOne != nil {\n\t\tOnLoadedOne(name, 0)\n\t}\n")
	fmt.Fprintf(&b, "\treturn nil\n")
	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "// LoadAll 从 dir 加载全部业务表（每张表读 <dir>/<逻辑名>.tsv，如 demo.tsv）。\n")
	fmt.Fprintf(&b, "// 每加载完一张表触发 OnLoadedOne 钩子（由引擎接线到 table.LoadedOne 事件）。\n")
	fmt.Fprintf(&b, "func (t *Tables) LoadAll(dir string) error {\n")
	for _, m := range metas {
		fmt.Fprintf(&b, "\tif err := loadTable(t.%s, dir, %q); err != nil {\n\t\treturn err\n\t}\n", ToGoIdent(m.Name), m.Name)
	}
	fmt.Fprintf(&b, "\treturn nil\n")
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// 按路径稳定排序（写对照表用）。
func SortEntries(es []MappingEntry) {
	sort.Slice(es, func(i, j int) bool { return es[i].AbsPath < es[j].AbsPath })
}
