# Changelog

所有项目的显著变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [1.0.0] - 2024-01-09

### 新增

- 实现基础Pipeline功能，支持批量数据处理
  - 可配置的批处理大小（FlushSize）
  - 可配置的缓冲区大小（BufferSize）
  - 可配置的刷新间隔（FlushInterval）
  - 支持同步和异步处理模式

- 实现数据去重Pipeline功能
  - 基于键值的数据去重处理
  - 支持自定义键值提取
  - 保持与基础Pipeline相同的配置选项

### 特性

- 高性能的数据处理能力
- 完善的错误处理机制
- 全面的单元测试覆盖
- 支持泛型，提供类型安全
- 简洁易用的API设计

[1.0.0]: https://github.com/rushairer/go-pipeline/releases/tag/v1.0.0