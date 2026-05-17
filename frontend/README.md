# 灵知棱镜前端

这是灵知棱镜的 React/Vite 前端。界面围绕资料驱动的 AI 学习设计，覆盖资料上传、知识点清单、讲解练习、AI 反馈和复盘报告。

## 技术栈

- React 19 + TypeScript
- Vite
- Tailwind CSS
- React Router
- TanStack Query
- Redux Toolkit
- Axios
- React PDF
- Lucide React
- Vitest

## 本地启动

```bash
npm install
npm run dev
```

默认访问地址：

```text
http://localhost:5173
```

## 环境变量

复制 `.env.example` 为 `.env.development`，按本机后端地址调整：

```env
VITE_API_TARGET=http://localhost:8080
VITE_WS_TARGET=ws://localhost:8080
```

## 主要页面

- 登录/注册：`src/pages/auth`
- 通用对话：`src/pages/chat`
- AI 棱镜练习：`src/pages/interview`
- 练习报告：`src/pages/interview/InterviewReportPage.tsx`
- AI 服务商设置：`src/components/settings`

## 验证

```bash
npm.cmd run typecheck
npm.cmd run test:ci
```

## 开发约定

- 路由页只负责页面组装，复杂流程放到 hooks 和 services。
- 统一 HTTP 行为走 `src/lib/request.ts`。
- 运行时环境解析走 `src/config/env.ts`。
- 修改共享状态、服务封装或复杂交互时，优先补充测试。
