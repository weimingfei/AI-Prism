# 对象字典

| 对象 | 存储 / 文件 | 作用 |
| --- | --- | --- |
| `KnowledgeBaseModel` | MySQL `knowledge_bases` | 用户知识库索引。 |
| `KnowledgeDocumentModel` | MySQL `knowledge_documents` | 上传资料索引和处理状态。 |
| 资料载荷 | MongoDB `knowledge_documents` | 提取文本、Markdown、大纲、分块、Embedding 信息。 |
| `LearningSessionModel` | MySQL `learning_sessions` | 学习会话关系索引。 |
| 学习会话载荷 | MongoDB `learning_sessions` | 问答轮次、知识卡片、复习计划、掌握度。 |
| `CoachSessionModel` | MySQL `coach_sessions` | AI 棱镜会话索引。 |
| 陪练会话载荷 | MongoDB `coach_sessions` | 题目、建议、知识点清单、问答回放、资料内容。 |
| 运行态流程 | Redis `ai-prism:practice:runtime:flow:{sessionId}` | 当前题、追问次数、状态、版本。 |
| 热快照 | MongoDB `coach_runtime_hot_snapshots` | 流程、最近问答、分数聚合、归档水位。 |
| 冷快照 | MongoDB `coach_runtime_cold_snapshots` | 题目、建议、资料、知识点清单。 |
| 问答归档 | MongoDB `coach_runtime_turn_archives` | 可恢复的回答/追问回放日志。 |
