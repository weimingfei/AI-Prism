# Knowledge

知识库领域负责个人知识库、上传资料和知识点大纲。

当前职责：

- 创建知识库。
- 保存资料元数据和文本内容。
- 调用资料解析与大纲生成流程。
- 为后续 RAG 写入保留分块和 Embedding 扩展点。

后续职责：

- 支持 PDF、Markdown、TXT、网页资料解析。
- 将文档分块并生成 Embedding。
- 通过检索抽象写入 pgvector 或 Qdrant。
