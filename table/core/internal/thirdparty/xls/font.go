//lint:file-ignore U1000 第三方嵌入的 XLS 解析代码，保留字段/方法作接口/未来扩展。
package xls

type FontInfo struct {
	Height     uint16
	Flag       uint16
	Color      uint16
	Bold       uint16
	Escapement uint16
	Underline  byte
	Family     byte
	Charset    byte
	Notused    byte
	NameB      byte
}

type Font struct {
	Info *FontInfo
	Name string
}
