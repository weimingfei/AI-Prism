---
name: prism-ai-runtime
description: 灵知棱镜 AI 与运行时 Skill。改 Eino 工作流、模型 HTTP 服务商、Ollama/OpenAI 兼容配置、Redis 分布式 single-flight、ASR/TTS、MinerU 解析、Embedding/RAG、运行态快照、Eino Dev 或 application.toml 时使用。
---

# prism-ai-runtime

改 AI 编排或运行时基础设施前，先使用这个 Skill。

## 阅读顺序

- 先读 `references/runtime-map.md`，确认实现文件位置。
- 改配置前读 `references/generated-config-index.md`；配置变更后重新生成。

## 不变量

- 模型服务商必须来自配置或用户设置，不能写死在业务代码里。
- Go 后端只通过 HTTP/API 调用模型。
- Eino 负责 Agent、Workflow、Tool、RAG 编排。
- Redis single-flight 必须保护高成本模型调用，支持跨进程去重。
- 实时 ASR 必须使用增量分段拼接和去重，不能简单字符串追加。
