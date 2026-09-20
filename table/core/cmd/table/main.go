// Command table 是「打表工具」入口：把策划表（xls/xlsx 的多个工作表）生成
// 服务器（Go）与客户端的 tsv 数据 + 强类型代码（base + 上层钩子）。
//
// 用法：
//
//	table                                     # 扫描当前目录所有 .yaml/.yml，让你选一个执行（可循环重选）
//	table -config config.yaml                 # 直接指定配置（跳过选择）
//	table -config config.yaml -batch           # 非交互执行（AI / 脚本调用必带 -batch）
//	table -pack -config config.yaml -batch     # 反向：源表 txt → xlsx（源表目录取 planning_dir）
//	table -pack -pack-dir ./tables             # 反向，显式指定源表目录
//
// 配置见 config.example.yaml。详见仓库 README。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"core/internal/config"
	"core/internal/def"
	"core/internal/gen"
	"core/internal/pack"
	"core/internal/sheet"
)

// 把配置字符串映射为 def.Side。
func defaultSideToDef(s string) def.Side {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "s":
		return def.SideServer
	case "c":
		return def.SideClient
	default: // cs
		return def.SideBoth
	}
}

func main() {
	configPath := flag.String("config", "", "配置文件路径；留空则扫描当前目录所有 .yaml/.yml 让你选择")
	force := flag.Bool("force", false, "强制覆盖已存在的产物（打表的非 base 层 tsv / pack 的 xlsx）")
	pack := flag.Bool("pack", false, "反向：把源表 txt 打包成 xlsx（源表目录取 -pack-dir，或 -config 的 planning_dir）")
	packDir := flag.String("pack-dir", "", "源表所在目录；留空则用 -config 里的 planning_dir")
	batch := flag.Bool("batch", false, "非交互：不等待回车，失败以非 0 退出码结束（AI / 脚本调用必须带）")
	flag.Parse()

	// -pack：反向模式（源表 txt → xlsx），与打表方向相反：前者产出 xlsx 供策划编辑，
	// 后者消费 xlsx 产出 tsv + 类型化代码。存在的理由：AI 只能产出文本、生成不了 xlsx。
	// 源表目录优先取 -pack-dir；未给则取配置的 planning_dir（源表与 xlsx 同在策划目录下）。
	if *pack {
		dir := *packDir
		if dir == "" {
			if *configPath == "" {
				fmt.Fprintln(os.Stderr, "错误：-pack 需要 -pack-dir <目录> 或 -config <配置文件>（取其 planning_dir）")
				os.Exit(2)
			}
			cfg, err := config.Load(*configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：%v\n", err)
				os.Exit(2)
			}
			dir = cfg.PlanningDir
		}
		if err := runPack(dir, *force); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			os.Exit(2)
		}
		if !*batch {
			pauseBeforeExit()
		}
		return
	}

	if *configPath != "" {
		// 显式指定配置：执行一次。
		// batch 模式失败以非 0 退出码结束（供 AI / 脚本判定），且不暂停；
		// 否则暂停等回车，避免双击运行时窗口一闪而过看不到结果。
		if err := runConfig(*configPath, *force); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			if *batch {
				os.Exit(2)
			}
		}
		if !*batch {
			pauseBeforeExit()
		}
		return
	}

	// 交互循环：选一个执行 -> 执行完回到选择；输入 q 退出。
	for {
		chosen, err := selectConfigInteractive()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
			pauseBeforeExit()
			os.Exit(1)
		}
		if err := runConfig(chosen, *force); err != nil {
			fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		}

		fmt.Println()
		fmt.Print("按 Enter 重新选择配置，或输入 q 退出：")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ReplaceAll(line, "\ufeff", ""))
		if strings.EqualFold(line, "q") {
			break
		}
	}
	pauseBeforeExit()
}

