// Package pack 把「源表 txt」打包成策划可编辑的 xlsx（打表的反向操作）。
//
// 为什么需要它：打表工具原本是**单向**的（xlsx → tsv + 强类型代码），而 AI 只能产出
// 文本、无法直接生成 xlsx。于是约定一种「源表 txt」——**与 xlsx 的表头布局完全一致**，
// 只是用 tab 分隔：
//
//	name	hp	atk        ← 第 1 行：字段名（空 = 整列无效）
//	int	int	float32    ← 第 2 行：类型
//	cs	cs	c          ← 第 3 行：cs 标记（c=仅客户端 / s=仅服务器 / cs=两端 / 空=按配置默认）
//	名字	生命	攻击       ← 第 4 行：注释
//	1	100	10.5       ← 第 5 行起：数据（第一列为主键，为空的行跳过）
//
// 于是形成闭环：
//
//	AI 写 txt ──[table -pack]──▶ xlsx（策划用 Excel 编辑）
//	                                  │
//	              [table -config] ◀───┘ ──▶ tsv + 强类型代码（程序用）
//
// 文件名（去掉 .txt）即工作表名，因此同样受 `_c` / `_s` / `_cs` 后缀规则约束——
// 不带后缀的表会被打表流程跳过，不会生成任何东西。
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// minRows 一个源表至少要有 4 行表头 + 1 行数据，与 def.Parse 的要求一致。
const minRows = 5

// Result 单个源表的打包结果。
type Result struct {
	Source  string // 源 txt 路径
	Target  string // 目标 xlsx 路径
	Skipped bool   // true = 已存在且未指定 force，未覆盖
	Reason  string // 跳过原因
}

// FromDir 把 dir 下所有 *.txt 源表打包为同目录下的同名 *.xlsx。
//
// 已存在的 xlsx **默认不覆盖**：那多半是策划已经编辑过的成品，AI 不该把它冲掉。
// 确需重打包时传 force=true。
//
// 返回每个文件的处理结果与整体错误（单个文件失败不中断其余文件）。
func FromDir(dir string, force bool) ([]Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("pack: 读取目录 %s 失败: %w", dir, err)
	}

	var results []Result
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			continue
		}
		src := filepath.Join(dir, e.Name())
		dst := filepath.Join(dir, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))+".xlsx")

		res := Result{Source: src, Target: dst}

		if _, statErr := os.Stat(dst); statErr == nil && !force {
			res.Skipped = true
			res.Reason = "目标 xlsx 已存在（可能已被策划编辑），未覆盖；确需重打包请加 -force"
			results = append(results, res)
			continue
		}

		if err := txtToXLSX(src, dst); err != nil {
			res.Skipped = true
			res.Reason = err.Error()
		}
		results = append(results, res)
	}
	return results, nil
}

// txtToXLSX 把单个源表 txt 写为 xlsx（一个工作表，表名 = 文件名去扩展名）。
func txtToXLSX(src, dst string) error {
	// #nosec G304 -- 路径由本包 FromDir 基于用户配置的策划目录拼出。
	raw, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("读取失败: %w", err)
	}
	rows := parseRows(string(raw))
	if len(rows) < minRows {
		return fmt.Errorf("行数不足（%d 行，至少需要 4 行表头 + 1 行数据）", len(rows))
	}

	sheetName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	// excelize 的工作表名不允许这些字符；文件名一般不含，仍做一次兜底替换。
	sheetName = sanitizeSheetName(sheetName)

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if _, err := f.NewSheet(sheetName); err != nil {
		return fmt.Errorf("创建工作表 %q 失败: %w", sheetName, err)
	}
	// 删掉 excelize 默认带的 Sheet1，避免成品里多一个空表
	if sheetName != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	for r, row := range rows {
		for c, cell := range row {
			ref, err := excelize.CoordinatesToCellName(c+1, r+1)
			if err != nil {
				continue
			}
			// 纯数字写成数值，让策划在 Excel 里能直接排序/求和；
			// 其余（含 vector3 的 "5;5;0"、map 的 "1;10|2;20"）保持文本。
			if num, ok := asNumber(cell); ok {
				_ = f.SetCellValue(sheetName, ref, num)
			} else {
				_ = f.SetCellValue(sheetName, ref, cell)
			}
		}
	}

	if err := f.SaveAs(dst); err != nil {
		return fmt.Errorf("保存 %s 失败: %w", dst, err)
	}
	return nil
}

// parseRows 把源表文本切成单元格矩阵：按行切分，行内按 tab 切分。
//
// 兼容 \r\n 与 BOM（Windows 下用记事本/编辑器保存很容易带上）。
func parseRows(content string) [][]string {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	var rows [][]string
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

// asNumber 判断单元格是否为纯数字（含负数与小数）；是则返回其数值。
// 空白单元格不算数字。
func asNumber(s string) (float64, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// sanitizeSheetName 把 excelize 不允许出现在工作表名中的字符替换为下划线。
// 合法上限 31 字符，超长截断。
func sanitizeSheetName(name string) string {
	const invalid = `[]:*?/\`
	cleaned := strings.Map(func(r rune) rune {
		if strings.ContainsRune(invalid, r) {
			return '_'
		}
		return r
	}, name)
	if cleaned == "" {
		cleaned = "Sheet1"
	}
	if len(cleaned) > 31 {
		cleaned = cleaned[:31]
	}
	return cleaned
}
