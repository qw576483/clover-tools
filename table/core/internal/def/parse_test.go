package def

import (
	"strings"
	"testing"

	"core/internal/sheet"
)

// sheetOf 把 tab 分隔的文本块拼成一个内存工作表（与源表 *_cs.txt 的布局一致），
// 让本测试不依赖任何磁盘文件。
func sheetOf(name, txt string) *sheet.Sheet {
	var rows [][]string
	for _, l := range strings.Split(strings.ReplaceAll(txt, "\r\n", "\n"), "\n") {
		if l == "" {
			continue
		}
		rows = append(rows, strings.Split(l, "\t"))
	}
	return &sheet.Sheet{Name: name, Rows: rows}
}

// TestValidateCell 逐类型钉死"打表期闸门"的口径（bug#6）：
//   - 空值一律放行（2058 个合法空格是数据，不是缺陷）；
//   - 非空但不合法 ⇒ 报错（运行时解析器会把它静默变 0 的那一类）；
//   - 列表类不限制元素个数（不同 rank 长度本来就不同），但空项（连续/尾随分隔符）是缺陷；
//   - vector3 是定形类型，必须恰好 3 个分量。
func TestValidateCell(t *testing.T) {
	cases := []struct {
		name string
		typ  ColumnType
		val  string
		ok   bool
	}{
		// ---- 正例：空值放行 ----
		{"空 int", TypeInt, "", true},
		{"空 int32", TypeInt32, "", true},
		{"空 float32", TypeFloat32, "", true},
		{"空 []int", TypeSliceInt, "", true},
		{"空 []float32", TypeSliceFloat, "", true},
		{"空 []string", TypeSliceString, "", true},
		{"空 map[int]int", TypeMapIntInt, "", true},
		{"空 vector3", TypeVector3, "", true},
		{"纯空白 int", TypeInt, "   ", true},
		{"纯空白 []float32", TypeSliceFloat, "  ", true},

		// ---- 正例：bug#6 的原始数据必须无损通过 ----
		{"冷却小数列表（Garen W 原始值）", TypeSliceFloat, "22;19.5;17;14.5;12", true},
		{"冷却小数列表（Garen E 原始值）", TypeSliceFloat, "9;8.25;7.5;6.75;6", true},
		{"冷却全整数也合法", TypeSliceFloat, "0;0;0;0;0", true},
		{"冷却单元素", TypeSliceFloat, "60", true},
		{"冷却负小数", TypeSliceFloat, "-1;0.5", true},
		{"浮点科学计数", TypeSliceFloat, "1e-3;2E5", true},

		// ---- 正例：其余类型常规值 ----
		{"[]int 常规", TypeSliceInt, "1;2", true},
		{"[]int 单元素（长度不受限）", TypeSliceInt, "7", true},
		{"[]int 项内空白", TypeSliceInt, " 1 ; 2 ", true},
		{"int 负数", TypeInt, "-3", true},
		{"int32 上边界", TypeInt32, "2147483647", true},
		{"int32 下边界", TypeInt32, "-2147483648", true},
		{"int64 大数", TypeInt64, "9223372036854775807", true},
		{"float 整数写法", TypeFloat64, "3", true},
		{"float 规范小数", TypeFloat64, "19.5", true},
		{"float 前导正号", TypeFloat64, "+2.5", true},
		{"float 大写指数", TypeFloat64, "1E-3", true},
		{"map 常规", TypeMapIntInt, "1;2|3;4", true},
		{"map 冒号 kv", TypeMapIntInt, "1:2", true},
		{"map[int]string 值任意", TypeMapIntString, "1;x|7;a", true},
		{"map[int]string 值含中文", TypeMapIntString, "1;未取到", true},
		{"vector3 分号", TypeVector3, "5;5;0", true},
		{"vector3 逗号", TypeVector3, "100,100,1", true},
		{"string 结构化文本", TypeString, "{TotalDamage:{mFormulaParts:[{mDataValue:BaseDamage}]}}", true},
		{"[]string 尾随分号不判错（自由文本不做约束）", TypeSliceString, "b;", true},

		// ---- 反例 ----
		{"[]int 含非整数项（1;2;x 原始缺陷）", TypeSliceInt, "1;2;x", false},
		{"[]float32 含非数字项", TypeSliceFloat, "22;x;17", false},
		{"[]float32 含非数字项（中段）", TypeSliceFloat, "9;x;6", false},
		{"[]int 连续空项", TypeSliceInt, "1;;2", false},
		{"[]int 尾随分隔符", TypeSliceInt, "1;2;", false},
		{"[]int 前导分隔符", TypeSliceInt, ";1", false},
		{"[]float32 尾随分隔符", TypeSliceFloat, "1;2;", false},
		{"int 非整数", TypeInt, "1.5", false},
		{"int 中文占位", TypeInt, "未取到", false},
		{"int32 越界（运行时 parseInt32 会静默变 0）", TypeInt32, "3000000000", false},
		{"int32 负越界", TypeInt32, "-2147483649", false},
		{"float 含尾随字母", TypeFloat64, "1.5x", false},
		// inf / nan / 十六进制浮点：Go 的 strconv.ParseFloat 收，C# 的 float.TryParse 不收
		// ⇒ 若打表期放行，客户端会静默变 0（bug#6 同类）。三条车道必须同一定义，故一律拒。
		{"float inf（客户端会静默变 0）", TypeFloat64, "inf", false},
		{"float -Infinity", TypeFloat32, "-Infinity", false},
		{"float NaN", TypeFloat32, "NaN", false},
		{"float 十六进制浮点（C# 不认）", TypeFloat64, "0x1p-2", false},
		{"[]float32 含 inf", TypeSliceFloat, "1;inf", false},
		{"vector3 含 NaN", TypeVector3, "1;NaN;0", false},
		// 非规范写法（缺一侧整数/小数位）：收紧成规范十进制，打表期就报错，别留到运行时。
		{"float 尾随小数点", TypeFloat64, "5.", false},
		{"float 前导小数点", TypeFloat64, ".5", false},
		{"map 项缺 kv 分隔符", TypeMapIntInt, "1|2;3", false},
		{"map 键非整数", TypeMapIntInt, "a;1", false},
		{"map[int]int 值非整数", TypeMapIntInt, "1;x", false},
		{"map 尾随分隔符", TypeMapIntInt, "1;2|", false},
		{"map 连续分隔符", TypeMapIntString, "1;a||2;b", false},
		{"vector3 只有 2 个分量", TypeVector3, "5;5", false},
		{"vector3 有 4 个分量", TypeVector3, "1;2;3;4", false},
		{"vector3 含非数字分量", TypeVector3, "5;x;0", false},
		{"vector3 含空分量", TypeVector3, "5;;0", false},
	}
	for _, tc := range cases {
		err := ValidateCell(tc.typ, tc.val)
		switch {
		case tc.ok && err != nil:
			t.Errorf("%s: ValidateCell(%s, %q) = %v；期望合法（放行）", tc.name, tc.typ, tc.val, err)
		case !tc.ok && err == nil:
			t.Errorf("%s: ValidateCell(%s, %q) = nil；期望报错", tc.name, tc.typ, tc.val)
		}
	}
}

