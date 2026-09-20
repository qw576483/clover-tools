# 确定性闸门：**必然性规则不许住在 skill 里**

> 这一节回答一个被反复踩的问题：**"我已经写进 skill 了，为什么还是不做？"**
> 答案有业界出处，不是我的推测。

## 一、结论：提示词是"请求"，闸门才是"保证"

- **出处 1**：Hidekazu Konishi《Claude Code Hooks Complete Guide》原文命题 ——
  **"A system prompt is a request. A hook is a guarantee."**（系统提示是请求，hook 是保证）
  其三层层级：**CLAUDE.md = 说服（概率性）｜ permissions = 过滤（确定性、静态）｜ hooks = 强制（确定性、可编程）**。
  原文结论：**当需求是"必须被确定性地阻止或确定性地完成"时，应放进 permissions 或 hooks，而不是 CLAUDE.md。**
- **出处 2**：CCA Foundations（Domain 1: Agent Orchestration）——
  **"提示词合规具有非零失败率；程序化闸门具有零失败率。"**
  它明确把下面三种做法列为**错误答案**：① 把提示词写得更严厉；② 补 few-shot 示例；③ 让模型自评置信度。
  理由：三者都不是确定性的，仍然依赖模型"选择正确"。
- **判定规则（按"行为种类"分，不按重要性分）**：

| 行为种类 | 手段 |
|---|---|
| **必须始终成立的硬性不变量** | **确定性闸门 / 脚本 / hook（零失败率）** |
| **偏好 / 概率性判断**（风格、倾向、措辞、约定） | 提示词引导即可 |

⇒ **推论（本 skill 的定位就此改写）**：
**skill 只该装"偏好类"。凡是"必须始终成立"的，都必须做成脚本或钩子** —— 例如
"不许编数值""交付前必须过闸门""编译失败必须停""外观必须与原版对照"。
**只写在 skill 里 = 只有非零失败率 = 迟早被绕过。**

- **配套原则（出处 1 同源）**：**"为可闸门性设计工具面"** —— 把高风险动作暴露成
  **专用、类型化的入口**（如 `tools/verify.ps1`），而不是藏在自由命令里；
  否则 harness 拿不到可拦截的钩子，闸门无从下手。

## 二、外观 1:1 的验收：**检测层必须是确定性 diff，不是 AI 看图**

出处：Wopee.io《Screenshot Comparison Algorithms: A Visual Testing Guide》。

- **检测层用 `pixelmatch`**（漏检率极低）；规模大换 `ODiff`（**同算法**，快约 8×）。
  `SSIM` 需调参且会漏"局部细微变化"；`pHash` 只能当**预过滤**（<5ms，用来跳过没变的页）。
- **最大的杠杆是"环境确定性"，不是算法或阈值**：基线必须与运行时在**同一环境**生成
  （同分辨率、同 DPI、同字体、同渲染模式、**动画冻结**）。环境不一致时，
  1:1 验收只是在**把环境噪声当成回归在报警**。
- 严格 1:1 档位：`threshold → 0`、`maxDiffPixels = 0`。
  ⚠️ 语义陷阱：`threshold` 是**逐像素 YIQ 颜色容差**；"允许多少变更像素"是 `maxDiffPixels` /
  `maxDiffPixelRatio`，**是另一组旋钮**（这是"阈值不按预期工作"的最常见原因）。
- ⛔ **绝不要让 LLM / VLM 当检测层**。原文引用的实验：让 Claude、Gemini、ChatGPT 找出两张地图里
  **缺失的一条街道**，**三者全部失败**；而一个约 4.8 万参数的小型 CNN 以低误报率检出。
  原文结论：*"生成式 AI 模型只能识别它们被显式训练过的方面的差异"*；并且——
  **"如果某工具的卖点是『我们用 GPT-4V 对比你的截图』，应当和『我们用 GPT-4 校验 JSON schema』一样引起警惕。"**
- ✅ **AI 的正当位置是"分诊"**：在确定性 diff **之上**做聚类、给人类可读摘要、滤掉渲染噪声
  （业界形态：像素 diff 检测 + AI 分诊，宣称可过滤约 40% 的噪声差异、人工审查提速 3×）。
  **检测归 diff，分诊归 AI，终审归人。**

## 三、"必须始终成立"的四件事，应该挂在哪

