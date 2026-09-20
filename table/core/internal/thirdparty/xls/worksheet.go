//lint:file-ignore U1000 第三方嵌入的 XLS 解析代码，保留字段/方法作接口/未来扩展。
package xls

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

type boundsheet struct {
	Filepos uint32
	Type    byte
	Visible byte
	Name    byte
}

// in one WorkBook
type WorkSheet struct {
	bs   *boundsheet
	wb   *WorkBook
	Name string
	rows map[uint16]*Row
	//NOTICE: this is the max row number of the sheet, so it should be count -1
	MaxRow uint16
	parsed bool
}

// 取第 i 行（行号从 0 计）。缺失行返回 nil（不 panic，修复了上游在缺失行
// 直接 `row.wb = w.wb` 导致 nil 解引用的 bug）。
func (w *WorkSheet) Row(i int) *Row {
	if i < 0 || i > 65535 {
		return nil
	}
	// #nosec G115 -- i 已校验在 uint16 范围内。
	row, ok := w.rows[uint16(i)]
	if !ok {
		return nil
	}
	row.wb = w.wb
	return row
}

func (w *WorkSheet) parse(buf io.ReadSeeker) {
	w.rows = make(map[uint16]*Row)
	b := new(bof)
	var bof_pre *bof
	for {
		if err := binary.Read(buf, binary.LittleEndian, b); err == nil {
			bof_pre = w.parseBof(buf, b, bof_pre)
			if b.Id == 0xa {
				break
			}
		} else {
			fmt.Println(err)
			break
		}
	}
	w.parsed = true
}

