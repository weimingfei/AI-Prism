# 文档保鲜规则

## 原则

不要把容易变化的事实写两遍。

- 路由、配置字段、公开兼容接口这类清单由代码生成。
- 业务不变量、生命周期、状态机语义、运行规则这类稳定内容手写维护。
- 每个生成文件必须带生成标记和重新生成命令。

## 必要检查

改 API 路由、配置或 Skill 文档后执行：

```bash
python skills/prism-doc-maintenance/scripts/check_doc_freshness.py
```

如果检查失败，运行失败信息里提到的生成脚本，并提交更新后的参考文件。

## Review 清单

- 路由变了吗？重新生成 `skills/prism-repo-map/references/generated-api-index.md`。
- 配置或 TOML 变了吗？重新生成 `skills/prism-ai-runtime/references/generated-config-index.md`。
- 业务不变量变了吗？更新对应的手写 Skill 参考。
- 领域边界变了吗？更新 `skills/prism-repo-map/references/module-map.md`。
- 启动方式变了吗？更新 `AGENTS.md` 或 `backend/README.md`。
