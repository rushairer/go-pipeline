Title
v2.2.4

Tag
v2.2.4

Target
main

Release notes (English)
Summary
This release focuses on conservative performance improvements and documentation accuracy:
- Reuse batch containers on synchronous paths to reduce unnecessary allocations
- Trim metrics-path overhead by measuring flush duration only when metrics are enabled
- Clarify `MaxConcurrentFlushes` as an intentional hard-backpressure mechanism
- Refresh README benchmark guidance and replace stale benchmark snapshots with reproducible commands plus current reference results

Highlights
- Performance
  - Reuse batch containers after synchronous flushes (`SyncPerform` and synchronous drain path)
  - Keep async flush semantics unchanged: async batches still use steal-and-replace for memory safety
  - Avoid timer/metrics-path overhead when `MetricsHook` is not configured
  - Replace reflective batch-length lookup in metrics path with type-specific implementations
- Documentation
  - Clarify that `MaxConcurrentFlushes` intentionally propagates backpressure upstream when saturated
  - Fix conflicting README guidance around error-channel consumption: recommended, but not mandatory
  - Correct graceful-shutdown examples to wait on `Start(ctx)`'s `done` channel instead of implying that `ErrorChan` closes
  - Replace stale performance sections with reproducible benchmark instructions and current reference output
- Compatibility
  - No public API changes
  - No behavior change to async flush ordering or hard-backpressure semantics
  - Existing integrations can upgrade as a drop-in patch release

Upgrade notes
- No code changes are required for existing users
- If you rely on `MaxConcurrentFlushes`, note that the documentation now explicitly describes the existing hard-backpressure semantics
- If you maintain internal docs or examples, align them with the updated README guidance around `done`, `ErrorChan`, and benchmarking

Reference benchmark environment
- Date: 2026-04-08
- Platform: darwin/arm64
- CPU: Apple M4

Full changelog
- perf: reuse batch containers on synchronous flush paths
- perf: reduce metrics hot-path overhead and remove reflective batch-length lookup
- docs: clarify hard-backpressure semantics of MaxConcurrentFlushes
- docs: fix graceful-shutdown and error-channel guidance
- docs: replace stale performance snapshot with reproducible benchmark guide and fresh reference output

Release notes（中文）
摘要
本次发布聚焦“保守性能优化”和“文档准确性提升”：
- 在同步路径复用批容器，减少不必要的分配
- 仅在启用 metrics 时采集 flush 耗时，降低指标热路径开销
- 明确 `MaxConcurrentFlushes` 是有意设计的硬背压机制
- 更新 README 的 benchmark 指南，用可复现命令和当前参考结果替换过时快照

亮点
- 性能优化
  - 在同步 flush 路径（`SyncPerform` 与同步 drain）复用批容器
  - 保持异步 flush 语义不变：异步路径继续采用 steal-and-replace，保证内存安全
  - 在未配置 `MetricsHook` 时避免无意义的计时开销
  - 指标路径改用类型化批大小计算，移除反射开销
- 文档完善
  - 明确 `MaxConcurrentFlushes` 打满时会有意将背压传回上游
  - 修正 README 中关于错误通道消费的冲突表述：推荐消费，但不是强制
  - 修正优雅关闭示例，改为等待 `Start(ctx)` 返回的 `done`，不再暗示 `ErrorChan` 会关闭
  - 用可复现 benchmark 指南和新的参考输出替换过时性能章节
- 兼容性
  - 无公开 API 变更
  - 异步 flush 顺序和硬背压语义均未改变
  - 现有集成可直接作为补丁版本升级

升级提示
- 现有用户无需修改代码
- 若你的系统依赖 `MaxConcurrentFlushes`，现在 README 已明确说明其既是并发上限，也是硬背压控制
- 若你维护内部文档或接入示例，建议同步更新 `done`、`ErrorChan` 和 benchmark 的推荐写法

参考 benchmark 环境
- 日期：2026-04-08
- 平台：darwin/arm64
- CPU：Apple M4

完整变更
- perf: 同步 flush 路径复用批容器
- perf: 降低 metrics 热路径开销，并移除反射批大小计算
- docs: 明确 MaxConcurrentFlushes 的硬背压语义
- docs: 修正优雅关闭和错误通道使用说明
- docs: 用可复现 benchmark 指南和新的参考输出替换过时性能快照

Assets
- 无二进制产物
