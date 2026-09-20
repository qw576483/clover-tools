package sheet

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

func openXLSX(path string) (*Book, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("sheet: 打开 %s 失败: %w", path, err)
	}
	defer f.Close()

	book := &Book{File: path}
	for _, name := range f.GetSheetList() {
		raw, err := f.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("sheet: 读取工作表 %q 失败: %w", name, err)
		}
		sh := &Sheet{Name: name}
		for _, r := range raw {
			sh.Rows = append(sh.Rows, trimTrailingEmpty(r))
		}
		book.Sheets = append(book.Sheets, sh)
	}
	return book, nil
}
