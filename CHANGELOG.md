# 更新日志

所有此项目的显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased] - (未发布)

### 新增
- 待添加的新功能

### 修复
- 待修复的问题

### 优化
- 待优化的功能

### 移除
- 待移除的功能

## [2.2.3] - 2025-10-30

### 修复
- 修复了当批处理满时定时器未被重置的问题，避免了不必要的小批量刷新
- 正确处理定时器竞态条件，防止"幽灵"定时器触发

### 优化
- 重构了定时器重置逻辑，提取为 `resetTimer` 辅助方法，消除了三处重复代码
- 增强了定时器管理的线程安全性和一致性
- 提升了整体管道效率和资源利用率，减少系统开销

### 文档
- 为定时器重置逻辑添加了详细的中文注释说明
- 完善了竞态条件防护机制的技术文档

## [2.2.2] - 2025-10-20

### 新增
- **便捷 API**：新增 `Start(ctx)` 和 `Run(ctx, errBuf)` 方法，减少样板代码
  - `Start(ctx)` 返回 done 和 errs 通道，封装 AsyncPerform 和错误通道设置
  - `Run(ctx, errBuf)` 提供同步运行模式，支持显式错误缓冲区大小设置
- **动态参数调整**：支持运行时安全地更新关键参数
  - 可调整批大小、刷新间隔/超时、工作并发度等关键配置
  - 线程安全的配置更新机制
- **最终刷新超时保护**：`FinalFlushOnCloseTimeout` 功能
  - 当 DataChan 关闭且存在未满批次时，最终刷新操作支持超时保护
  - 防止关闭阶段的无限等待

### 优化
- 提升了开发者使用体验，降低了 API 使用复杂度
- 增强了关闭阶段的稳健性和可控性

### 文档
- 新增"便捷 API：Start 与 Run"使用指南
- 新增"迁移到 Start/Run 模式"的对照示例
- 完善"退出语义"章节，补充 FinalFlushOnCloseTimeout 说明与示例
- 扩展 FAQ 部分，增加 ErrAlreadyRunning 和 ErrorChan 缓冲区大小相关说明
- 同步更新中英文文档

### 测试
- 新增 `pipeline_helper_api_test.go` 测试文件，覆盖 Start/Run 行为验证

---

## 升级指南

### 从 v2.2.2 升级到 v2.2.3
- ✅ **无破坏性变更**：可直接替换使用
- ✅ **自动性能提升**：现有代码将自动受益于性能改进
- ✅ **无需配置修改**：保持现有配置即可

### 从 v2.2.1 及更早版本升级到 v2.2.2+
- 🔄 **可选迁移**：建议迁移到新的便捷 API（Start/Run）以简化代码
- ⚙️ **配置建议**：如需控制关闭时的最终刷新时长，请设置 `FinalFlushOnCloseTimeout`
- 📖 **兼容性**：原有 AsyncPerform/SyncPerform、ErrorChan/Done 方法继续支持

---

## 使用示例

### 便捷 API 示例

#### 异步模式
```go
done, errs := pipeline.Start(ctx)
go func() { 
    for err := range errs { 
        log.Println(err) 
    } 
}()
<-done
```

#### 同步模式
```go
if err := pipeline.Run(ctx, 128); err != nil {
    if errors.Is(err, gopipeline.ErrContextIsClosed) { 
        // 因上下文取消而退出
    }
}
```

### 定时器重置优化（内部实现）
```go
// v2.2.3 新增的 resetTimer 方法
func (p *PipelineImpl[T]) resetTimer(timer *time.Timer) {
    next := p.CurrentFlushInterval()
    if next <= 0 {
        // 提供默认最小间隔，防止忙循环
        next = time.Millisecond * 50
    }
    
    // 安全停止定时器并排空通道，防止竞态条件
    if !timer.Stop() {
        select {
        case <-timer.C: // 排空通道中的旧信号
        default:
        }
    }
    timer.Reset(next)
}
```

---

## 技术说明

### 定时器竞态条件处理
v2.2.3 版本采用了 Go 语言官方推荐的定时器安全重置模式，有效防止了以下问题：
- 批次满时定时器未重置导致的过早触发
- 定时器通道中残留信号导致的"幽灵触发"
- 并发环境下的定时器状态不一致

### 性能优化效果
- 减少了不必要的小批次刷新操作
- 提升了批处理的整体吞吐量
- 降低了系统资源开销
- 改善了高并发场景下的稳定性