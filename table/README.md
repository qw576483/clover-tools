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
| 2 | 类型 | `int` `int32` `int64` `float32` `float64` `string` `map[k][v]` `[]int` `[]float32` `[]string` `vector3` |
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
| `slice` | 元素之间 `;` | `[]int` → `7;8;9`；`[]float32` → `22;19.5;17`（冷却这类小数列表必须用 `[]float32`，用 `[]int` 小数会被静默吃成 0） |
| `vector3` | `;` 或 `,` 分隔三个浮点 | `5;5;0` 或 `100,100,1` |

> 单元格的**合法留空**一律表示 0 / 空容器（策划可放心留空）。
> 非空值必须能被对应类型完整解析：打表期会逐格校验，不合法直接报错并指出
> `文件!表名 行号 列名 原文`（空值不判错）；列表类不限制元素个数，但**空项**
> （`1;;2`、尾随 `;`）与 `vector3` 分量数不为 3 都是错误。`string` / `[]string` 不做内容约束。
>
> 浮点格只接受**规范十进制** `digits[.digits][exp]`：`19.5` / `-1` / `1e-3` 可以。
> 两类写法会被判错，但**原因不同**，别混为一谈：
> - `inf` / `NaN` / `0x1p-2`（十六进制浮点）：**必须**判错 —— Go 的 `strconv.ParseFloat`
>   接受它们、客户端 C# 的 `float.TryParse` 不接受 ⇒ 打表期放行、客户端**静默变 0**。
> - `.5` / `5.`：这是**定义收窄**，不是缺陷修复 —— 其实 Go 与 C# 都能解析它们
>   （`NumberStyles.Float` 含 `AllowDecimalPoint`）。排除它们只是为了让"规范十进制"
>   有个无歧义的形式定义，三条车道因此可以逐字符相同。属于**宁严勿宽**的取舍：
>   现状全量源表 0 处使用，真被拦下改写成 `0.5` 即可（是"报错"，不是"静默变 0"）。
>
> 客户端解析固定用 `InvariantCulture`（见 `gen/cs.go` 的 `ToFloat`），与系统区域无关：
> 逗号小数点区域不会把 `19.5` 读成 195 或 0 —— 这也是本文件的卫生要求之一。

### 类型 token：唯一定义写法 + 容错别名

**唯一定义写法 = `[]float32`**（新表一律写这个）。以下 5 个只是**容错别名**：

| 类型 | 唯一定义写法 | 容错别名（等价写法） |
|------|-------------|--------------------|
| 浮点列表 | `[]float32` | `float32[]`、`slice[float32]`、`[]float`、`float[]`、`slice[float]` |

**为什么保留别名而不是只留一个写法**：与本工具既有的容错风格保持一致 ——
`[]int` 本来就接受 3 种写法（`[]int` / `int[]` / `slice[int]`），`map[int]int` 接受 4 种
（含 `map[int,int]` / `mapkv[int][int]`）。别名是为策划手误/历史表兜底，**不是并列推荐写法**；
新表写别名不算错，但代码评审会被要求改成唯一定义写法。

⚠️ 别名表在**三处必须完全一致**，改一处必须同时改另两处，否则会出现"打表能过、闸门判错"的分裂：
1. `core/internal/def/parse.go` 的 `ParseType`（打表期唯一的类型解析入口，`ValidateCell` 与生成代码都基于它）；
2. `tools/verify.ps1` 的 `table-cell-strict` 闸门（`$strictAliases`）；
3. 本文档本节。

---

## 对照表

每次打表生成 `对照表.tsv`（两列：`source_file` / `sheet`），逐个产物文件记录它来自哪个 xls 的哪个 sheet，便于追溯。

---

## 详细参考

- 配置字段逐个注释 → [`core/config.yaml`](./core/config.yaml)
- 命令行用法、产物目录结构、覆盖门控 → [`core/cmd/table/main.go`](./core/cmd/table/main.go)
- 生成器输出细节 → [`core/internal/gen/go.go`](./core/internal/gen/go.go)（服务器 Go）、[`core/internal/gen/cs.go`](./core/internal/gen/cs.go)（客户端 C#）
