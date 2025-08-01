# Go Pipeline

A high-performance, type-safe Go pipeline processing library built with Go 1.21+ generics, featuring functional API design and powerful concurrency handling.

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/dl/)
[![Test Coverage](https://img.shields.io/badge/Coverage-87.1%25-brightgreen.svg)](./coverage)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Performance](https://img.shields.io/badge/Performance-28ns%2Fop-green.svg)](#-performance-benchmarks)

## 🚀 Features

### Core Features
- **🔗 Functional API Design** - Elegant chaining operations with Transform, Filter, Parallel, and more
- **⚡ High Performance** - Transform operations at just 28ns/op with zero-allocation hot paths
- **🛡️ Type Safety** - Built on Go 1.21+ generics with compile-time type checking, no runtime type assertions
- **🔄 Concurrency Control** - Thread-safe with atomic operations, context cancellation and timeout support
- **📊 Multiple WorkerPool Types** - Fixed, Dynamic, and Priority-based worker pool strategies

### Advanced Features
- **🔁 Smart Retry Mechanisms** - Exponential, Linear, Fibonacci backoff strategies and more
- **❌ Advanced Error Handling** - Sophisticated error chain tracking and context preservation
- **🎯 Asynchronous Processing** - Complete async execution and batch processing support
- **🔍 Performance Monitoring** - Built-in statistics and worker pool monitoring
- **🧪 Test-Friendly** - 87.1% test coverage with integration, concurrency, and benchmark tests

## 📦 Installation

```bash
go get github.com/jzx17/gopipeline
```

**Requirements**: Go 1.21 or higher

## 🚀 Quick Start

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/jzx17/gopipeline/pkg/pipeline"
)

func main() {
    // Create a simple string processing chain
    processString := pipeline.NewDynamicChain[string]().
        Then(pipeline.Transform(strings.TrimSpace)).
        Then(pipeline.Transform(strings.ToUpper)).
        Then(pipeline.Transform(func(s string) string {
            return "PROCESSED: " + s
        })).
        ToFunc()

    ctx := context.Background()
    result, err := processString(ctx, "   hello world   ")
    if err != nil {
        panic(err)
    }

    fmt.Println(result) // Output: PROCESSED: HELLO WORLD
}
```

### Pipeline Processing

```go
package main

import (
    "context"
    "fmt"
    "strconv"
    "time"

    "github.com/jzx17/gopipeline/pkg/pipeline"
    "github.com/jzx17/gopipeline/pkg/types"
)

func main() {
    // Configure Pipeline
    config := &types.Config{
        Workers:    5,
        BufferSize: 100,
        Timeout:    30 * time.Second,
    }

    // Create processing function
    processFunc := func(ctx context.Context, input string) (int, error) {
        num, err := strconv.Atoi(input)
        if err != nil {
            return 0, err
        }
        return num * 2, nil
    }

    // Create and start Pipeline
    p := pipeline.New[string, int](processFunc, config)
    if err := p.Start(); err != nil {
        panic(err)
    }
    defer p.Close()

    // Synchronous execution
    result, err := p.Execute(context.Background(), "42")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Sync result: %d\n", result)

    // Asynchronous execution
    resultChan := p.ExecuteAsync(context.Background(), "24")
    asyncResult := <-resultChan
    if asyncResult.Error != nil {
        panic(asyncResult.Error)
    }
    fmt.Printf("Async result: %d (duration: %v)\n", asyncResult.Value, asyncResult.Duration)
}
```

## 📖 Complete User Guide

### 1. Functional API

#### Transform Operations

```go
// Basic Transform
upperCase := pipeline.Transform(strings.ToUpper)
result, err := upperCase(ctx, "hello") // "HELLO"

// Type conversion Transform
stringToInt := pipeline.TryTransform(func(s string) (int, error) {
    return strconv.Atoi(s)
})
```

#### Chain Processing

```go
// Dynamic Chain - Most flexible
dynamicChain := pipeline.NewDynamicChain[string]().
    Then(pipeline.Transform(strings.TrimSpace)).
    Then(pipeline.Transform(strings.ToUpper))

// Array Chain - Highest performance
arrayChain := pipeline.NewArrayChain(
    pipeline.Transform(strings.TrimSpace),
    pipeline.Transform(strings.ToUpper),
)

// Immutable Chain - Functional style
immutableChain := pipeline.NewImmutableChain(
    pipeline.Transform(strings.TrimSpace),
    pipeline.Transform(strings.ToUpper),
)

// Builder Chain - Most clear
builderChain := pipeline.NewChainBuilder[string]().
    AddFunc("trim", func(ctx context.Context, s string) (string, error) {
        return strings.TrimSpace(s), nil
    }).
    AddFunc("upper", func(ctx context.Context, s string) (string, error) {
        return strings.ToUpper(s), nil
    }).
    Build()
```

#### Parallel Processing

```go
// Execute two functions in parallel
parallelProcess := pipeline.Parallel(
    pipeline.Transform(func(i int) int { return i * i }),     // Square
    pipeline.Transform(func(i int) int { return i * i * i }), // Cube
)

result, err := parallelProcess(ctx, 5)
// result.R1 = 25, result.R2 = 125
```

#### Conditional Processing

```go
// Conditional branching
conditionalProcess := pipeline.Conditional(
    func(i int) bool { return i > 10 },
    pipeline.Transform(func(i int) string { return fmt.Sprintf("big: %d", i) }),
    pipeline.Transform(func(i int) string { return fmt.Sprintf("small: %d", i) }),
)
```

#### Filter Processing

```go
// Simple filtering
filterPositive := pipeline.FilterFunc(func(i int) bool {
    return i > 0
})

// Filtering with result
filterWithResult := pipeline.FilterWithResult(func(i int) (bool, error) {
    if i < 0 {
        return false, errors.New("negative number not allowed")
    }
    return i > 10, nil
})
```

#### Retry Mechanisms

```go
// Add retry functionality
retryableProcess := pipeline.WithRetry(
    failingFunc,
    3,                    // Maximum retry attempts
    100*time.Millisecond, // Retry interval
)
```

#### Error Handling

```go
// Custom error handling
handledProcess := pipeline.HandleErrors(
    riskyFunc,
    func(err error) error {
        return fmt.Errorf("handled: %w", err)
    },
)
```

#### Observer Pattern

```go
// Tap operation - observe data flow without changing it
tappedProcess := pipeline.Tap(
    processFunc,
    func(result string) {
        log.Printf("Processed: %s", result)
    },
)
```

### 2. WorkerPool

#### Fixed Size Worker Pool

```go
import "github.com/jzx17/gopipeline/pkg/worker"

// Configuration
config := &worker.FixedWorkerPoolConfig{
    PoolSize:      5,   // 5 worker goroutines
    QueueSize:     100, // Queue capacity
    SubmitTimeout: 5 * time.Second,
}

// Create and start
pool, err := worker.NewFixedWorkerPool(config)
if err != nil {
    panic(err)
}

ctx := context.Background()
if err := pool.Start(ctx); err != nil {
    panic(err)
}
defer pool.Close()

// Submit task
task := worker.NewBasicTask(func(ctx context.Context) error {
    fmt.Println("Processing task...")
    time.Sleep(100 * time.Millisecond)
    return nil
})

if err := pool.Submit(task); err != nil {
    panic(err)
}
```

#### Dynamic Worker Pool

```go
// Auto-scaling worker pool
config := &worker.DynamicWorkerPoolConfig{
    MinWorkers:  2,
    MaxWorkers:  10,
    QueueSize:   200,
    IdleTimeout: 30 * time.Second,
}

pool, err := worker.NewDynamicWorkerPool(config)
if err != nil {
    panic(err)
}

// Dynamic worker pool also supports manual scaling
pool.ScaleUp(8)   // Scale up to 8 workers
pool.ScaleDown(4) // Scale down to 4 workers
```

#### Priority Worker Pool

```go
// Worker pool with task priority support
config := &worker.PriorityWorkerPoolConfig{
    PoolSize:      4,
    QueueCapacity: 1000,
}

pool, err := worker.NewPriorityWorkerPool(config)
if err != nil {
    panic(err)
}

// Submit high priority task
highPriorityTask := worker.NewBasicTaskWithPriority(
    func(ctx context.Context) error {
        fmt.Println("High priority task")
        return nil
    },
    10, // Priority (higher number = higher priority)
)

pool.SubmitWithPriority(highPriorityTask, 10)
```

### 3. Asynchronous Processing

#### AsyncPipeline Wrapper

```go
// Wrap synchronous Pipeline as asynchronous
asyncPipeline := pipeline.NewAsyncPipeline(syncPipeline, &pipeline.AsyncOptions{
    BufferSize:  50,
    Concurrency: 8,
})
defer asyncPipeline.Close()

// Asynchronous execution
resultChan := asyncPipeline.ExecuteAsync(ctx, input)
result := <-resultChan

// Batch asynchronous processing
inputs := []string{"1", "2", "3", "4", "5"}
batchResultChan := asyncPipeline.ExecuteBatch(ctx, inputs)

for result := range batchResultChan {
    if result.Error != nil {
        fmt.Printf("Error [%d]: %v\n", result.Index, result.Error)
    } else {
        fmt.Printf("Result [%d]: %v\n", result.Index, result.Value)
    }
}
```

### 4. Retry Strategies

```go
import "github.com/jzx17/gopipeline/pkg/retry"

// Exponential backoff
exponentialBackoff := retry.NewExponentialBackoff(
    5,                     // Maximum retry attempts
    100*time.Millisecond,  // Initial delay
    retry.WithMultiplier(2.0),
    retry.WithMaxDelay(5*time.Second),
)

// Linear backoff
linearBackoff := retry.NewLinearBackoff(
    3,                    // Maximum retry attempts
    200*time.Millisecond, // Delay increment per attempt
)

// Fixed delay
fixedBackoff := retry.NewFixedBackoff(
    4,                    // Maximum retry attempts
    500*time.Millisecond, // Fixed delay
)

// Fibonacci backoff
fibonacciBackoff := retry.NewFibonacciBackoff(
    6,                    // Maximum retry attempts
    100*time.Millisecond, // Base delay
)

// Using retry executor
executor := retry.NewRetryExecutor(exponentialBackoff)

result := executor.Execute(ctx, func() error {
    // Potentially failing operation
    return someRiskyOperation()
})
```

### 5. Error Handling

```go
import "github.com/jzx17/gopipeline/internal/errors"

// Create error handler
handler := errors.NewChainHandler(
    errors.NewRetryHandler(exponentialBackoff),
    errors.NewLoggingHandler(logger),
    errors.NewFallbackHandler(fallbackFunc),
)

// Use in Pipeline
config := &types.Config{
    Workers:      5,
    BufferSize:   100,
    ErrorHandler: handler.HandleError,
}
```

## 🔧 Advanced Configuration

### Pipeline Configuration

```go
config := &types.Config{
    Workers:         10,                    // Number of worker goroutines
    BufferSize:      1000,                  // Buffer size
    Timeout:         1 * time.Minute,       // Global timeout
    StageTimeout:    10 * time.Second,      // Individual stage timeout
    MaxRetries:      5,                     // Maximum retry attempts
    RetryDelay:      200 * time.Millisecond, // Retry delay
}
```

### Performance Tuning

```go
// High performance configuration
highPerfConfig := &types.Config{
    Workers:       runtime.NumCPU() * 2, // CPU cores * 2
    BufferSize:    10000,                 // Large buffer
    Timeout:       30 * time.Second,
}

// Low latency configuration
lowLatencyConfig := &types.Config{
    Workers:    1,                     // Single thread to avoid switching overhead
    BufferSize: 1,                     // Small buffer
    Timeout:    100 * time.Millisecond,
}

// High throughput configuration
highThroughputConfig := &types.Config{
    Workers:    runtime.NumCPU() * 4, // More worker goroutines
    BufferSize: 50000,                // Extra large buffer
    Timeout:    5 * time.Minute,
}
```

## 📊 Performance Benchmarks

Benchmark results on Apple M4 Pro:

```
BenchmarkTransform-14                     43M    28.07 ns/op    16 B/op   1 allocs/op
BenchmarkDynamicChain-14                  25M    48.89 ns/op    40 B/op   2 allocs/op
BenchmarkComplexProcessingChain-14        14M    85.24 ns/op    16 B/op   2 allocs/op
BenchmarkChainComparison/DynamicChainAsFunc-14  222M  5.53 ns/op  0 B/op  0 allocs/op

BenchmarkFixedWorkerPool_Submit-14       1000M   1.61 ns/op     0 B/op   0 allocs/op
BenchmarkFixedWorkerPool_HighThroughput-14  3M  410.8 ns/op   248 B/op   3 allocs/op

BenchmarkRetryExecutor_NoRetry-14          18M   63.76 ns/op     0 B/op   0 allocs/op
BenchmarkRetryExecutor_Async-14             2M  511.0 ns/op   240 B/op   4 allocs/op
```

### Performance Characteristics

- **🚀 Ultra-low Latency**: Transform operations in just 28ns
- **⚡ Zero Allocation**: Functional API hot paths with zero memory allocations
- **🔄 High Throughput**: WorkerPool supports millions of tasks per second
- **📈 Linear Scaling**: Performance scales linearly with CPU cores

## 🏗️ Architecture Overview

Go Pipeline is built with a layered architecture designed for maximum performance and flexibility:

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Functional API Layer                     │
│  Transform • Filter • Parallel • Conditional • Chain       │
├─────────────────────────────────────────────────────────────┤
│                    Pipeline Layer                           │
│  Pipeline[T,R] • AsyncPipeline • BatchPipeline             │
├─────────────────────────────────────────────────────────────┤
│                    Worker Pool Layer                        │
│  FixedPool • DynamicPool • PriorityPool                    │
├─────────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                     │
│  Retry • ErrorHandling • Context • Statistics             │
└─────────────────────────────────────────────────────────────┘
```

### Design Patterns

- **Generic Types**: Full type safety with Go 1.21+ generics
- **Builder Pattern**: Fluent API for pipeline construction
- **Chain of Responsibility**: Stage-based processing
- **Worker Pool Pattern**: Multiple concurrent processing strategies
- **Strategy Pattern**: Pluggable retry policies and error handlers

## 🎯 API Reference

### Core Interfaces

#### Pipeline[T, R]
```go
type Pipeline[T, R any] interface {
    Execute(ctx context.Context, input T) (R, error)
    ExecuteAsync(ctx context.Context, input T) <-chan Result[R]
    ExecuteBatch(ctx context.Context, inputs []T) <-chan BatchResult[R]
    Start() error
    Stop() error
    Close() error
}
```

#### ProcessFunc[T, R]
```go
type ProcessFunc[T, R any] func(context.Context, T) (R, error)
```

### Functional API

| Function | Description | Example |
|----------|-------------|---------|
| `Transform[T, R](func(T) R)` | Simple transformation | `Transform(strings.ToUpper)` |
| `TryTransform[T, R](func(T) (R, error))` | Fallible transformation | `TryTransform(strconv.Atoi)` |
| `FilterFunc[T](func(T) bool)` | Filter elements | `FilterFunc(func(i int) bool { return i > 0 })` |
| `Parallel[T, R1, R2](ProcessFunc[T, R1], ProcessFunc[T, R2])` | Parallel execution | `Parallel(square, cube)` |
| `Conditional[T, R](func(T) bool, ProcessFunc[T, R], ProcessFunc[T, R])` | Conditional branching | `Conditional(isPositive, positive, negative)` |
| `WithRetry[T, R](ProcessFunc[T, R], int, time.Duration)` | Add retry logic | `WithRetry(riskyFunc, 3, time.Second)` |

### Chain Types Comparison

| Chain Type | Performance | Flexibility | Use Case |
|------------|-------------|-------------|----------|
| **DynamicChain** | Good | Highest | Development, prototyping |
| **ArrayChain** | Highest | Medium | Production, hot paths |
| **ImmutableChain** | High | Medium | Functional programming |
| **ChainBuilder** | Good | High | Complex pipelines, debugging |

## 🔍 Complete Examples

### Data Processing Pipeline

```go
package main

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/jzx17/gopipeline/pkg/pipeline"
    "github.com/jzx17/gopipeline/pkg/retry"
)

type ProcessResult struct {
    Original string
    Cleaned  string
    Number   int
    Doubled  int
}

func main() {
    // Build complex data processing pipeline
    processChain := pipeline.NewChainBuilder[string]().
        AddFunc("validate", func(ctx context.Context, input string) (string, error) {
            if strings.TrimSpace(input) == "" {
                return "", fmt.Errorf("empty input")
            }
            return input, nil
        }).
        AddFunc("clean", func(ctx context.Context, input string) (string, error) {
            cleaned := strings.TrimSpace(strings.ToLower(input))
            return cleaned, nil
        }).
        AddFunc("parse", func(ctx context.Context, input string) (string, error) {
            // Try to parse number
            _, err := strconv.Atoi(input)
            if err != nil {
                return "", fmt.Errorf("not a valid number: %s", input)
            }
            return input, nil
        }).
        AddFunc("transform", func(ctx context.Context, input string) (string, error) {
            num, _ := strconv.Atoi(input)
            doubled := num * 2
            return fmt.Sprintf("%d", doubled), nil
        }).
        Build()

    // Add retry mechanism
    retryableProcess := pipeline.WithRetry(
        processChain,
        3,
        100*time.Millisecond,
    )

    // Test data
    testInputs := []string{
        "  42  ",     // Normal
        "invalid",    // Invalid
        "",           // Empty
        "100",        // Normal
        "  -5  ",     // Negative
    }

    ctx := context.Background()
    for i, input := range testInputs {
        fmt.Printf("Processing input %d: '%s'\n", i+1, input)
        
        result, err := retryableProcess(ctx, input)
        if err != nil {
            fmt.Printf("  ❌ Error: %v\n", err)
        } else {
            fmt.Printf("  ✅ Result: %s\n", result)
        }
        fmt.Println()
    }
}
```

### Microservice Processing Pipeline

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/jzx17/gopipeline/pkg/pipeline"
    "github.com/jzx17/gopipeline/pkg/types"
    "github.com/jzx17/gopipeline/pkg/worker"
)

