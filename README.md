# clover-tools

Clover 生态的**开发期工具集**：给 AI agent 用的交付 skill、策划表打表工具、可视化验证工作区。

与 [`clover-server-tools`](https://github.com/qw576483/clover-server-tools) 的分工：本仓库是**开发期**工具（配表生成、AI 交付范式、可视化验证），那边是**运行时/运维**工具（本地依赖环境、调试客户端、压测机器人、集群编排、运营后台）。

## 内容一览

| 目录 | 是什么 | 详细说明 |
|---|---|---|
| `ai-skill/` | **Clover 引擎的项目式交付 skill**：把「做游戏 / 写业务代码 / 改引擎 / 配表 / 多 agent 编排 / 交付验收」固化成可执行的规则与范式。含规则层（三条铁律 + 成本闸门 + 路由表）、`patterns/`（服务端/客户端范式）、`reference/`（约定、模块表、环境、CLI、验证模板）、`scaffold/`（新项目 / agent 任务书模板）、`experience/`（踩坑沉淀）、`scripts/`（健康检查等） | [`ai-skill/SKILL.md`](ai-skill/SKILL.md) |
| `table/` | **打表工具**：一键把策划表（`xls` / `xlsx`）生成服务器（Go）与客户端（C#）的 tsv 数据 + 强类型代码，并输出对照表 | [`table/README.md`](table/README.md) |
| `visual-verify/` | 可视化验证工作区（npm 工作目录，依赖不入库，使用前先 `npm i`） | — |

## ai-skill 怎么用

`ai-skill/SKILL.md` 是入口（祈使句 + 成本闸门 + 路由表，按「每轮必读」设计），全文版在 `ai-skill/reference/rules-full.md`（同一套规则 + 案例与代价数字）。

把 `ai-skill/` 装到 AI 宿主（例如 CodeBuddy 的 skills 目录）后，涉及 `clover-*` 仓库、Clover 工程、Unity 客户端或 Go 服务端开发的任务会命中该 skill。

改完 skill 后跑一次健康检查（体积 / 路由可达 / 放宽表述 / 副本一致 / 全文版在位）：

```bash
pwsh ai-skill/scripts/skill-health.ps1 -RepoRoot <本仓库路径>
```

## table 怎么用

```bash
cd table/core
./table.exe            # 交互模式：扫描当前目录下的 yaml 让你选
# 或
go run ./cmd/table -config config.yaml
```

- 策划表放 `table/策划/`，产物默认落到 `table/产出/`（`服务器/`、`客户端/`、`对照表.tsv`）。
- 表头 4 行约定（字段名 / 类型 / cs 侧标记 / 注释）、表名后缀（`_cs` / `_c` / `_s`）决定生成到哪一侧、以及各产物的覆盖策略，见 [`table/README.md`](table/README.md)。

## 相关仓库

| 仓库 | 说明 |
|---|---|
| [clover-server-engine](https://github.com/qw576483/clover-server-engine) | Go 服务端引擎 |
| [clover-client-unity-engine](https://github.com/qw576483/clover-client-unity-engine) | Unity 客户端引擎 UPM 包 |
| [clover-server-tools](https://github.com/qw576483/clover-server-tools) | 运行时 / 运维工具集 |
| [clover-doc](https://github.com/qw576483/clover-doc) | 框架文档 |

## 许可证

[MIT](LICENSE)
