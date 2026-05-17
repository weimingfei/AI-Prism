---
name: prism-repo-map
description: 灵知棱镜仓库地图。查找后端/前端入口、API 路由、业务模块、mcube 分区、启动路径、生成的路由索引，或改跨模块路由和归属时使用。
---

# prism-repo-map

改模块边界、API 路由、启动路径或前后端联调前，先使用这个 Skill。

## 阅读顺序

- 先读 `references/module-map.md`，确认模块归属和入口文件。
- 再读 `references/generated-api-index.md`，确认当前后端路由。这个文件由脚本生成，不手工维护。
- 如果路由索引缺失或过期，从仓库根目录运行 `scripts/extract_api_index.py`。

## 约束

- Go 业务模块保持在 `backend/apps/<domain>/{api,impl}`。
- 生成类路由文档必须继续由脚本生成。
- 后端可以保留 `/api/xunzhi/v1/**` 兼容入口，但新增业务优先使用 `/api/lingzhi/v1/**`。
