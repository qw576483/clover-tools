//lint:file-ignore U1000 第三方嵌入的 XLS 解析代码，保留字段/方法作接口/未来扩展。
package xls

import (
	"io"
	"os"

	"core/internal/thirdparty/ole2"
)

// one xls file
func Open(file string, charset string) (*WorkBook, error) {
	// #nosec G304 -- explicit file open API; caller controls path safety.
	if fi, err := os.Open(file); err == nil {
		return OpenReader(fi, charset)
	} else {
		return nil, err
	}
}

// Open xls file from reader
func OpenReader(reader io.ReadSeeker, charset string) (wb *WorkBook, err error) {
	var ole *ole2.Ole
	if ole, err = ole2.Open(reader, charset); err == nil {
		var dir []*ole2.File
		if dir, err = ole.ListDir(); err == nil {
			var book *ole2.File
			var root *ole2.File
			for _, file := range dir {
				name := file.Name()
				if name == "Workbook" {
					book = file
					// break
				}
				if name == "Book" {
					book = file
					// break
				}
				if name == "Root Entry" {
					root = file
				}
			}
			if book != nil {
				wb = newWorkBookFromOle2(ole.OpenFile(book, root))
				return
			}
		}
	}
	return
}
