//lint:file-ignore U1000 第三方嵌入的 XLS 解析代码，保留字段/方法作接口/未来扩展。
package xls

type rowInfo struct {
	Index    uint16
	Fcell    uint16
	Lcell    uint16
	Height   uint16
	Notused  uint16
	Notused2 uint16
	Flags    uint32
}

// the data of one row
type Row struct {
	wb   *WorkBook
	info *rowInfo
	cols map[uint16]contentHandler
}

// Get the Nth Col from the Row, if has not, return nil.
// Suggest use Has function to test it.
func (r *Row) Col(i int) string {
	if i < 0 || i > 65535 {
		return ""
	}
	// #nosec G115 -- i 已校验在 uint16 范围内。
	serial := uint16(i)
	if ch, ok := r.cols[serial]; ok {
		strs := ch.String(r.wb)
		return strs[0]
	} else {
		for _, v := range r.cols {
			if v.FirstCol() <= serial && v.LastCol() >= serial {
				strs := v.String(r.wb)
				return strs[serial-v.FirstCol()]
			}
		}
	}
	return ""
}

// Get the number of Last Col of the Row.
func (r *Row) LastCol() int {
	return int(r.info.Lcell)
}

// Get the number of First Col of the Row.
func (r *Row) FirstCol() int {
	return int(r.info.Fcell)
}