// runPack 执行「源表 txt → xlsx」的打包流程并打印逐文件结果。
//
// 单个文件失败不中断其余文件（由 pack.FromDir 保证），因此这里只汇总展示；
// 返回 error 仅表示目录级失败（如目录不存在 / 不可读），供调用方决定退出码。
func runPack(dir string, force bool) error {
	results, err := pack.FromDir(dir, force)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Printf("目录 %s 下没有找到 *.txt 源表\n", dir)
		return nil
	}
	var packed, skipped int
	for _, r := range results {
		if r.Skipped {
			skipped++
			fmt.Printf("  跳过 %s：%s\n", filepath.Base(r.Source), r.Reason)
			continue
		}
		packed++
		fmt.Printf("  打包 %s → %s\n", filepath.Base(r.Source), filepath.Base(r.Target))
	}
	fmt.Printf("完成：%d 个已打包，%d 个跳过\n", packed, skipped)
	return nil
}

// 加载并执行单个配置文件的打表流程；返回错误由调用方处理（不在此退出）。
func runConfig(cfgPath string, force bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("加载配置 %s 失败: %w", cfgPath, err)
	}

	defSide := defaultSideToDef(cfg.DefaultSideEnum())

	// 1) 扫描策划表
	entries, err := os.ReadDir(cfg.PlanningDir)
	if err != nil {
		return fmt.Errorf("读取策划目录 %s 失败: %w", cfg.PlanningDir, err)
	}
	var tables []*def.TableDef
	var skipped []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".xls") && !strings.HasSuffix(name, ".xlsx") {
			continue
		}
		full := filepath.Join(cfg.PlanningDir, e.Name())
		book, err := sheet.Open(full)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告：打开 %s 失败: %v\n", full, err)
			skipped = append(skipped, e.Name())
			continue
		}
		for _, sh := range book.Sheets {
			if len(sh.Rows) < 5 {
				// 不足表头+数据，跳过该 sheet（不视为错误）
				continue
			}
			td, err := def.Parse(sh, e.Name(), defSide)
			if err != nil {
				fmt.Fprintf(os.Stderr, "警告：解析 %s!%s 失败: %v\n", e.Name(), sh.Name, err)
				skipped = append(skipped, e.Name()+"!"+sh.Name)
				continue
			}
			if len(td.ColumnsForSide(def.SideServer)) == 0 && len(td.ColumnsForSide(def.SideClient)) == 0 {
				skipped = append(skipped, e.Name()+"!"+sh.Name+"(无有效列)")
				continue
			}
			tables = append(tables, td)
		}
	}

	if len(tables) == 0 {
		fmt.Println("没有可生成的表（检查策划目录与表头约定）。")
		return nil
	}

	var allEntries []gen.MappingEntry

	// 2) 表名后缀门控：仅 _cs / _s / _c 后缀的表才生成；无后缀（如 demo / demo_）跳过。
	//    同时按列侧路由：_cs 两端都出、_s 仅服务器、_c 仅客户端（列侧已天然实现）。
	var serverTables, clientTables []*def.TableDef
	var skippedNoSuffix []string
	for _, t := range tables {
		if _, _, ok := gen.ParseTableName(t.Name); !ok {
			skippedNoSuffix = append(skippedNoSuffix, t.Name+"(表名无 _c/_s/_cs 后缀，跳过)")
			continue
		}
		if len(t.ColumnsForSide(def.SideServer)) > 0 {
			serverTables = append(serverTables, t)
		}
		if len(t.ColumnsForSide(def.SideClient)) > 0 {
			clientTables = append(clientTables, t)
		}
	}

	// 3) 服务器侧（go）— base 子包用小写 base（Go 约定）
	if len(serverTables) > 0 {
		serverCodeDir := cfg.ServerDir
		baseCodeDir := filepath.Join(serverCodeDir, "base")
		var baseImport string
		if modPath, modRoot, e := moduleInfo(serverCodeDir); e == nil {
			rel, rerr := filepath.Rel(modRoot, baseCodeDir)
			if rerr != nil {
				return fmt.Errorf("计算 base 导入路径失败: %w", rerr)
			}
			baseImport = modPath + "/" + filepath.ToSlash(rel)
		} else {
			// 独立产出目录（不在 Go module 内）：无法推导真实 import，用占位路径。
			// 复制进业务项目后，把上层 registry.go / <name>.go 里的 import 前缀改成你的 module 即可。
			baseImport = "table/base"
			fmt.Fprintf(os.Stderr, "提示：%s 向上未找到 go.mod，base 导入路径用占位 %q；复制进项目后请改成实际 module 路径。\n", serverCodeDir, baseImport)
		}
		out := gen.GoOutput{
			Pkg:         "table",
			BasePkg:     "base",
			BaseImport:  baseImport,
			CodeDir:     serverCodeDir,
			BaseCodeDir: baseCodeDir,
			TSVDir:      cfg.ServerTSVDir(),
		}
		es, err := gen.GoGenerate(out, serverTables, force)
		if err != nil {
			return fmt.Errorf("生成服务器代码失败: %w", err)
		}
		allEntries = append(allEntries, es...)
	}

	// 4) 客户端侧（cs）— base 子目录用大写 Base（C# 约定）
	if len(clientTables) > 0 {
		clientCodeDir := cfg.ClientDir
		baseCodeDir := filepath.Join(clientCodeDir, "Base")
		out := gen.CSOutput{
			CodeDir:     clientCodeDir,
			BaseCodeDir: baseCodeDir,
			TSVDir:      cfg.ClientTSVDir(),
		}
		es, err := gen.CsGenerate(out, clientTables, force)
		if err != nil {
			return fmt.Errorf("生成客户端(c#)代码失败: %w", err)
		}
		allEntries = append(allEntries, es...)
	}

	// 5) 对照表
	if err := gen.WriteMapping(cfg.RootDir(), cfg.MappingFile, allEntries); err != nil {
		return fmt.Errorf("写对照表失败: %w", err)
	}

	// 6) 摘要
	fmt.Printf("打表完成：\n")
	fmt.Printf("  解析工作表：%d 张\n", len(tables))
	fmt.Printf("  服务器表：%d 张（输出 %s）\n", len(serverTables), cfg.ServerDir)
	fmt.Printf("  客户端表：%d 张（格式 %s，输出 %s）\n", len(clientTables), cfg.ClientFormatNorm(), cfg.ClientDir)
	if len(skipped) > 0 {
		fmt.Printf("  解析跳过：%s\n", strings.Join(skipped, ", "))
	}
	if len(skippedNoSuffix) > 0 {
		fmt.Printf("  后缀门控跳过：%s\n", strings.Join(skippedNoSuffix, ", "))
	}
	fmt.Printf("  对照表：%s\n", cfg.MappingFile)
	return nil
}

