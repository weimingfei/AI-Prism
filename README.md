# AI Prism

AI Prism 是一套 AI 学习教练与个人知识库系统，面向资料学习、讲解练习、面试训练和学习复盘场景。系统支持整理学习资料、生成知识结构、进行 AI 对话与追问反馈，并通过报告和知识卡片沉淀学习过程。

## 功能特性

- AI 对话与学习辅助
- 面试练习和讲解练习流程
- 知识库与文档处理接口
- 学习报告、问答复盘和总结卡片
- 语音相关能力接入点
- Docker Compose 一键部署前端、后端和数据服务

## 演示视频

https://github.com/weimingfei/AI-Prism/releases/download/v1.0/example_display.mp4

[![演示视频封面](frontend/public/videos/example_display.jpg)](https://github.com/weimingfei/AI-Prism/releases/download/v1.0/example_display.mp4)

## 技术栈

- 前端：React、TypeScript、Vite、Tailwind CSS、TanStack Query
- 后端：Go、Gin、mcube、GORM
- 存储：MySQL、MongoDB、Redis
- AI 服务：Ollama 与 OpenAI 兼容 HTTP API
- 部署：Docker、Docker Compose

## 目录结构

```text
backend/        Go 后端服务
frontend/       React/Vite 前端应用
skills/         项目知识卡片
docker-compose.yml
```

## Docker 部署

在项目根目录启动全部服务：

```bash
docker compose up -d --build
```

默认访问地址：

```text
前端：http://localhost:5173
后端健康检查：http://localhost:8080/healthz/
```

常用命令：

```bash
docker compose ps
docker compose logs -f backend
docker compose logs -f frontend
docker compose down
```

清空本地数据卷：

```bash
docker compose down -v
```

## AI 服务配置

使用远程 Ollama 服务：

```powershell
$env:AI_PROVIDER="ollama"
$env:AI_BASE_URL="http://your-ollama-host:11434"
$env:AI_MODEL="your-model"
docker compose up -d --build
```

使用 OpenAI 兼容接口：

```powershell
$env:AI_PROVIDER="openai"
$env:AI_BASE_URL="https://api.example.com/v1"
$env:AI_MODEL="your-model"
docker compose up -d --build
```

如服务商需要访问凭据，请在启动服务前通过环境变量配置。

## 本地开发

后端：

```bash
cd backend
go mod tidy
go run ./cmd/api start -f etc/application.toml
```

前端：

```bash
cd frontend
npm install
npm run dev
```

## 验证

后端：

```bash
cd backend
go test ./...
```

前端：

```bash
cd frontend
npm run lint
npm run typecheck
npm run build
```
