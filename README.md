# clover-tools

Clover 生态的**开发期工具集**。

## 从没用过 Clover？

照着 **[新手指南：用 AI 从零做一个 Clover 游戏](https://github.com/qw576483/clover-doc/blob/main/ai/ai-quick-start.md)** 走一遍即可 —— 从装 Unity 6 到让 AI 开出第一个工程，全程不用自己写代码。

用这套流程做出来的成品见 **[游戏 Demo 清单](https://github.com/qw576483/clover-doc/blob/main/ai/game-demo.md)**。

## 交流群

QQ 群：**clover-engine交流1群** `1101150552`

## 内容一览

| 目录 | 是什么 | 详细说明 |
|---|---|---|
| `table/` | **打表工具**：一键把策划表（`xls` / `xlsx`）生成服务器（Go）与客户端（C#）的 tsv 数据 + 强类型代码，并输出对照表 | [`table/README.md`](table/README.md) |
| `visual-verify/` | 可视化验证工作区（npm 工作目录，依赖不入库，使用前先 `npm i`） | — |

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
| [clover-ai-skill](https://github.com/qw576483/clover-ai-skill) | AI 交付 skill（规则 / 范式 / 脚手架） |
| [clover-server-engine](https://github.com/qw576483/clover-server-engine) | Go 服务端引擎 |
| [clover-client-unity-engine](https://github.com/qw576483/clover-client-unity-engine) | Unity 客户端引擎（UPM 包） |
| [clover-server-tools](https://github.com/qw576483/clover-server-tools) | 运行时 / 运维工具集 |
| [clover-doc](https://github.com/qw576483/clover-doc) | 框架文档 |

## 许可证

[MIT](LICENSE)
