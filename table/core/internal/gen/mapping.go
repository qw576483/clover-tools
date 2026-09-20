package gen

import (
	"fmt"
	"strings"
)

// 把对照表写为 tsv，只保留来源定位所需的两列。
// 表头：source_file \t sheet
func WriteMapping(rootDir, path string, entries []MappingEntry) error {
	SortEntries(entries)
	var b strings.Builder
	b.WriteString("source_file\tsheet\n")
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("%s\t%s\n", e.Source, e.Sheet))
	}
	if _, err := WriteFile(path, b.String(), true); err != nil {
		return fmt.Errorf("gen: 写对照表失败: %w", err)
	}
	return nil
}
