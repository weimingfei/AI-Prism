# 模块地图

## 后端入口

- `backend/cmd/api/main.go`：Gin 服务、mcube 控制器加载、Eino Dev 启动。
- `backend/etc/application.toml`：本地运行配置。
- `backend/internal/config/config.go`：配置模型与环境变量覆盖规则。
- `backend/internal/storage`：MySQL、MongoDB、Redis 客户端和 GORM 模型。

## 后端业务分区

- `backend/apps/user`：注册、登录、当前用户接口。
- `backend/apps/chat`：通用对话、模型列表、TTS、实时 ASR WebSocket。
- `backend/apps/interviewcompat`：兼容接口层，当前承载 AI 棱镜练习链路。
- `backend/apps/knowledge`：知识库和资料接入。
- `backend/apps/learning`：学习会话、讲解、追问、卡片、复习计划。
- `backend/apps/feynman`：AI 棱镜诊断门面。

## AI 运行时

- `backend/internal/agent`：Eino 工作流和 Agent 实现。
- `backend/internal/ai`：模型 HTTP 客户端、服务商覆盖、Redis 分布式 single-flight。
- `backend/internal/asr`：实时 ASR 增量拼接和去重。
- `backend/internal/xunfei`：讯飞 ASR/TTS WebSocket 集成。
- `backend/internal/filetype`：Magic Bytes 文件类型检测。
- `backend/internal/bloom`：Redis 布隆过滤器。

## 前端入口

- `frontend/src/app/router.tsx`：路由声明。
- `frontend/src/services`：API 客户端封装。
- `frontend/src/pages/interview`：当前 AI 棱镜练习页。
- `frontend/src/hooks/interview`：练习页流程控制。
- `frontend/src/hooks/audio`、`frontend/src/services/audioToTextWs.ts`：实时 ASR 接入。
- `frontend/src/components/settings`：AI 服务商设置入口。