type APIRequest struct {
    UserID string `json:"user_id"`
    Action string `json:"action"`
    Data   string `json:"data"`
}

type APIResponse struct {
    Success bool        `json:"success"`
    Result  interface{} `json:"result,omitempty"`
    Error   string      `json:"error,omitempty"`
}

func main() {
    // Create request processing Pipeline
    config := &types.Config{
        Workers:    10,
        BufferSize: 1000,
        Timeout:    30 * time.Second,
    }

    requestProcessor := func(ctx context.Context, req APIRequest) (APIResponse, error) {
        // Simulate request processing logic
        switch req.Action {
        case "process":
            // Process data
            result := fmt.Sprintf("Processed data for user %s: %s", req.UserID, req.Data)
            return APIResponse{Success: true, Result: result}, nil
        case "validate":
            // Validate data
            if req.Data == "" {
                return APIResponse{Success: false, Error: "empty data"}, nil
            }
            return APIResponse{Success: true, Result: "valid"}, nil
        default:
            return APIResponse{Success: false, Error: "unknown action"}, nil
        }
    }

    // Create Pipeline
    p := pipeline.New[APIRequest, APIResponse](requestProcessor, config)
    if err := p.Start(); err != nil {
        panic(err)
    }
    defer p.Close()

    // Create async wrapper
    asyncPipeline := pipeline.NewAsyncPipeline(p, &pipeline.AsyncOptions{
        BufferSize:  100,
        Concurrency: 5,
    })
    defer asyncPipeline.Close()

    // HTTP server
    http.HandleFunc("/api/process", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req APIRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }

        // Asynchronous processing
        resultChan := asyncPipeline.ExecuteAsync(r.Context(), req)
        result := <-resultChan

        w.Header().Set("Content-Type", "application/json")
        if result.Error != nil {
            resp := APIResponse{Success: false, Error: result.Error.Error()}
            json.NewEncoder(w).Encode(resp)
        } else {
            json.NewEncoder(w).Encode(result.Value)
        }
    })

    fmt.Println("Server starting on :8080...")
    http.ListenAndServe(":8080", nil)
}
```

## 🛠️ Development Tools

The project provides a comprehensive Makefile for development:

```bash
# Build and Test
make build          # Build the project
make test           # Run all tests
make test-unit      # Run unit tests
make test-integration # Run integration tests
make test-benchmark # Run benchmark tests

