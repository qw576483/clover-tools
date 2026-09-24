package gen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"core/internal/def"
)

// 本文件钉住「base 层 Load 的上浮口径」，判据分三层：
//
//	① TestBaseLoadTemplateStaticMarkers  —— 静态：模板确实生成了按列名取值 + 三类 fail-fast
//	   的结构（含出处注释、且不再引用 encoding/csv、不再出现 cell(rec, i) 调用）。
//	② TestGeneratedBaseLoadEndToEnd      —— 动态（真判据）：把生成物写进临时 module 真跑一遍：
//	   正控 3 例（中间空单元格 / 表头换列序 / 中间插列）必须逐字取对；负控 3 例
//	   （少一列 / 表头缺列 / 整型列填字母）必须各自报出**它自己那条**错误。
//	③ TestEmitGeneratedBaseProbe         —— 人工取证：把生成物落盘到 GEN_PROBE_DIR。
//
// 为什么必须真跑：这三类缺陷的症状都是「不报错、数值全错」（旧模板的 cell(rec, i) 就是
// 按下标取值 ⇒ 源表插一列即整行错位）。只断言字符串出现过的测试无法把它测红 ——
// 把 c.hp 改成 c.str 也照样"通过"。

const probeGoMod = "module probe\n\ngo 1.21\n"

// 探针表的列集合：含「中间的空单元格」（projectile_key）、其后一列（sprite_dir，专治"吃列"），
// 以及一个复合类型列（cooldown，验证复合列也走按列名取）。
func probeCols() []def.Column {
	return []def.Column{
		{Name: "id", Type: def.TypeInt, Side: def.SideServer, Index: 0},
		{Name: "key", Type: def.TypeString, Side: def.SideServer, Index: 1},
		{Name: "hp", Type: def.TypeInt, Side: def.SideServer, Index: 2},
		{Name: "hit_speed_ms", Type: def.TypeInt, Side: def.SideServer, Index: 3},
		{Name: "projectile_key", Type: def.TypeString, Side: def.SideServer, Index: 4},
		{Name: "sprite_dir", Type: def.TypeString, Side: def.SideServer, Index: 5},
		{Name: "cooldown", Type: def.TypeSliceFloat, Side: def.SideServer, Index: 6},
	}
}

func probeMeta() goTableMeta {
	return goTableMeta{
		Name:       "probe",
		Tsv:        "probe.tsv",
		BaseRow:    "BaseProbeRow",
		BaseTable:  "BaseProbeTable",
		UpperTable: "ProbeTable",
		PkGoType:   "int",
	}
}

// 正控 1：字段数与表头一致，但**中间有一格是空的**（projectile_key）。
// 旧模板（csv + TrimLeadingSpace）会在这里吃掉一个 tab ⇒ sprite_dir 及之后整体左移。
const tsvPositive = "id\tkey\thp\thit_speed_ms\tprojectile_key\tsprite_dir\tcooldown\n" +
	"26020001\tarcher\t125\t1200\t\tarchers\t22;19.5;17\n"

// 正控 2：表头**换列序**（sprite_dir 提到最前）。按下标取值必然全错；按列名取值必须全对。
const tsvReordered = "sprite_dir\tcooldown\tkey\tid\tprojectile_key\thit_speed_ms\thp\n" +
	"archers\t22;19.5;17\tarcher\t26020001\t\t1200\t125\n"

// 正控 3：表头**中间插了一列**（brand_new，生成物不认识它）。这是 cr 实测踩到的那个场景：
// 插列后按下标取值会让其后所有字段整体错位且不报错；按列名取值则必须完全不受影响。
const tsvInsertedCol = "id\tkey\tbrand_new\thp\thit_speed_ms\tprojectile_key\tsprite_dir\tcooldown\n" +
	"26020001\tarcher\tX\t125\t1200\t\tarchers\t22;19.5;17\n"

// 负控 1：第 2 行少一列（6 vs 表头 7）⇒ 必须报「第 2 行有 6 列，表头 7 列」。
const tsvShortRow = "id\tkey\thp\thit_speed_ms\tprojectile_key\tsprite_dir\tcooldown\n" +
	"26020001\tarcher\t125\t1200\tarchers\t22;19.5;17\n"

// 负控 2：表头缺一个**本实现会读**的列（hit_speed_ms）⇒ 必须报「表头缺少列 hit_speed_ms」。
const tsvMissingCol = "id\tkey\thp\tprojectile_key\tsprite_dir\tcooldown\n" +
	"26020001\tarcher\t125\t\tarchers\t22;19.5;17\n"

// 负控 3：整型列 hp 填了字母 ⇒ 必须报「第 2 行列 hp 的值 "abc" 不是整数」，⛔ 不许静默按 0。
const tsvBadInt = "id\tkey\thp\thit_speed_ms\tprojectile_key\tsprite_dir\tcooldown\n" +
	"26020001\tarcher\tabc\t1200\t\tarchers\t22;19.5;17\n"

