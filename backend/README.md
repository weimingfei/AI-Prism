# AI Prism Backend

AI Prism Backend 是 AI Prism 的 Go 后端服务，提供 AI 对话、面试练习、学习流程、知识库、文档处理和模型服务接入等核心接口。

全栈项目地址：[fblossoms/AI-Prism](https://github.com/fblossoms/AI-Prism)

## 功能特性

- AI 对话与模型访问接口
- 面试练习、报告生成和复盘流程
- 知识库与文档管理接口
- 学习会话与复习流程支持
- Redis 运行态、缓存和 single-flight 协调
- MySQL 与 MongoDB 存储集成
- Docker 后端镜像构建与部署配置

## 技术栈

- Go
- Gin
- mcube IOC
- GORM
- MySQL
- MongoDB
- Redis
- Ollama 与 OpenAI 兼容 AI 服务接口

## 目录结构

```text
apps/           业务域与 API 模块
cmd/api/        服务入口
etc/            配置模板
internal/       运行时、存储、AI 和工具包
```

## 本地运行

准备 MySQL、MongoDB、Redis 和 AI 服务端点后，启动后端服务：

```bash
go mod tidy
go run ./cmd/api start -f etc/application.toml
```

默认后端地址：

```text
http://localhost:8080
```

健康检查：

```bash
curl http://localhost:8080/healthz/
```

## Docker

可通过全栈项目根目录的 Docker Compose 启动，也可以单独构建后端镜像：

```bash
docker build -t ai-prism-backend .
```

后端镜像默认使用 `etc/application.docker.toml` 作为容器配置模板。

## 主要接口分组

- `/api/lingzhi/v1/users/**`
- `/api/lingzhi/v1/ai/**`
- `/api/lingzhi/v1/interview/**`
- `/api/lingzhi/v1/xunfei/**`
- `/api/v1/knowledge-bases`
- `/api/v1/documents`
- `/api/v1/learning/sessions/**`

## 验证

```bash
go test ./...
```
