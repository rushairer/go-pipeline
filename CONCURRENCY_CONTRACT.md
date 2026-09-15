# Go Pipeline Concurrency Contract

This document defines the intended concurrency, lifecycle, backpressure, and error-observation semantics of `go-pipeline`.

## Design goal

`go-pipeline` is a lightweight batching primitive for sustained high-throughput processing of large data streams.

Its preferred execution model is:

```text
continuous input
      ↓
accumulate batch A
      ↓
dispatch batch A ─────────────→ flush A
      ↓
accumulate batch B
      ↓
dispatch batch B ─────────────→ flush B
      ↓
continue receiving data
```

Independent batches should not be serialized merely to make lifecycle accounting easier.

The core priorities are:

1. high steady-state throughput;
2. batch isolation;
3. concurrent flush execution;
4. bounded concurrency and explicit backpressure when configured;
5. non-blocking error observation.

## `AsyncPerform` semantics

`AsyncPerform(ctx)` does **not** mean that the function call itself returns immediately.

The call runs the pipeline receive/batch loop in the caller goroutine. The word `Async` refers to batch flush execution: when a batch is ready, the pipeline may dispatch its flush in another goroutine and immediately continue building later batches.

Use `Start(ctx)` when the caller needs a non-blocking launch:

```go
done, errs := pipeline.Start(ctx)

// caller continues doing other work
```

`Start` is a convenience wrapper around the asynchronous-flush execution model; it does not change the underlying completion semantics.

## `Done` semantics

`Done()` and the `done` channel returned by `Start(ctx)` signal that the current pipeline **run loop** has ended.

For `AsyncPerform`, `done` is intentionally **not** a strong join barrier for every asynchronous flush that was already dispatched before the run loop exited.

This is a deliberate performance contract. The core implementation does not maintain per-batch completion bookkeeping solely to make `Done` a global worker join.

If an application requires a strict "all side effects are durably complete" barrier, that barrier belongs in the processor or an upper-layer runtime that explicitly owns durable task completion.

## Error semantics

A processor `flush` returns a batch/flush-level `error`.

The standard pipeline forwards that error to `ErrorChan` using non-blocking, best-effort delivery. If the error channel buffer is full, the error event may be dropped rather than blocking the data-processing hot path. `MetricsHook.ErrorDropped()` can be used to observe that condition.

Therefore:

```text
ErrorChan
    = logging / metrics / alerting / best-effort observation

ErrorChan
    ≠ durable failure queue
    ≠ complete failure ledger
    ≠ per-item result collector
```

If every failed item or batch must be retained, the processor or upper layer should persist failures to a database, Kafka topic, retry queue, file, or another durable sink.

## Backpressure is intentional

The pipeline does not promise that producers can write forever without blocking.

`BufferSize` bounds queued input. `MaxConcurrentFlushes` bounds in-flight asynchronous flushes. When the configured capacity is exhausted, the pipeline intentionally propagates backpressure upstream instead of allowing unbounded goroutine or memory growth.

This means the desired contract is not "nothing ever blocks". It is:

> Do not introduce unnecessary serialization between independent batches; when capacity is exhausted, apply explicit and bounded backpressure.

## Non-goals

The core package is not intended to be a:

- durable task queue;
- transactional workflow engine;
- exactly-once execution runtime;
- complete result collector;
- durable retry ledger.

Those capabilities can be built above `go-pipeline` by systems that own the corresponding persistence and recovery semantics.

---

# Go Pipeline 并发契约（中文）

本文固定 `go-pipeline` 的并发、生命周期、背压与错误观测语义，避免后续为了“更强完成语义”而无意中破坏高吞吐热路径。

## 设计目标

`go-pipeline` 面向持续高并发、海量数据的批处理场景：持续接收数据，攒成一批后立即派发处理，并继续接收和构建后续批次。

核心优先级是：

1. 持续高吞吐；
2. 批次之间相互隔离；
3. batch flush 可以并发；
4. 通过有限并发和背压保护下游；
5. 错误观测不能反向拖慢数据主链。

## `AsyncPerform` 语义

`AsyncPerform(ctx)` 并不表示“函数调用立即返回”。它会在调用方 goroutine 中运行 pipeline 的接收/攒批循环。

这里的 `Async` 指的是：批次达到条件后，flush 可以被派发到独立 goroutine 中执行，后续批次无需串行等待前一批处理结束。

如果调用方需要非阻塞启动，应使用：

```go
done, errs := pipeline.Start(ctx)
```

## `Done` 语义

`Done()` 以及 `Start(ctx)` 返回的 `done` 表示**当前 pipeline run loop 已结束**。

在 `AsyncPerform` 模式下，它有意不承诺“此前所有已经派发的异步 flush 都已经 join 完成”。核心实现不会仅为了把 `Done` 变成全局 worker barrier，而给 steady-state 热路径增加每批 completion bookkeeping。

如果业务必须确认“所有副作用都已经持久化完成”，应由 processor 或更上层 runtime 显式实现自己的 durable completion barrier。

## 错误语义

`flush` 返回的是 batch/flush 级错误。

标准实现会以 non-blocking / best-effort 方式把错误发送到 `ErrorChan`。当错误通道缓冲区已满时，可以丢弃新的错误事件，避免一个慢错误消费者把整个高吞吐 pipeline 卡住；可通过 `MetricsHook.ErrorDropped()` 观测丢弃情况。

因此：

```text
ErrorChan
    = 日志 / 指标 / 告警 / best-effort 错误观测

ErrorChan
    ≠ 持久化失败队列
    ≠ 完整失败账本
    ≠ 单条数据结果收集器
```

如果业务要求任何失败都不能丢，应由 processor 或上层系统把失败数据写入数据库、Kafka、retry queue、文件等可靠介质。

## 背压是有意设计

`go-pipeline` 不承诺生产者永远不会阻塞。

`BufferSize` 限制输入排队量，`MaxConcurrentFlushes` 限制在飞异步 flush 数。当系统达到容量边界时，pipeline 会把背压传回上游，而不是无限增长 goroutine 或内存。

所以正确目标不是“任何地方都不阻塞”，而是：

> 独立批次之间不做不必要的串行等待；达到容量边界时，通过明确、有限的背压保护系统。

## 非目标

核心包不负责：

- durable task queue；
- transactional workflow engine；
- exactly-once runtime；
- 完整结果收集；
- durable retry ledger。

这些能力应该由真正拥有持久化与恢复语义的上层系统构建。