| 要保证的 | 挂点 | 做法 |
|---|---|---|
| **每轮都记得规则** | 会话开始 / 每次提交提示 | 注入**必读卡**（**只注入卡片**，⛔ 不把正文一起灌 —— 灌了等于没灌） |
| **交付前必须过闸门** | **停止前钩子** | 跑 `tools/verify.ps1`；有 FAIL 就**拒绝结束**，把失败清单回给执行者继续做（用标记文件避免死循环） |
| **编译失败必须停** | **批次边界**（`SKILL.md` §1.13 第 ③ 拍）/ 跑验证前 | 编译非零退出 ⇒ **阻断后续一切"验证"动作**（否则测的是旧程序集）。⛔ **不是"每次保存都编译"** —— 批次内部可以连续改，编译发生在"要开始验证"的那一刻 |
| **外观必须与原版对照** | 交付前 | 生成**同机位并排图 + 像素 diff 报告**；`HUMAN-ONLY` 项交人过目 |

## 四、落地顺序（从便宜到贵，别倒着做）

1. **脚本闸门**（`tools/verify.ps1`）—— 最可移植，不需要任何 harness 支持。【本项目已落地】
2. **"交付前必须跑闸门"接成停止前钩子**（宿主支持才做）。
3. **必读卡接成会话开始 / 每次提交时注入**。
4. 最后才考虑 permissions 类静态 deny。

## 五、怎么挂：**先通用层，再宿主适配层**（⛔ 规则层不许写死某个宿主）

**原则**：必然性必须由"**宿主之外的机制**"强制。这套机制**不该依赖你用哪个编辑器 / 哪个 AI 工具** ——
所以先选**通用层**（任何开发环境都有），再按宿主**补强**。
**"用的不是 CodeBuddy" 不等于无解** —— git hook 与 CI 就是宿主无关的兜底。

### 5.1 通用强制层（**任何环境都可用，优先选这些**）

| 机制 | 挂什么 | 为什么通用 |
|---|---|---|
| **git hook**（`pre-commit` / `pre-push`） | 提交/推送前跑 `tools/verify.ps1`，有 FAIL 就**阻止本次提交** | **只要有 git 就能用**，与编辑器、与 AI 工具**完全无关**；而"提交"本来就是交付前的必经点 |
| **CI**（GitHub Actions / Gitee Go / 其它） | 同上，作为最终闸门 | 与本地环境无关，团队共用一份判据 |
| **构建 / 打包入口**（Makefile target、Unity 编辑器菜单、打包脚本） | 出包**之前**跑闸门，FAIL 就不出包 | 谁想出货都得走这个入口 |
| **人工闸门**（最后兜底） | 规则改为"**交付必须贴出闸门输出原文**；拿不出 ⇒ 不接受交付" | 上面都不具备时用；代价是人多点一步，但**判定依旧在 AI 之外** |

### 5.2 宿主适配层（**按你实际用的工具挑一张**，细节随版本变化，落地前核对官方文档）

| 宿主 | 机制 |
|---|---|
| **CodeBuddy** | 10 个事件（`PreToolUse` / `PostToolUse` / `Stop` / `SessionStart` / `UserPromptSubmit` …）；配置在 `~/.codebuddy/settings.json`（用户级）或 `<项目>/.codebuddy/settings.json`；**注册须在 `/hooks` 面板确认**；**Windows 强制走 Git Bash**；阻断 = **退出码 2** |
| **Claude Code** | `settings.json` 的 hooks（`PreToolUse` / `PostToolUse` / `Stop`…）；`Stop` 可"拒绝结束"，必须配 loop-guard |
| **Cursor / 其它 IDE** | 各自的 rules / hooks 机制；**没有钩子就直接用 5.1** |
| **没有钩子的环境** | **不降级**：用 5.1 的 git hook / CI / 打包入口 + 人工闸门 |

**⛔ 本节的硬规则**

1. **规则层（`SKILL.md`）只许写"必然性必须由宿主外部的机制强制"，绝不许写死某个产品的路径、字段名或面板名。**
   宿主细节一律留在本文件（能力层），并注明"版本敏感、落地前核对官方文档"。
2. 规则层引用本节时，只说"**通用强制层 + 宿主适配表**"，不点名产品。

### 5.3 以下为 CodeBuddy 的具体配置（**示例，不是唯一解**）

