// Package sheet 读取策划表（.xls / .xlsx），统一抽象为 Sheet{Rows [][]string}。
//
// 两个后端：
//   - .xlsx/.xlsm/... 走 excelize
//   - .xls（Excel 97-2003）走 extrame/xls
//
// 统一输出纯文本单元格矩阵，剥离所有样式与公式（excelize 返回计算值、xls 返回存储值）。
package sheet

import (
	"fmt"
	"path/filepath"
	"strings"
)

// 单个工作表（已剥离样式，仅保留文本单元格）。
type Sheet struct {
	Name string
	Rows [][]string
}

// 一个工作簿（一个 xls/xlsx 文件）解析结果。
type Book struct {
	File   string
	Sheets []*Sheet
}

// 根据扩展名分发到 xlsx / xls 读取器，返回全部工作表。
func Open(path string) (*Book, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return openXLSX(path)
	case ".xls":
		return openXLS(path)
	default:
		return nil, fmt.Errorf("sheet: 不支持的表格格式 %q（仅支持 .xls/.xlsx）", ext)
	}
}

// 裁掉行尾连续的空单元格（很多表格右侧有大量空列）。
func trimTrailingEmpty(row []string) []string {
	n := len(row)
	for n > 0 && strings.TrimSpace(row[n-1]) == "" {
		n--
	}
	return row[:n]
}