const testHeader = "id\tname\thp\tcd\n" +
	"int\tstring\tint\t[]float32\n" +
	"cs\tcs\tcs\tcs\n" +
	"主键\t名字\t生命\t冷却\n"

// TestParseAcceptsLegalAndEmptyRows 正例：合法值 + 合法空值整表通过，
// 且"空值放行"不会把行/列吃掉（行数必须等于数据行数）。
func TestParseAcceptsLegalAndEmptyRows(t *testing.T) {
	txt := testHeader +
		"1\tA\t100\t22;19.5;17\n" + // 小数冷却
		"2\tB\t\t\n" + // 空 int + 空 []float32（2058 格那类合法留空）
		"3\tC\t0\t0;0;0;0;0\n"
	td, err := Parse(sheetOf("t_cs", txt), "t_cs.xlsx", SideBoth)
	if err != nil {
		t.Fatalf("合法数据不该报错：%v", err)
	}
	if len(td.Rows) != 3 {
		t.Fatalf("期望 3 行，实际 %d 行", len(td.Rows))
	}
	if len(td.Columns) != 4 {
		t.Fatalf("期望 4 列，实际 %d 列", len(td.Columns))
	}
	if got := td.Columns[3].Type; got != TypeSliceFloat {
		t.Fatalf("第 4 列类型应为 []float32，实际 %s", got)
	}
}

