package gen

import (
	"fmt"
	"os"
	"path/filepath"
)

// 写文件。overwrite=false 且文件已存在则跳过（保护非 base 文件）。
// 返回 (written bool, err error)。
func WriteFile(path, content string, overwrite bool) (bool, error) {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return false, nil // 已存在，跳过
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return false, fmt.Errorf("gen: 创建目录 %s 失败: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("gen: 写文件 %s 失败: %w", path, err)
	}
	return true, nil
}
