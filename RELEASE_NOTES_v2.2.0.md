Title
v2.2.0

Tag
v2.2.0

Target
main

Release notes (English)
Summary
This release focuses on developer ergonomics and shutdown robustness:
- New convenience APIs Start(ctx) and Run(ctx, errBuf) to reduce boilerplate
- Wire FinalFlushOnCloseTimeout to protect the final flush on channel-close path
- Documentation updates (usage, migration, shutdown semantics, FAQ)
- New tests for helper APIs

Highlights
- Features
  - Convenience APIs:
    - Start(ctx) returns done and errs, wrapping AsyncPerform and error channel setup
    - Run(ctx, errBuf) runs synchronously with an explicit error buffer size
  - FinalFlushOnCloseTimeout:
    - When DataChan is closed and there is a partial batch, the final flush now honors a context timeout if configured
- Docs
  - “Convenience APIs: Start and Run” usage and notes
  - “Migration to Start/Run” side-by-side example
  - “Shutdown semantics” adds FinalFlushOnCloseTimeout explanation and example
  - FAQ entries for ErrAlreadyRunning and ErrorChan buffer-size “first call decides”
- Tests
  - Add pipeline_helper_api_test.go to cover Start/Run behavior
- Compatibility
  - No breaking API changes; existing AsyncPerform/SyncPerform and ErrorChan/Done remain supported

Upgrade notes
- If you prefer simpler wiring, switch to Start/Run. Otherwise no action needed.
- If you rely on a long final flush at shutdown, set FinalFlushOnCloseTimeout accordingly and ensure your flush function respects context cancellation.

Examples
- Async
  done, errs := pipeline.Start(ctx)
  go func() { for err := range errs { log.Println(err) } }()
  <-done
- Sync
  if err := pipeline.Run(ctx, 128); err != nil {
    if errors.Is(err, gopipeline.ErrContextIsClosed) { /* canceled */ }
  }

Full changelog
- feat: add Start/Run helpers to reduce boilerplate
- feat: wire FinalFlushOnCloseTimeout on channel-close final flush path
- docs: usage, migration guide, shutdown semantics, FAQ (EN/ZN synced)
- tests: add helper API test coverage

Release notes（中文）
摘要
本次提交聚焦“易用性”和“退出阶段的稳健性”：
- 新增便捷 API：Start(ctx)、Run(ctx, errBuf)，降低样板代码
- FinalFlushOnCloseTimeout：在“通道关闭的最终 flush”路径接入超时保护
- 文档完善（用法、迁移、退出语义、FAQ）
- 新增便捷 API 相关测试

亮点
- 功能
  - 便捷 API：
    - Start(ctx) 返回 done 与 errs，封装 AsyncPerform 与错误通道初始化
    - Run(ctx, errBuf) 同步运行并设置错误通道容量
  - FinalFlushOnCloseTimeout：
    - 当 DataChan 被关闭且有未满批次时，最终 flush 将按配置的超时上下文执行
- 文档
  - 新增“便捷 API：Start 与 Run”小节
  - 新增“迁移到 Start/Run 模式”的对照示例
  - 在“退出语义”补充 FinalFlushOnCloseTimeout 说明与示例
  - FAQ 增加 ErrAlreadyRunning 与 ErrorChan “首次调用决定容量”
- 测试
  - 新增 pipeline_helper_api_test.go 覆盖 Start/Run 行为
- 兼容性
  - 无破坏性变更；原有 AsyncPerform/SyncPerform、ErrorChan/Done 均可继续使用

升级提示
- 若希望更简洁的接线方式，建议迁移到 Start/Run；否则无需改动。
- 如需在关停时限制最终 flush 时长，请设置 FinalFlushOnCloseTimeout，并确保你的 flush 函数尊重上下文取消。

示例
- 异步
  done, errs := pipeline.Start(ctx)
  go func() { for err := range errs { log.Println(err) } }()
  <-done
- 同步
  if err := pipeline.Run(ctx, 128); err != nil {
    if errors.Is(err, gopipeline.ErrContextIsClosed) { /* 因取消退出 */ }
  }

完整变更
- feat: 新增 Start/Run 便捷 API，降低样板
- feat: 在通道关闭的最终 flush 路径接入 FinalFlushOnCloseTimeout
- docs: 用法、迁移、退出语义、FAQ（中英文同步）
- tests: 新增便捷 API 测试

Assets
- 无二进制产物