func (w *WorkSheet) parseBof(buf io.ReadSeeker, b *bof, pre *bof) *bof {
	var col interface{}
	switch b.Id {
	// case 0x0E5: //MERGEDCELLS
	// ws.mergedCells(buf)
	case 0x208: //ROW
		r := new(rowInfo)
		_ = binary.Read(buf, binary.LittleEndian, r)
		w.addRow(r)
	case 0x0BD: //MULRK
		mc := new(MulrkCol)
		size := (b.Size - 6) / 6
		_ = binary.Read(buf, binary.LittleEndian, &mc.Col)
		mc.Xfrks = make([]XfRk, size)
		for i := uint16(0); i < size; i++ {
			_ = binary.Read(buf, binary.LittleEndian, &mc.Xfrks[i])
			}
		_ = binary.Read(buf, binary.LittleEndian, &mc.LastColB)
		col = mc
	case 0x0BE: //MULBLANK
		mc := new(MulBlankCol)
		size := (b.Size - 6) / 2
		_ = binary.Read(buf, binary.LittleEndian, &mc.Col)
		mc.Xfs = make([]uint16, size)
		for i := uint16(0); i < size; i++ {
			_ = binary.Read(buf, binary.LittleEndian, &mc.Xfs[i])
			}
		_ = binary.Read(buf, binary.LittleEndian, &mc.LastColB)
		col = mc
	case 0x203: //NUMBER
		col = new(NumberCol)
		_ = binary.Read(buf, binary.LittleEndian, col)
		case 0x06: //FORMULA
		c := new(FormulaCol)
		_ = binary.Read(buf, binary.LittleEndian, &c.Header)
		c.Bts = make([]byte, b.Size-20)
		_ = binary.Read(buf, binary.LittleEndian, &c.Bts)
		col = c
	case 0x27e: //RK
		col = new(RkCol)
		_ = binary.Read(buf, binary.LittleEndian, col)
		case 0xFD: //LABELSST
		col = new(LabelsstCol)
		_ = binary.Read(buf, binary.LittleEndian, col)
		case 0x204:
		c := new(labelCol)
		_ = binary.Read(buf, binary.LittleEndian, &c.BlankCol)
		var count uint16
		_ = binary.Read(buf, binary.LittleEndian, &count)
		c.Str, _ = w.wb.get_string(buf, count)
		col = c
	case 0x201: //BLANK
		col = new(BlankCol)
		_ = binary.Read(buf, binary.LittleEndian, col)
		case 0x1b8: //HYPERLINK
		var hy HyperLink
		_ = binary.Read(buf, binary.LittleEndian, &hy.CellRange)
		_, _ = buf.Seek(20, 1)
		var flag uint32
		_ = binary.Read(buf, binary.LittleEndian, &flag)
		var count uint32

		if flag&0x14 != 0 {
			_ = binary.Read(buf, binary.LittleEndian, &count)
			hy.Description = b.utf16String(buf, count)
		}
		if flag&0x80 != 0 {
			_ = binary.Read(buf, binary.LittleEndian, &count)
			hy.TargetFrame = b.utf16String(buf, count)
		}
		if flag&0x1 != 0 {
			var guid [2]uint64
			_ = binary.Read(buf, binary.BigEndian, &guid)
			if guid[0] == 0xE0C9EA79F9BACE11 && guid[1] == 0x8C8200AA004BA90B { //URL
				hy.IsUrl = true
				_ = binary.Read(buf, binary.LittleEndian, &count)
				hy.Url = b.utf16String(buf, count/2)
			} else if guid[0] == 0x303000000000000 && guid[1] == 0xC000000000000046 { //URL{
				var upCount uint16
				_ = binary.Read(buf, binary.LittleEndian, &upCount)
			_ = binary.Read(buf, binary.LittleEndian, &count)
				bts := make([]byte, count)
			_ = binary.Read(buf, binary.LittleEndian, &bts)
			hy.ShortedFilePath = string(bts)
			_, _ = buf.Seek(24, 1)
			_ = binary.Read(buf, binary.LittleEndian, &count)
				if count > 0 {
					_ = binary.Read(buf, binary.LittleEndian, &count)
					_, _ = buf.Seek(2, 1)
					hy.ExtendedFilePath = b.utf16String(buf, count/2+1)
				}
			}
		}
		if flag&0x8 != 0 {
			_ = binary.Read(buf, binary.LittleEndian, &count)
			var bts = make([]uint16, count)
			_ = binary.Read(buf, binary.LittleEndian, &bts)
			runes := utf16.Decode(bts[:len(bts)-1])
			hy.TextMark = string(runes)
		}

		w.addRange(&hy.CellRange, &hy)
	case 0x809:
		_, _ = buf.Seek(int64(b.Size), 1)
	case 0xa:
	default:
		// log.Printf("Unknow %X,%d\n", b.Id, b.Size)
		_, _ = buf.Seek(int64(b.Size), 1)
	}
	if col != nil {
		w.add(col)
	}
	return b
}

func (w *WorkSheet) add(content interface{}) {
	if ch, ok := content.(contentHandler); ok {
		if col, ok := content.(Coler); ok {
			w.addCell(col, ch)
		}
	}

}

func (w *WorkSheet) addCell(col Coler, ch contentHandler) {
	w.addContent(col.Row(), ch)
}

func (w *WorkSheet) addRange(rang Ranger, ch contentHandler) {

	for i := rang.FirstRow(); i <= rang.LastRow(); i++ {
		w.addContent(i, ch)
	}
}

func (w *WorkSheet) addContent(row_num uint16, ch contentHandler) {
	var row *Row
	var ok bool
	if row, ok = w.rows[row_num]; !ok {
		info := new(rowInfo)
		info.Index = row_num
		row = w.addRow(info)
	}
	row.cols[ch.FirstCol()] = ch
}

func (w *WorkSheet) addRow(info *rowInfo) (row *Row) {
	if info.Index > w.MaxRow {
		w.MaxRow = info.Index
	}
	var ok bool
	if row, ok = w.rows[info.Index]; ok {
		row.info = info
	} else {
		row = &Row{info: info, cols: make(map[uint16]contentHandler)}
		w.rows[info.Index] = row
	}
	return
}