// 驱动程序：把生成物真跑一遍，每例一行输出（便于断言与贴原始输出）。
const probeDriver = `package main

import (
	"fmt"
	"os"

	"probe/base"
)

func read(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		fmt.Println("READFAIL", name, err)
		os.Exit(2)
	}
	return string(b)
}

func dump(tag string, t *base.BaseProbeTable) {
	r := t.Get(26020001)
	if r == nil {
		fmt.Printf("%s n=%d NO-ROW\n", tag, t.Len())
		return
	}
	fmt.Printf("%s n=%d key=%q hp=%d hit=%d proj=%q sprite=%q cd=%v\n",
		tag, t.Len(), r.Key, r.Hp, r.HitSpeedMs, r.ProjectileKey, r.SpriteDir, r.Cooldown)
}

func neg(tag, file string) {
	t := base.NewBaseProbeTable()
	if err := t.Load(read(file)); err != nil {
		fmt.Printf("%s err=%v\n", tag, err)
		return
	}
	fmt.Printf("%s err=<nil> 未被拦下\n", tag)
}

func main() {
	t := base.NewBaseProbeTable()
	if err := t.Load(read("case_positive.tsv")); err != nil {
		fmt.Println("POS load-err:", err)
		os.Exit(3)
	}
	dump("POS", t)

	t2 := base.NewBaseProbeTable()
	if err := t2.Load(read("case_reordered.tsv")); err != nil {
		fmt.Println("REORDER load-err:", err)
		os.Exit(3)
	}
	dump("REORDER", t2)

	t3 := base.NewBaseProbeTable()
	if err := t3.Load(read("case_inserted_col.tsv")); err != nil {
		fmt.Println("INSERT load-err:", err)
		os.Exit(3)
	}
	dump("INSERT", t3)

	neg("NEG1", "case_short_row.tsv")
	neg("NEG2", "case_missing_col.tsv")
	neg("NEG3", "case_bad_int.tsv")
}
`

// 生成一份 base 包到 dir（base/ 子目录），并返回各文件名。
func writeProbeModule(t *testing.T, dir string) {
	t.Helper()
	baseDir := filepath.Join(dir, "base")
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		t.Fatalf("创建 %s 失败: %v", baseDir, err)
	}
	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o600); err != nil {
			t.Fatalf("写 %s 失败: %v", rel, err)
		}
	}
	write("go.mod", probeGoMod)
	write(filepath.Join("base", "base_table.go"), goRuntime("base"))
	write(filepath.Join("base", "base_probe.go"), goBaseTable("base", probeCols(), probeMeta()))
	write("main.go", probeDriver)

	fixtures := map[string]string{
		"case_positive.tsv":     tsvPositive,
		"case_reordered.tsv":    tsvReordered,
		"case_inserted_col.tsv": tsvInsertedCol,
		"case_short_row.tsv":    tsvShortRow,
		"case_missing_col.tsv":  tsvMissingCol,
		"case_bad_int.tsv":      tsvBadInt,
	}
	for name, content := range fixtures {
		write(name, content)
	}
}

// 跑子进程用的环境：剥掉可能干扰的 go 环境变量，并禁网（临时 module 无外部依赖，必须能离线跑）。
func probeEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		switch strings.ToUpper(strings.SplitN(kv, "=", 2)[0]) {
		case "GOFLAGS", "GOPROXY", "GOTOOLCHAIN", "GO111MODULE", "GOWORK":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOFLAGS=", "GOPROXY=off", "GOTOOLCHAIN=local")
}

