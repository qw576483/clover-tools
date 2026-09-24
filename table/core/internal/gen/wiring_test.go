package gen

import (
	"strings"
	"testing"

	"core/internal/def"
)

// TestSliceFloatWiredEndToEnd 钉死 []float32 这条新类型是「端到端」接好的（bug#6）：
// def 里声明了类型、Go/C# 的类型映射、两个 runtime 模板里的解析器，缺任何一环都会让
// 策划写出 `[]float32` 后拿到编译不过或静默变 0 的产物 —— 那正是本次要修的那类缺陷。
// TestCsToFloatIsCultureInvariant 钉死客户端浮点解析的区域无关性（bug#6 的同类隐患）：
// 默认的 float.TryParse(s, out v) 用 CurrentCulture，在逗号小数点区域（如 de-DE）会把
// "19.5" 判为非法 ⇒ 静默 0，而服务端 Go 的 strconv.ParseFloat 是文化无关的 ⇒ 两端分歧。
// 必须 InvariantCulture；且必须是 NumberStyles.Float（**不含** AllowThousands，
// 否则 InvariantCulture 下 "19.5" 会被当千分位解析成 195）。
func TestCsToFloatIsCultureInvariant(t *testing.T) {
	csRT := csRuntime()
	if !strings.Contains(csRT, "using System.Globalization;") {
		t.Error("运行时模板缺少 using System.Globalization;（ToFloat 需要 CultureInfo/NumberStyles）")
	}
	if !strings.Contains(csRT, "NumberStyles.Float, CultureInfo.InvariantCulture") {
		t.Error("ToFloat 未用 (NumberStyles.Float, CultureInfo.InvariantCulture)：区域不同则客户端数值与服务端不一致")
	}
	if strings.Contains(csRT, "NumberStyles.Float | NumberStyles.AllowThousands") ||
		strings.Contains(csRT, "NumberStyles.AllowThousands") {
		t.Error("ToFloat 不得允许千分位：InvariantCulture 下 \"19.5\" 会被解析成 195")
	}
}

func TestSliceFloatWiredEndToEnd(t *testing.T) {
	if got := GoType(def.TypeSliceFloat); got != "[]float32" {
		t.Errorf("GoType([]float32) = %q，期望 \"[]float32\"", got)
	}
	if got := CsType(def.TypeSliceFloat); got != "float[]" {
		t.Errorf("CsType([]float32) = %q，期望 \"float[]\"", got)
	}

	goRT := goRuntime("base")
	if !strings.Contains(goRT, "func parseSliceFloat(s string) []float32") {
		t.Error("生成的 Go 运行时缺少 parseSliceFloat")
	}
	if !strings.Contains(goRT, "ParseFloat") {
		t.Error("Go 的 parseSliceFloat 未走 strconv.ParseFloat（小数会被吃成整数/0）")
	}
	if !strings.Contains(csRuntime(), "public static float[] ParseSliceFloat(string s)") {
		t.Error("生成的 C# 运行时缺少 ParseSliceFloat")
	}
	// 空值放行 + 小数无损：模板里必须保留「整格为空直接返回空容器」与按元素 TrimSpace 的写法。
	if !strings.Contains(goRT, "func parseSliceFloat(s string) []float32 {\n\tout := []float32{}\n\ts = strings.TrimSpace(s)\n\tif s == \"\" {\n\t\treturn out\n\t}") {
		t.Error("Go 的 parseSliceFloat 空值放行分支被改动（2058 个合法空格会受影响）")
	}
}