// 扫描当前目录所有 .yaml/.yml 配置文件，让用户选择其一执行。
// 仅一个时直接使用；多个时按编号选择（回车默认第 1 个）；输入不合法会反复提示直到有效。
func selectConfigInteractive() (string, error) {
	var files []string
	for _, pat := range []string{"*.yaml", "*.yml"} {
		ms, err := filepath.Glob(pat)
		if err != nil {
			return "", fmt.Errorf("扫描配置文件失败: %w", err)
		}
		files = append(files, ms...)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("当前目录未发现任何 .yaml/.yml 配置文件")
	}
	sort.Strings(files)

	if len(files) == 1 {
		fmt.Printf("未发现多个配置，直接使用 %s\n", files[0])
		return files[0], nil
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("发现以下配置文件，请选择要执行的配置：")
		for i, f := range files {
			fmt.Printf("  [%d] %s\n", i+1, f)
		}
		fmt.Print("请输入编号（回车默认 1）：")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ReplaceAll(line, "\ufeff", "")) // 去除 BOM（管道输入常见）

		if line == "" {
			return files[0], nil
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(files) {
			return files[n-1], nil
		}
		fmt.Printf("无效的选择 %q，请输入 1~%d 之间的编号，或回车默认第 1 个。\n", line, len(files))
	}
}

// 在退出前等待一次回车，避免双击运行时窗口一闪而过。
func pauseBeforeExit() {
	fmt.Println()
	fmt.Print("按 Enter 退出...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

// 从 dir 向上查找 go.mod，返回其 module 路径与模块根目录。
func moduleInfo(dir string) (modulePath, moduleRoot string, err error) {
	cur := dir
	for {
		mod := filepath.Join(cur, "go.mod")
		// #nosec G304 -- go.mod 在当前工作目录或其父目录中查找，路径由 filepath.Join 组合。
		if data, e := os.ReadFile(mod); e == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module ")), cur, nil
				}
			}
			return "", "", fmt.Errorf("go.mod 不含 module 指令: %s", mod)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", "", fmt.Errorf("在 %s 向上未找到 go.mod", dir)
}
