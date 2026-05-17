# 贡献说明

## 开始前

- 先阅读 README，确认目录结构和环境变量。
- 大范围结构调整或新功能先写清楚目标，避免边改边扩大范围。
- 基础设施清理和业务功能尽量分开提交。

## 本地流程

1. 使用 Node.js 20+ 和 npm 10+。
2. 复制 `.env.example` 为 `.env.development`，按需要填写后端地址。
3. 执行 `npm install`。
4. 执行 `npm run dev` 启动前端。
5. 提交前执行 `npm run check`。

## 代码约定

- 页面负责组装，状态流转和副作用放到 hooks。
- API 调用集中在 `src/services` 和 `src/lib/request.ts`。
- 环境变量只从 `src/config/env.ts` 读取。
- 触碰共享逻辑时补测试；无法补测试时，在提交说明里写清原因。

## Review 清单

- `npm run lint`
- `npm run typecheck`
- `npm run test:ci`
- `npm run build`