// 静态判据（廉价、无子进程）：模板必须生成按列名取值 + 三类 fail-fast 的结构。
func TestBaseLoadTemplateStaticMarkers(t *testing.T) {
	rt := goRuntime("base")
	for _, want := range []string{
		"func splitTSV(content string) ([]string, [][]string, error)",
		"type cols struct",
		"func newCols(tableName string, header []string) (*cols, error)",
		"func (c *cols) require(cols ...string) error",
		"func checkRowWidth(tableName string, header, rec []string, line int) error",
		// 出处注释必须留在产物里（后来人要能顺着找到口径真源）。
		"clover-project-cr",
		"server/game/table/tsv.go",
		"三类 fail-fast",
	} {
		if !strings.Contains(rt, want) {
			t.Errorf("base 运行时模板缺少 %q", want)
		}
	}
	// Split 而不是 csv：两端（Go / C#）逐列一致的前提。这里只判**代码级**标记 ——
	// 解释性注释里出现 "encoding/csv" 字样是允许的（那段就是在讲为什么不用它）。
	if strings.Contains(rt, `"encoding/csv"`) || strings.Contains(rt, "csv.NewReader") {
		t.Error("base 运行时模板仍引用 encoding/csv：它把字段当带引号规则的文本解析，与客户端 Split('\\t') 分叉")
	}
	// TrimLeadingSpace 那类「空单元格吃列」的写法必须彻底消失（不只是改成 false）。
	if strings.Contains(rt, ".TrimLeadingSpace") || strings.Contains(rt, "FieldsPerRecord =") {
		t.Error("base 运行时模板仍残留 csv 选项：空单元格吃列的根因应当从结构上消失")
	}

	code := goBaseTable("base", probeCols(), probeMeta())
	if strings.Contains(code, `"encoding/csv"`) {
		t.Error("base 表文件仍 import encoding/csv")
	}
	if strings.Contains(code, "cell(rec, ") {
		t.Error("base 表文件仍在按下标取值（cell(rec, i)）：插列会静默整体错位")
	}
	for _, want := range []string{
		`c, err := newCols("probe", header)`,
		`header, recs, err := splitTSV(content)`,
		`checkRowWidth("probe", header, rec, c.line)`,
		`row.Id = c.int(rec, "id")`,
		`row.Key = c.str(rec, "key")`,
		`row.Hp = c.int(rec, "hp")`,
		`row.HitSpeedMs = c.int(rec, "hit_speed_ms")`,
		`row.ProjectileKey = c.str(rec, "projectile_key")`,
		`row.SpriteDir = c.str(rec, "sprite_dir")`,
		`row.Cooldown = parseSliceFloat(c.str(rec, "cooldown"))`,
		`if c.err != nil`,
		`"id", "key", "hp", "hit_speed_ms",`,
		`"projectile_key", "sprite_dir", "cooldown"); err != nil {`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("base 表模板缺少 %q\n----\n%s", want, code)
		}
	}
}

// 动态判据：把生成物真跑一遍。正控 3 例逐字取对；负控 3 例各自报出自己那条。
func TestGeneratedBaseLoadEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeProbeModule(t, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Dir = dir
	cmd.Env = probeEnv()
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	if err != nil {
		t.Fatalf("go run 失败: %v\n---- 子进程输出 ----\n%s", err, out)
	}
	t.Logf("生成的 base 包运行输出：\n%s", out)

	const got = " n=1 key=\"archer\" hp=125 hit=1200 proj=\"\" sprite=\"archers\" cd=[22 19.5 17]"
	for _, tag := range []string{"POS", "REORDER", "INSERT"} {
		line := findLine(out, tag)
		if line == "" {
			t.Errorf("%s：没有输出行（生成物 Load 未跑通）\n----\n%s", tag, out)
			continue
		}
		if !strings.HasSuffix(line, got) {
			t.Errorf("%s：取值与输入不一致，期望行尾 %q，实得 %q", tag, got, line)
		}
	}
	// 负控：每例必须报出**它自己那条**错误（命中关键词）。
	negs := []struct {
		tag, want string
	}{
		{"NEG1", `err=probe: 第 2 行有 6 列，表头 7 列（列数不一致 ⇒ 数据错位，拒绝加载）`},
		{"NEG2", `err=probe: 表头缺少列 hit_speed_ms；实际表头 = [#0=id #1=key #2=hp #3=projectile_key #4=sprite_dir #5=cooldown]`},
		{"NEG3", `err=probe: 第 2 行列 hp 的值 "abc" 不是整数`},
	}
	for _, n := range negs {
		line := findLine(out, n.tag)
		if line == "" {
			t.Errorf("%s：没有输出行\n----\n%s", n.tag, out)
			continue
		}
		if !strings.Contains(line, n.want) {
			t.Errorf("%s：错误信息不对（可能红的是别的原因），期望含 %q，实得 %q", n.tag, n.want, line)
		}
		if strings.Contains(line, "err=<nil>") {
			t.Errorf("%s：脏数据被放行（Load 返回 nil）⇒ 判据失效", n.tag)
		}
	}
}

func findLine(out, tag string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), tag) {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// 人工取证：GEN_PROBE_DIR 非空时把生成物落盘（默认跳过，避免污染常规测试）。
func TestEmitGeneratedBaseProbe(t *testing.T) {
	dir := os.Getenv("GEN_PROBE_DIR")
	if dir == "" {
		t.Skip("GEN_PROBE_DIR 未设置：跳过（仅用于人工取证）")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("创建 %s 失败: %v", dir, err)
	}
	files := map[string]string{
		"base_table.go": goRuntime("base"),
		"base_probe.go": goBaseTable("base", probeCols(), probeMeta()),
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("写 %s 失败: %v", p, err)
		}
		t.Logf("已写出 %s（%d 字节）", p, len(content))
	}
}