// TestSliceFloatRowAssignment 钉死生成的行结构体字段类型与赋值调用。
func TestSliceFloatRowAssignment(t *testing.T) {
	cols := []def.Column{{Name: "cooldown", Type: def.TypeSliceFloat, Side: def.SideBoth, Index: 0}}

	goCode := goBaseTable("base", cols, goTableMeta{
		Name:       "champion_skill",
		BaseRow:    "BaseChampionSkillRow",
		BaseTable:  "BaseChampionSkillTable",
		UpperTable: "ChampionSkillTable",
		PkGoType:   "int",
	})
	if !strings.Contains(goCode, "Cooldown []float32") {
		t.Errorf("Go 行结构体未把 cooldown 生成为 []float32：\n%s", goCode)
	}
	if !strings.Contains(goCode, "row.Cooldown = parseSliceFloat(c.str(rec, \"cooldown\"))") {
		t.Errorf("Go Load 未按列名取 cooldown 再走 parseSliceFloat:\n%s", goCode)
	}

	csCode := csBaseTable(&def.TableDef{Name: "champion_skill_cs"}, cols, csTableMeta{
		Name:       "champion_skill",
		BaseRow:    "BaseChampionSkillRow",
		BaseTable:  "BaseChampionSkillTable",
		UpperTable: "ChampionSkillTable",
		PkCsType:   "int",
	})
	if !strings.Contains(csCode, "public float[] Cooldown;") {
		t.Errorf("C# 行结构体未把 cooldown 生成为 float[]：\n%s", csCode)
	}
	if !strings.Contains(csCode, "row.Cooldown = TableParsers.ParseSliceFloat(cols.Str(rec, \"cooldown\"));") {
		t.Errorf("C# Load 未按列名取 cooldown 再走 ParseSliceFloat:\n%s", csCode)
	}
	// C# 侧同样不许按下标取值（插列会静默整体错位），且不得引入 throw（既有 Load 无返回值，抛会改契约）。
	if strings.Contains(csCode, "TableParsers.Cell(rec, ") {
		t.Errorf("C# Load 仍在按下标取值（TableParsers.Cell(rec, i)）：插列会静默整体错位\n%s", csCode)
	}
	if strings.Contains(csCode, "throw ") {
		t.Errorf("C# base 不得抛异常（Load 无返回值 ⇒ 抛会改契约）\n%s", csCode)
	}
	// 严格校验必须是**另开的可选入口**，既有 Load 签名不变。
	for _, want := range []string{
		"public void Load(string path)",
		"public void LoadText(string content)",
		"public bool Validate(string path, out string error)",
		"public bool ValidateText(string content, out string error)",
		"public static readonly TsvColSpec[] TsvSpecs",
		`new TsvColSpec("cooldown", "string"),`, // 复合类型（[]float32）不在严格校验范围内（打表期已逐格校验）,
	} {
		if !strings.Contains(csCode, want) {
			t.Errorf("C# base 缺少 %q：\n%s", want, csCode)
		}
	}
}

// TestExistingSilentParsersUnchanged 本次修复**不许**动既有运行时解析器：
// 改动它们会波及 2058 个合法空格，并需要改 Load 签名。这里把它们的签名钉死，
// 防止有人「顺手统一」把 []int/[]float 也改成报错版。
func TestExistingSilentParsersUnchanged(t *testing.T) {
	goRT := goRuntime("base")
	for _, want := range []string{
		"func parseInt(s string) int",
		"func parseInt32(s string) int32",
		"func parseInt64(s string) int64",
		"func parseFloat64(s string) float64",
		"func parseSliceInt(s string) []int",
		"func parseSliceString(s string) []string",
		"func parseMapIntInt(s string) map[int]int",
		"func parseMapIntString(s string) map[int]string",
		"func parseVector3(s string) Vector3",
	} {
		if !strings.Contains(goRT, want) {
			t.Errorf("既有 Go 运行时解析器签名被改动：%q 不存在（本次修复只允许新增 parseSliceFloat）", want)
		}
	}
	csRT := csRuntime()
	for _, want := range []string{"ToInt(string s)", "ToLong(string s)", "ToFloat(string s)", "ParseSliceInt(string s)", "ParseSliceString(string s)"} {
		if !strings.Contains(csRT, want) {
			t.Errorf("既有 C# 运行时解析器签名被改动：%q 不存在", want)
		}
	}
}