# Code Quality
make fmt            # Format code
make lint           # Run code linting
make ci             # Complete CI pipeline

# Performance Analysis
make profile        # Generate performance profiles
make profile-cpu    # View CPU profile
make profile-mem    # View memory profile

# Coverage
make test-coverage       # Generate coverage report
make test-coverage-check # Check coverage threshold (80%)
```

## ✅ Production Checklist

Before deploying to production, ensure you've considered:

### Configuration
- [ ] Set appropriate worker pool size based on your workload
- [ ] Configure buffer sizes to balance memory usage and throughput
- [ ] Set proper timeouts for operations
- [ ] Optimize buffer sizes and worker counts for your workload
- [ ] Configure appropriate retry policies

### Error Handling
- [ ] Implement comprehensive error handling strategies
- [ ] Set up proper logging for failures
- [ ] Configure fallback mechanisms for critical operations
- [ ] Test error scenarios thoroughly

### Monitoring
- [ ] Monitor worker pool utilization
- [ ] Track processing latency and throughput
- [ ] Set up alerts for error rates
- [ ] Monitor memory usage and GC behavior

### Testing
- [ ] Run load tests with production-like data
- [ ] Test with concurrent workloads
- [ ] Validate error handling under stress
- [ ] Benchmark performance characteristics

## 📚 Best Practices

### Performance Optimization

1. **Choose the Right Chain Type**
   ```go
   // For hot paths - highest performance
   chain := pipeline.NewArrayChain(steps...)
   
   // For development - most flexible
   chain := pipeline.NewDynamicChain[T]().Then(step1).Then(step2)
   ```

2. **Optimize Worker Pool Configuration**
   ```go
   // CPU-bound tasks
   workers := runtime.NumCPU()
   
   // I/O-bound tasks
   workers := runtime.NumCPU() * 2-4
   ```

3. **Use Appropriate Buffer Sizes**
   ```go
   // Low latency
   bufferSize := 1-10
   
   // High throughput
   bufferSize := 1000-10000
   ```

### Error Handling Strategy

1. **Layer Your Error Handling**
   ```go
   // Operation level
   process := pipeline.TryTransform(riskyOperation)
   
   // Retry level
   process = pipeline.WithRetry(process, 3, time.Second)
   
   // Pipeline level
   config.ErrorHandler = customErrorHandler
   ```

2. **Use Context for Cancellation**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   
   result, err := pipeline.Execute(ctx, input)
   ```

