//lint:file-ignore U1000 第三方嵌入的 XLS 解析代码，保留字段/方法作接口/未来扩展。
package xls

type Format struct {
	Head struct {
		Index uint16
		Size  uint16
	}
	str string
}
