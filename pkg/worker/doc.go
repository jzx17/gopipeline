/*
Package worker provides production-grade worker pool implementations with support for fixed-size Worker Pools and comprehensive task management.

# Overview

This package implements high-performance, type-safe Worker Pools supporting:
- Fixed-size worker goroutine pools
- Task queuing and distribution mechanisms
- Concurrency safety guarantees
- Complete statistical information
- Graceful resource management
- Error handling and recovery
- Context cancellation support

# Core Components

## FixedWorkerPool

Fixed-size worker pool implementation providing the following features:
- Fixed number of worker goroutines
- Buffered task queue
- Task submission timeout control
- Real-time statistics
- Graceful shutdown mechanism

## Worker

Single worker goroutine implementation responsible for:
- Task execution and state management
- Error handling and panic recovery
- Statistics collection
- Lifecycle management

## Task

Task interface and implementations including:
- BasicTask: Basic task implementation
- PriorityTask: Priority task wrapper
- TaskWithTimeout: Task wrapper with timeout

# Performance Characteristics

Benchmark results (Apple M4 Pro):
- Task submission: ~24ns/op, zero memory allocations
- Task execution: ~950ns/op
- Throughput: >378K tasks/second (high load test)
- Statistics retrieval: ~2.5ns/op

# Concurrency Safety

All components have undergone rigorous concurrency safety testing:
- Passes Go race detector
- Supports high-concurrency task submission
- Atomic operations ensure statistical accuracy
- Proper resource synchronization and cleanup

# Error Handling

Comprehensive error handling mechanisms:
- Panic recovery and error wrapping
- Configurable error handlers
- Detailed error context information
- Error statistics and monitoring

# Usage Examples

Basic usage:

	config := &worker.FixedWorkerPoolConfig{
		PoolSize:  10,
		QueueSize: 100,
	}

	pool, err := worker.NewFixedWorkerPool(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Submit task
	task := worker.NewBasicTask(func(ctx context.Context) error {
		// Execute work
		return nil
	})

	if err := pool.Submit(task); err != nil {
		log.Printf("Failed to submit task: %v", err)
	}

Task submission with timeout:

	err := pool.SubmitWithTimeout(task, 5*time.Second)
	if err == types.ErrTimeout {
		log.Println("Task submission timed out")
	}

Retrieve statistics:

	stats := pool.Stats()
	fmt.Printf("Active Workers: %d/%d\n", stats.ActiveWorkers, stats.PoolSize)
	fmt.Printf("Total Completed: %d\n", stats.TotalCompleted)
	fmt.Printf("Average Execution Time: %v\n", stats.AverageExecutionTime)

# Configuration Options

FixedWorkerPoolConfig supports the following configurations:
- PoolSize: Number of worker goroutines
- QueueSize: Task queue buffer size
- SubmitTimeout: Default task submission timeout
- ErrorHandler: Custom error handler
- EnableMetrics: Whether to enable metrics collection

# Production-Grade Features

This implementation provides production-ready characteristics:
- High memory efficiency, avoiding frequent allocations
- CPU usage optimization, supporting high concurrency
- Graceful shutdown without losing executing tasks
- Complete observability support
- Detailed error diagnostic information

# Extensibility

The package is designed with extensibility in mind:
- Interface design supports different types of worker pools
- Task interface supports custom task types
- Statistics interface supports monitoring system integration
- Error handling supports customization needs
*/
package worker
