// Package gopipeline provides a lightweight, high-throughput batching primitive.
//
// The package is optimized for continuously receiving large data streams, grouping
// them into batches, and dispatching independent batch flushes concurrently. Its
// core contract deliberately favors throughput, batch isolation, bounded
// concurrency/backpressure, and non-blocking error observation over transactional
// task-queue semantics.
//
// AsyncPerform runs the pipeline loop in the caller goroutine while allowing batch
// flushes to execute concurrently. Use Start on the concrete pipeline type when a
// non-blocking launch is desired.
//
// Done (and the done channel returned by Start) signals completion of the current
// pipeline run loop. In AsyncPerform mode it is intentionally not a join barrier for
// every previously dispatched asynchronous flush. This keeps completion tracking
// out of the steady-state hot path.
//
// ErrorChan is an observability channel, not durable failure storage. The standard
// implementation sends errors non-blockingly and may drop error events when the
// channel buffer is full so that a slow error consumer cannot stall batch
// processing. Applications that require a complete failure ledger, per-item
// results, retries, or exactly-once guarantees should implement those concerns in
// the processor or in an upper-layer system.
//
// See CONCURRENCY_CONTRACT.md for the full concurrency and lifecycle contract.
package gopipeline