### Resource Management

1. **Always Close Resources**
   ```go
   defer pipeline.Close()  // Releases workers and channels
   defer pool.Close()      // Stops worker goroutines
   ```

2. **Use Proper Lifecycle Management**
   ```go
   if err := pipeline.Start(); err != nil {
       return err
   }
   defer pipeline.Stop()  // Graceful shutdown
   ```

## 🤝 Contributing

Contributions are welcome! Please see [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Environment Setup

```bash
# Clone repository
git clone https://github.com/jzx17/gopipeline.git
cd go-pipeline

# Install dependencies
make deps

# Run tests
make test

# Check code quality
make ci
```

## 📝 License

This project is licensed under the MIT License. See [LICENSE](LICENSE) file for details.

## 🔗 Related Links

- [API Documentation](https://pkg.go.dev/github.com/jzx17/gopipeline)
- [Example Code](./examples/)
- [Performance Benchmarks](./benchmarks/)
- [Changelog](CHANGELOG.md)

## ❓ FAQ

### Q: Why choose this library over other Pipeline libraries?

A: This library offers unique advantages:
- Built on Go 1.21+ generics for complete type safety
- Functional API design that's clean and elegant
- Extremely high performance (28ns/op)
- 87.1% test coverage
- Production-grade error handling and retry mechanisms

### Q: How to choose the right WorkerPool type?

A: Selection guide:
- **FixedWorkerPool**: Stable load with predictable resource usage
- **DynamicWorkerPool**: Variable load requiring auto-scaling
- **PriorityWorkerPool**: Mixed priority tasks requiring fair scheduling

### Q: Performance tuning recommendations?

A: Performance optimization tips:
1. Use functional APIs (Transform, Filter, etc.) for best performance
2. Set appropriate worker count (typically 1-4x CPU cores)
3. Adjust BufferSize to balance memory usage and throughput
4. Optimize buffer sizes and worker pool configuration
5. Use ArrayChain or ImmutableChain for better performance

### Q: How to handle errors and retries?

A: The library provides multi-level error handling:
1. Use `TryTransform` for potentially failing operations
2. Use `WithRetry` to add retry mechanisms
3. Use `HandleErrors` for custom error handling
4. Configure ErrorHandler for global error handling

---

<div align="center">

**⭐ If this project helps you, please give it a star!**

Made with ❤️ by [joechen](https://github.com/joechen)

</div>