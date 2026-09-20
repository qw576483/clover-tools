package gen

import (
	"fmt"
	"path/filepath"
	"strings"

	"core/internal/def"
)

// 把表某侧投影为 TSV 字符串：第一行为字段名，后续为数据行。
// 复合类型直接透传策划在单元格中填写的原字符串（约定见 README），不做二次转换。
func GenTSV(t *def.TableDef, side def.Side) string {
	cols := t.ColumnsForSide(side)
	var b strings.Builder
	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Name
	}
	b.WriteString(strings.Join(headers, "\t"))
	b.WriteString("\n")
	for _, row := range t.Rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = row[c.Index]
		}
		b.WriteString(strings.Join(cells, "\t"))
		b.WriteString("\n")
	}
	return b.String()
}

// 把表某侧投影写出为单个 tsv 文件（dir/<fileName>），每次生成覆盖。
// tsv 是生成的「数据」，不像代码那样区分 base/上层，故只输出一份权威文件。
// fileName 为 tsv 文件名（含 .tsv，已剥离 _c/_s/_cs 后缀，如 demo.tsv）。
func WriteTSV(dir, fileName string, t *def.TableDef, side def.Side) error {
	content := GenTSV(t, side)
	if _, err := WriteFile(filepath.Join(dir, fileName), content, true); err != nil {
		return fmt.Errorf("gen: 写 tsv 失败: %w", err)
	}
	return nil
}
