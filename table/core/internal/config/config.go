// Package config 读取打表工具的配置文件（config.yaml）。
//
// 配置决定：
//   - 策划表目录（扫描其中的 xls/xlsx）；
//   - 服务器 / 客户端各自的输出根目录（代码直接写入，tsv 在其下的 tsv/ 或 Tsv/ 子目录）；
//   - 客户端代码格式（固定 cs）；
//   - 第 3 行 cs 标列为空时的默认侧（cs/s/c）；
//   - 对照表输出路径（记录每个 tsv/代码文件来自哪个 xls）。
//
// 所有目录在 Load 时解析为相对配置文件的绝对路径，调用方无需再处理相对路径。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// 打表工具配置。
type Config struct {
	// 策划表目录（扫描 *.xls / *.xlsx）。
	PlanningDir string `yaml:"planning_dir"`

	// 客户端代码格式：固定 "cs"（C#）。
	ClientFormat string `yaml:"client_format"`

	// 服务器（Go）输出根目录。代码直接写入此目录（base/ 子包），tsv 写入 tsv/ 子目录。
	ServerDir string `yaml:"server_dir"`

	// 客户端（cs）输出根目录。代码直接写入此目录（Base/ 子目录），tsv 写入 Tsv/ 子目录。
	ClientDir string `yaml:"client_dir"`

	// 第 3 行 cs 标列为空时的默认侧：cs / s / c。
	DefaultSide string `yaml:"default_side"`

	// 对照表输出路径（记录 tsv/代码 -> 来源 xls）。
	MappingFile string `yaml:"mapping_file"`

	// 配置文件所在目录，用于把相对路径解析为绝对路径。
	dir string `yaml:"-"`
}

// 返回内置默认配置（用于生成 config.example.yaml 与兜底）。
func Default() *Config {
	return &Config{
		PlanningDir:  "../策划",
		ClientFormat: "cs",
		ServerDir:    "../产出/服务器",
		ClientDir:    "../产出/客户端",
		DefaultSide:  "cs",
		MappingFile:  "../产出/对照表.tsv",
	}
}

// 读取并校验配置文件。path 可为相对或绝对路径。
func Load(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("config: 解析路径失败: %w", err)
	}
	// #nosec G304 -- 本函数为配置文件加载入口，path 已转绝对路径并由调用方控制。
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("config: 读取 %s 失败: %w", abs, err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("config: 解析 %s 失败: %w", abs, err)
	}
	cfg.dir = filepath.Dir(abs)

	// 兜底：空字段用默认。
	d := Default()
	if cfg.PlanningDir == "" {
		cfg.PlanningDir = d.PlanningDir
	}
	if cfg.ClientFormat == "" {
		cfg.ClientFormat = d.ClientFormat
	}
	if cfg.ServerDir == "" {
		cfg.ServerDir = d.ServerDir
	}
	if cfg.ClientDir == "" {
		cfg.ClientDir = d.ClientDir
	}
	if cfg.DefaultSide == "" {
		cfg.DefaultSide = d.DefaultSide
	}
	if cfg.MappingFile == "" {
		cfg.MappingFile = d.MappingFile
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg.resolvePaths()
	return cfg, nil
}

func (c *Config) validate() error {
	if strings.ToLower(strings.TrimSpace(c.ClientFormat)) != "cs" {
		return fmt.Errorf("config: client_format 仅支持 cs，当前 %q", c.ClientFormat)
	}
	switch strings.ToLower(strings.TrimSpace(c.DefaultSide)) {
	case "cs", "s", "c":
	default:
		return fmt.Errorf("config: default_side 仅支持 cs / s / c，当前 %q", c.DefaultSide)
	}
	return nil
}

func (c *Config) resolvePaths() {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(c.dir, p)
	}
	c.PlanningDir = abs(c.PlanningDir)
	c.ServerDir = abs(c.ServerDir)
	c.ClientDir = abs(c.ClientDir)
	c.MappingFile = abs(c.MappingFile)
}

// 把 default_side 字符串解析为 def.Side 的等价位标记。
// 为避免循环依赖，这里返回原始字符串，由调用方转成 def.Side。
func (c *Config) DefaultSideEnum() string {
	return strings.ToLower(strings.TrimSpace(c.DefaultSide))
}

// 返回规范化的客户端格式（cs）。
func (c *Config) ClientFormatNorm() string {
	return strings.ToLower(strings.TrimSpace(c.ClientFormat))
}

// 配置文件所在目录（用于把产物路径写成相对它的形式，写对照表用）。
func (c *Config) RootDir() string {
	return c.dir
}

// ServerTSVDir 返回服务器 tsv 目录（= ServerDir/tsv）。
func (c *Config) ServerTSVDir() string {
	return filepath.Join(c.ServerDir, "tsv")
}

// ClientTSVDir 返回客户端 tsv 目录（= ClientDir/Tsv，C# 端 PascalCase 约定）。
func (c *Config) ClientTSVDir() string {
	return filepath.Join(c.ClientDir, "Tsv")
}
