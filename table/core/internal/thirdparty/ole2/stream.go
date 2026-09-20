//lint:file-ignore U1000 第三方嵌入的 OLE2 解析代码，保留字段/方法作接口/未来扩展。
package ole2

type Stream struct {
	Ole     *Ole
	Start   uint32
	Pos     uint32
	Cfat    int
	Size    int
	Fatpos  uint32
	Bufsize uint32
	Eof     byte
	Sfat    bool
}
