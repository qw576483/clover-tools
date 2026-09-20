# table — 打表工具（Excel → tsv + 强类型代码）

一键把策划表（xls/xlsx）生成 **服务器（Go）** 与 **客户端（C#）** 的 tsv 数据 + 强类型代码。

---

## 快速开始

```
table/
├── core/              # 打表工具本体（Go）
│   ├── config.yaml            # 你的配置（复制示例后改）
│   ├── cmd/table/main.go      # 入口
│   └── internal/              # config / def / sheet / gen
├── 策划/               # 策划表输入（*.xls / *.xlsx）
└── 产出/               # 默认输出（server_dir/client_dir 可自定义）
    ├── 服务器/
    ├── 客户端/
    └── 对照表.tsv
```

### 1. 准备配置

```bash
cd core
# 仓库已带 config.yaml 示例，直接编辑即可
```

编辑 `config.yaml`，主要字段：

| 字段 | 说明 | 示例 |
|------|------|------|
| `planning_dir` | 策划表目录 | `../策划` |
| `client_format` | 客户端格式：`cs` | `cs` |
| `server_dir` | 服务器输出根目录 | `../产出/服务器` |
| `client_dir` | 客户端输出根目录 | `../产出/客户端` |
| `default_side` | 第 3 行 cs 为空时默认侧 | `cs` |
| `mapping_file` | 对照表输出路径 | `../产出/对照表.tsv` |

> tsv 自动推导为 `<server_dir>/tsv`（服务器）/ `<client_dir>/Tsv`（客户端，C# PascalCase 约定），无需单独配置。
> 所有相对路径相对于 `config.yaml` 所在目录。

### 2. 放入策划表

把 `.xls` / `.xlsx` 放进 `planning_dir`。

### 3. 运行

```bash
# 交互模式（推荐）：扫描当前目录所有 .yaml/.yml，让你选一个
cd core && ./table.exe

# 直接指定配置
cd core && ./table.exe -config config.yaml

# 从源码运行（无需预编译）
cd core && go run ./cmd/table
```

---

## 策划表约定

### 表头（前 4 行）

| 行 | 含义 | 说明 |
|----|------|------|
| 1 | 字段名 | 英文，空 = 整列无效 |
| 2 | 类型 | `int` `int32` `int64` `float32` `float64` `string` `map[k][v]` `[]t` `vector3` |
| 3 | cs 标记 | `c`=仅客户端 / `s`=仅服务器 / `cs`=都要 / 空=按 `default_side` |
| 4 | 注释 | 生成代码注释用，可空 |

第 5 行起是数据。**第一列为主键**，主键为空的行自动跳过。

### 表名后缀（决定生成到哪侧）

| 表名 | 逻辑名 | 去向 |
|------|--------|------|
| `hero_cs` | `hero` | 客户端 + 服务器 |
| `hero_c` | `hero` | 仅客户端 |
| `hero_s` | `hero` | 仅服务器 |
| `hero` / `hero_` | — | **跳过，不生成** |

后缀在文件名、标识符、tsv 名中一律剥离，逻辑名即前缀。

---

## 输出结构

### 服务器（server_dir 指向的目录）

```
服务器/
├── <table>.go          # 上层业务（首次生成，不覆盖，可加自定义逻辑）
├── registry.go         # Tables + LoadAll + Default（每次覆盖，勿手改）
├── base/               # base 包（Go 用小写）
│   ├── base_table.go   # 共享运行时（Vector3 + 复合类型解析）
│   ├── base_<table>.go # Row/Table/Get/Load
│   └── base_registry.go
└── tsv/
    └── <table>.tsv     # 每次覆盖
```

### 客户端（client_dir 指向的目录）

结构对称，但目录名与文件名遵循 C# 的 PascalCase（`Base/`、`Registry.cs`、`Tsv/`）：

