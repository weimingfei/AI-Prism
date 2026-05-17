# 运行时地图

## Eino / Agent

- `backend/internal/agent/knowledge_outline.go`：资料解析、质量检测、知识点大纲。
- `backend/internal/agent/feynman_coach.go`：AI 棱镜评分与追问诊断。
- `backend/internal/agent/learning_workflows.go`：题目生成、追问、知识卡片、复习计划工作流。
- `backend/cmd/api/main.go`：Eino Dev 类型注册和图暴露。

## 模型调用

- `backend/internal/ai/http_client.go`：HTTP 模型客户端。
- `backend/internal/ai/singleflight_client.go`：本地 + Redis 分布式 single-flight 包装器。
- `backend/internal/ai/flight_result_serializer.go`：GZIP 压缩与 SHA-256 校验的结果序列化。

## 语音媒体

- `backend/internal/asr/realtime_assembler.go`：实时 ASR 分段去重和文本重建。
- `backend/internal/xunfei/client.go`：讯飞 ASR/TTS WebSocket 客户端。
- `backend/apps/chat/api/api.go`：浏览器侧 ASR/TTS 接口。

## 解析 / RAG

- `backend/internal/agent/knowledge_outline.go`：快速解析和 MinerU 兜底。
- `backend/internal/embedding/embedding.go`：Embedding 抽象。
- `backend/internal/retrieval/retrieval.go`：检索抽象，后续对接 pgvector 或 Qdrant。
