Title
v2.2.3

Tag
v2.2.3

Target
main

Release notes (English)
Summary
This release focuses on performance optimization and code quality improvements:
- Fixed timer reset issue that could cause unnecessary small batch flushes
- Refactored timer reset logic to eliminate code duplication
- Enhanced timer management with proper race condition handling
- Improved overall pipeline efficiency and resource utilization

Highlights
- Bug Fixes
  - Fixed timer reset issue in batch-full scenario:
    - When a batch becomes full and triggers doFlush, the timer is now properly reset
    - Prevents unnecessary flushes of small batches due to stale timer triggers
    - Significantly improves throughput and reduces system overhead
- Code Quality
  - Refactored timer reset logic:
    - Extracted common timer reset code into a dedicated `resetTimer` method
    - Eliminated code duplication across three locations in performLoop
    - Enhanced maintainability and consistency
  - Improved timer management:
    - Added comprehensive comments explaining race condition prevention
    - Proper channel draining to prevent "ghost" timer fires
    - Thread-safe timer operations throughout the pipeline
- Performance
  - Reduced unnecessary flush operations
  - Better resource utilization through optimized timer management
  - Improved batch processing efficiency

Technical Details
- The `resetTimer` method safely handles timer reset with proper channel draining
- Uses the classic Go pattern for preventing timer race conditions
- Maintains backward compatibility with existing APIs

Upgrade notes
- No breaking changes; this is a drop-in replacement
- Existing code will automatically benefit from the performance improvements
- No configuration changes required

Full changelog
- fix: reset timer after batch-full flush to prevent unnecessary small batch flushes
- refactor: extract timer reset logic into dedicated resetTimer method
- improve: enhance timer management with proper race condition handling
- docs: add comprehensive comments for timer reset logic

Release notes（中文）
摘要
本次发布专注于性能优化和代码质量提升：
- 修复定时器重置问题，避免不必要的小批次刷新
- 重构定时器重置逻辑，消除代码重复
- 增强定时器管理，正确处理竞态条件
- 提升整体管道效率和资源利用率

亮点
- 问题修复
  - 修复批次满时的定时器重置问题：
    - 当批次满并触发 doFlush 时，现在会正确重置定时器
    - 防止因过期定时器触发导致的不必要小批次刷新
    - 显著提升吞吐量并减少系统开销
- 代码质量
  - 重构定时器重置逻辑：
    - 将通用的定时器重置代码提取为专用的 `resetTimer` 方法
    - 消除 performLoop 中三处相同代码的重复
    - 提升可维护性和一致性
  - 改进定时器管理：
    - 添加详细注释说明竞态条件防护机制
    - 正确的通道排空以防止"幽灵"定时器触发
    - 整个管道中的线程安全定时器操作
- 性能优化
  - 减少不必要的刷新操作
  - 通过优化定时器管理提升资源利用率
  - 改善批处理效率

技术细节
- `resetTimer` 方法通过正确的通道排空安全处理定时器重置
- 使用经典的 Go 模式防止定时器竞态条件
- 保持与现有 API 的向后兼容性

升级提示
- 无破坏性变更；可直接替换使用
- 现有代码将自动受益于性能改进
- 无需修改配置

示例
```go
// 新增的 resetTimer 方法使用示例（内部方法）
func (p *PipelineImpl[T]) resetTimer(timer *time.Timer) {
    next := p.CurrentFlushInterval()
    if next <= 0 {
        next = time.Millisecond * 50
    }
    
    if !timer.Stop() {
        select {
        case <-timer.C: // 排空通道中的旧信号
        default:
        }
    }
    timer.Reset(next)
}
```

完整变更
- fix: 修复批次满后的定时器重置，防止不必要的小批次刷新
- refactor: 将定时器重置逻辑提取为专用的 resetTimer 方法
- improve: 增强定时器管理，正确处理竞态条件
- docs: 为定时器重置逻辑添加详细注释

Assets
- 无二进制产物