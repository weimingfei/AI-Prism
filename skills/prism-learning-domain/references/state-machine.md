# AI 棱镜练习状态机

Go 实现位置：

- `backend/apps/interviewcompat/impl/flow_machine.go`
- `backend/apps/interviewcompat/impl/runtime.go`
- `backend/apps/interviewcompat/impl/impl.go`

## 状态

- `INIT`：会话已创建，但还没有准备好当前问题。
- `ASKING`：等待用户回答当前知识点问题。
- `EVALUATING`：正在由 AI 棱镜评分和诊断。
- `FOLLOW_UP`：用户正在回答 AI 追问。
- `COMPLETED`：会话或知识点集合已关闭。

## 合法流转

- `INIT -> ASKING | COMPLETED`
- `ASKING -> EVALUATING | FOLLOW_UP | COMPLETED`
- `EVALUATING -> ASKING | FOLLOW_UP | COMPLETED`
- `FOLLOW_UP -> EVALUATING | ASKING | COMPLETED`
- `COMPLETED` 不再向外流转。

## 回答链路

1. 使用 `READ_WRITE_REQUIRED + HOT_RUNTIME` 恢复运行态。
2. 拒绝过期的 `questionNumber`。
3. 将流程切到 `EVALUATING`。
4. 调用 AI 棱镜评分工作流。
5. 写入知识点结果。
6. 结合分数、AI 追问和最大追问次数决定下一步。
7. 流转到 `FOLLOW_UP`、`ASKING` 或 `COMPLETED`。
8. 追加问答轮次并刷新运行态快照。