```
客户端/
├── <Table>.cs          # 上层业务（首次生成，不覆盖，可加自定义逻辑）
├── Registry.cs         # Tables + Default（每次覆盖，勿手改）
├── Base/               # base 子目录用大写 Base
│   ├── BaseTable.cs    # 共享运行时（Vector3 + 复合类型解析）
│   ├── Base<Table>.cs  # Row/Table/Get/Load
│   └── BaseRegistry.cs
└── Tsv/                # 大写 Tsv（C# 约定；服务器侧为小写 tsv）
    └── <Table>.tsv     # 每次覆盖
```

> 例：`demo_cs`（逻辑名 `demo`）→ 客户端代码 `Demo.cs` / `Base/BaseDemo.cs`、数据 `Tsv/Demo.tsv`；
> 服务器代码 `demo.go` / `base/base_demo.go`、数据 `tsv/demo.tsv`。
> 客户端文件名（含 tsv）按 C# 约定首字母大写，服务器侧保持小写。

### 覆盖规则

| 产物 | 覆盖策略 |
|------|---------|
| `base/*`（服务器）/ `Base/*`（客户端） | 每次覆盖 |
| 上层 `registry.go` / `Registry.cs` | 每次覆盖（自动纳入新表，勿手改） |
| 上层 `<table>.go` / `<Table>.cs` | 首次生成，不覆盖（可加业务逻辑） |
| `tsv/*.tsv`（服务器）/ `Tsv/*.tsv`（客户端） | 每次覆盖 |

---

## 生成代码示例

假设表名 `demo_cs`（逻辑名 `demo`）：

**服务器 Go：**

```go
// base/base_demo.go（自动生成，每次覆盖）
package base

type BaseDemoRow struct {
    IAmInt       int
    IAmStr       string
    IAmMapIntInt map[int]int
    IAmVector3   Vector3
}
type BaseDemoTable struct { ... }
func (t *BaseDemoTable) Get(id int) *BaseDemoRow { return t.index[id] }
func (t *BaseDemoTable) Load(content string) error { ... }
```

```go
// demo.go（业务层，可改，不覆盖）
package table

import "your-module/base"

type DemoTable struct{ *base.BaseDemoTable }
func NewDemoTable() *DemoTable {
    return &DemoTable{BaseDemoTable: base.NewBaseDemoTable()}
}

// 三个加载钩子，按需覆写：
func (t *DemoTable) OnBeforeLoad()  {}
func (t *DemoTable) OnLoadRow(row *base.BaseDemoRow) {}
func (t *DemoTable) OnAfterLoad()   {}
```

```go
// 加载全部表
tables := NewTables()
tables.LoadAll("tsv")            // 从 tsv 目录加载
hero := table.Default.Demo.Get(1) // 强类型访问
```

**客户端 C# 用法类似：** 通过 `Tables.Default.<表名>.Get(id)` 强类型访问即可。

---

## 复合类型单元格分隔符

| 类型 | 分隔规则 | 示例 |
|------|---------|------|
| `map` | 对之间 `\|`，kv 之间 `;`（兼容 `:`） | `1;10\|2;20` |
| `slice` | 元素之间 `;` | `7;8;9` |
| `vector3` | `;` 或 `,` 分隔三个浮点 | `5;5;0` 或 `100,100,1` |

---

## 对照表

每次打表生成 `对照表.tsv`（两列：`source_file` / `sheet`），逐个产物文件记录它来自哪个 xls 的哪个 sheet，便于追溯。

---

## 详细参考

- 配置字段逐个注释 → [`core/config.yaml`](./core/config.yaml)
- 命令行用法、产物目录结构、覆盖门控 → [`core/cmd/table/main.go`](./core/cmd/table/main.go)
- 生成器输出细节 → [`core/internal/gen/go.go`](./core/internal/gen/go.go)（服务器 Go）、[`core/internal/gen/cs.go`](./core/internal/gen/cs.go)（客户端 C#）
