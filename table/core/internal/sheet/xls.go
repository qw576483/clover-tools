package sheet

import (
	"fmt"

	"core/internal/thirdparty/xls"
)

func openXLS(path string) (*Book, error) {
	// 第二个参数为密码，空串表示无密码。
	f, err := xls.Open(path, "")
	if err != nil {
		return nil, fmt.Errorf("sheet: 打开 %s 失败: %w", path, err)
	}
	book := &Book{File: path}
	// extrame/xls 的 WorkSheet 无列数信息，采用上限读取再由 trimTrailingEmpty 裁掉右侧空列。
	// 512 足以覆盖绝大多数策划表；超大表建议拆分或使用 .xlsx。
	const maxCol = 512
	for i := 0; i < f.NumSheets(); i++ {
		sh := f.GetSheet(i)
		if sh == nil {
			continue
		}
		sheet := &Sheet{Name: sh.Name}
		maxRow := int(sh.MaxRow)
		for r := 0; r <= maxRow; r++ {
			row := sh.Row(r)
			if row == nil {
				// 缺失行（空行）：保留一行空切片以维持行号对齐
				sheet.Rows = append(sheet.Rows, []string{})
				continue
			}
			cells := make([]string, maxCol)
			for c := 0; c < maxCol; c++ {
				cells[c] = row.Col(c)
			}
			sheet.Rows = append(sheet.Rows, trimTrailingEmpty(cells))
		}
		book.Sheets = append(book.Sheets, sheet)
	}
	return book, nil
}
