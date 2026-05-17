---
name: prism-doc-maintenance
description: 灵知棱镜文档维护 Skill。改 AGENTS.md、项目 Skill、生成文档、API 文档、配置文档、架构参考，或需要降低文档腐化风险时使用。
---

# prism-doc-maintenance

改文档、公开接口或配置说明前，先使用这个 Skill。

## 阅读顺序

- 读 `references/doc-rot-policy.md`，确认哪些内容手写、哪些内容生成。
- 路由文档看 `skills/prism-repo-map/references/generated-api-index.md`。
- 配置文档看 `skills/prism-ai-runtime/references/generated-config-index.md`。

## 维护原则

- 易变清单由脚本生成，不手写复制。
- 手写文档只记录稳定的业务规则、设计取舍和排障经验。
- 改完文档敏感内容后，运行 `python skills/prism-doc-maintenance/scripts/check_doc_freshness.py`。