// TestParseRejectsBadCell 反例：错误串必须能直接定位到「文件名!表名 + 行号 + 列名 + 原文」，
// 不许只说"解析失败"。
func TestParseRejectsBadCell(t *testing.T) {
	txt := testHeader +
		"1\tA\t100\t22;19.5;17\n" +
		"2\tB\t200\t9;x;6\n" // 第 6 行：冷却列含非数字
	_, err := Parse(sheetOf("t_cs", txt), "t_cs.xlsx", SideBoth)
	if err == nil {
		t.Fatal("含 9;x;6 的 []float32 单元格必须报错（旧行为是运行时静默变 0）")
	}
	for _, want := range []string{"t_cs.xlsx", "t_cs", "第 6 行", "cd", "[]float32", "9;x;6"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误串缺少 %q，无法定位；实际：%v", want, err)
		}
	}
}

// TestParseRejectsEmptyListItem 反例：尾随/连续分隔符造成的空项是书写缺陷。
func TestParseRejectsEmptyListItem(t *testing.T) {
	txt := testHeader + "1\tA\t100\t22;19.5;\n"
	_, err := Parse(sheetOf("t_cs", txt), "t_cs.xlsx", SideBoth)
	if err == nil || !strings.Contains(err.Error(), "空项") {
		t.Fatalf("尾随 ';' 应报「空项」错误，实际：%v", err)
	}
}

// TestParseRejectsBadColumnType 反例：列有名字但类型声明非法 ⇒ 报错。
// 旧行为是 `continue` 静默丢列 —— 产出的结构体与 tsv 都会悄悄少一列，且没有任何提示。
func TestParseRejectsBadColumnType(t *testing.T) {
	txt := "id\tbad\n" +
		"int\tstrin\n" + // 类型拼错
		"cs\tcs\n" +
		"主键\t错类型\n" +
		"1\tx\n"
	_, err := Parse(sheetOf("t_cs", txt), "t_cs.xlsx", SideBoth)
	if err == nil {
		t.Fatal("类型声明非法必须报错，不许静默丢列")
	}
	for _, want := range []string{"t_cs.xlsx", "bad", "strin"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误串缺少 %q；实际：%v", want, err)
		}
	}
}

// TestParseSkipsCommentAndBlankRowsButStillValidates 边界：
// 主键为空 / 主键以 # 开头的行仍按约定跳过，且不会被校验误伤
// （它们的非主键列可以随便写，例如表尾说明文字）。
func TestParseSkipsCommentAndBlankRowsButStillValidates(t *testing.T) {
	txt := testHeader +
		"1\tA\t100\t1;2\n" +
		"#\t这是注释行\t随便写\t随便写\n" + // 主键以 # 开头 -> 跳过，不校验
		"\t表尾说明\t\t\n" + // 主键为空 -> 跳过，不校验
		"2\tB\t200\t3;4\n"
	td, err := Parse(sheetOf("t_cs", txt), "t_cs.xlsx", SideBoth)
	if err != nil {
		t.Fatalf("被跳过的行不该参与校验：%v", err)
	}
	if len(td.Rows) != 2 {
		t.Fatalf("期望 2 条数据行，实际 %d 条", len(td.Rows))
	}
}

// TestParseTypeSliceFloatAliases 钉死 []float32 的别名集合（策划容错）。
func TestParseTypeSliceFloatAliases(t *testing.T) {
	for _, s := range []string{"[]float32", "float32[]", "slice[float32]", "[]float", "float[]", "slice[float]", " []Float32 "} {
		got, ok := ParseType(s)
		if !ok || got != TypeSliceFloat {
			t.Errorf("ParseType(%q) = (%v, %v)，期望 TypeSliceFloat", s, got, ok)
		}
	}
}
