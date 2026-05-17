---
name: prism-learning-domain
description: 灵知棱镜学习业务 Skill。改上传、文档解析、知识点大纲、知识点清单、AI 棱镜陪练、AI 追问、评分、Markdown 卡片、复习计划、兼容路由、长会话恢复或学习状态流转时使用。
---

# prism-learning-domain

改核心学习陪练业务时使用这个 Skill。

## 阅读顺序

- 先读 `references/object-dictionary.md`，确认业务对象和存储位置。
- 再读 `references/lifecycle.md`，确认从上传到练习的完整链路。
- 改回答、追问、结束流程前，必须读 `references/state-machine.md`。

## 不变量

- 用户上传资料后，必须先完成资料解析和知识点分析，再进入知识点练习。
- 前端必须在大纲抽取后展示知识点清单，让用户选择、跳过或结束。
- 跳过的知识点不能当作已掌握。
- AI 追问必须受用户选择的最大追问次数约束。
- 回答推进必须经过运行态恢复和流程状态机，不能绕过状态治理。
- 会话恢复和最终报告是业务能力，不是调试功能。
