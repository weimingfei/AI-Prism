# 业务分区

`apps/` 采用 mcube 风格的业务域分区：

```text
apps/
  <domain>/
    api/           # HTTP/gRPC 适配层，只做入参出参和路由注册
    impl/          # 业务实现、事务和持久化编排
    test/          # 领域测试装配
    interface.go   # 服务契约
    model.go       # 领域模型和 DTO
    README.md
```

约定：

- `api` 包通过 `ioc.Api()` 注册 HTTP Handler。
- `impl` 包通过 `ioc.Controller()` 注册领域服务。
- 跨领域依赖优先依赖兄弟 app 包里的接口，不直接依赖实现结构体。
- `api` 不写业务逻辑。
- 长耗时 AI、RAG、语音编排放在领域服务或 `internal/agent`，不要散落到 Handler。