> 官方文档已确认的部分。**若你用的是别的宿主，请跳过 5.3，改用 5.1 + 5.2 对应那一行。**

**支持 10 个事件**：`PreToolUse`（**可阻断**）｜`PostToolUse`｜`UserPromptSubmit`｜
`Stop`｜`SubagentStop`｜`SessionStart`｜`SessionEnd`｜`PreCompact`｜`PostCompact`｜`Notification`

**配置位置**
- 用户级：`~/.codebuddy/settings.json`（对所有项目生效）
- 项目级：`<项目>/.codebuddy/settings.json`；脚本建议放 `<项目>/.codebuddy/hooks/` 并纳入版本控制
- ⚠️ **`/hooks` 面板是权威入口**：外部直接改配置文件，**需在面板确认后才生效**
- ⚠️ **Windows 上 hook 强制走 Git Bash**（不支持 cmd/PowerShell）⇒ 命令必须 bash 兼容；要调 PowerShell 必须显式：
  `powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$CODEBUDDY_PROJECT_DIR/tools/verify.ps1"`
- 可用环境变量：`$CODEBUDDY_PROJECT_DIR`（项目根）、`$CODEBUDDY_SKILL_DIR`（Skill 目录）
- **阻断机制（已确认）**：hook 以**退出码 2** 退出 ⇒ 阻止该工具调用（退出码 0 = 放行）。示例见官方 4 号示例。

**①「批次结束时验」—— `PostToolUse`（已确认可用的形态；⛔ 不是"每次编辑后"）**

> **触发条件是「批次边界」**（`SKILL.md` §1.13 第 ③ 拍），不是每次 Edit/Write ——
> 每次保存都跑一遍 = 把分钟级动作重复 N 次。下面的 `matcher` 只是"监听这些工具"，
> **另需一个批次标记文件判定"这一批做完了没"**，否则就退化成"改一处验一次"。
官方示例给出的取文件路径方式是从 **stdin 的 JSON** 里取（**不要自己编环境变量名**）：
```json
{
  "hooks": {
    "PostToolUse": [
      { "matcher": "Edit|Write", "hooks": [ { "type": "command",
        "command": "jq -r '.tool_input.file_path' | { read f; case \"$f\" in *.cs) powershell.exe -NoProfile -ExecutionPolicy Bypass -File \"$CODEBUDDY_PROJECT_DIR/tools/verify.ps1\" ;; esac; }" } ] }
    ]
  }
}
```

**②「交付前必须过闸门」—— 挂点说明（诚实标注）**
- **`PreToolUse` 的阻断能力是官方文档确认的**（退出码 2）。
- **`Stop`（响应结束时的"收尾动作"）是否能"拒绝结束"，CodeBuddy 官方文档未写明**；
  同类工具（Claude Code）的 `Stop` 可以阻断，且必须配合"本轮已拦过"的标记防死循环。
  ⇒ **落地前先实测确认**；**若 `Stop` 不能阻断**，退而求其次：
  把 **「批次预演」产出的 delta 清单**在**批次边界**注入上下文（见 `reference/visual-loop.md` 第五节 A 通道），
  让它"**每个批次**都被提醒"，而不是"最后一步才被拦"。
  ⛔ **不是"每次编辑都注入"** —— delta 清单由**一次批次 diff**产出；
  每次保存都跑 diff = 把分钟级动作重复 N 次（`SKILL.md` §1.13）。
- ⛔ **不要把未确认的字段/事件名直接写进项目配置** —— 先在 `/hooks` 面板里试。

**③ 其它可用挂点**
- `SessionStart`：注入**必读卡**（只注入卡片）
- `UserPromptSubmit`：把当前闸门状态作为上下文注入
- `SubagentStop`：检查子 agent 的产物是否齐（防"我改好了"式回报）

**Skill 自身也能挂**：`SKILL.md` 的 frontmatter 支持 hooks，但**仅 `context: fork` 的 Skill 生效**，
且需先开 `{"allowUntrustedFrontmatterHooks": true}`。

**安全**：hook 以你的完整凭据自动运行 ⇒ 注册前逐条审查命令；不用未验证输入；外部命令用绝对路径。

---

> **一句话判据**：**凡是你希望"100% 不发生"的事，都不要只写在 skill 里。**
