package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gopipeline/pkg/types"
)

// WorkerState defines the state of a Worker
type WorkerState int32

const (
	// WorkerStateIdle represents idle worker state
	WorkerStateIdle WorkerState = iota
	// WorkerStateWorking represents working worker state
	WorkerStateWorking
	// WorkerStateStopped represents stopped worker state
	WorkerStateStopped
)

// String returns the string representation of WorkerState
func (ws WorkerState) String() string {
	switch ws {
	case WorkerStateIdle:
		return "idle"
	case WorkerStateWorking:
		return "working"
	case WorkerStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Worker represents a single worker goroutine
type Worker struct {
	id       int
	state    int32 // atomic state
	taskChan chan types.Task
	quit     chan struct{}
	done     chan struct{}

	// statistics
	totalProcessed int64
	totalFailed    int64
	lastTaskTime   int64 // Unix nanosecond timestamp

	// error handling
	errorHandler types.ErrorHandler

	// pool callback for syncing statistics
	completionCallback func(time.Duration, bool)

	// time operations
	clock types.Clock

	// synchronization
	mu sync.RWMutex
}

// NewWorker creates a new Worker with default real clock
func NewWorker(id int, taskChan chan types.Task) *Worker {
	return NewWorkerWithClock(id, taskChan, types.NewRealClock())
}

// NewWorkerWithClock creates a new Worker with specified clock
func NewWorkerWithClock(id int, taskChan chan types.Task, clock types.Clock) *Worker {
	if clock == nil {
		clock = types.NewRealClock()
	}
	
	return &Worker{
		id:       id,
		state:    int32(WorkerStateIdle),
		taskChan: taskChan,
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
		clock:    clock,
	}
}

// ID returns the Worker ID
func (w *Worker) ID() int {
	return w.id
}

// State returns the current Worker state
func (w *Worker) State() WorkerState {
	return WorkerState(atomic.LoadInt32(&w.state))
}

// SetErrorHandler sets the error handler
func (w *Worker) SetErrorHandler(handler types.ErrorHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errorHandler = handler
}

// SetCompletionCallback sets the task completion callback
func (w *Worker) SetCompletionCallback(callback func(time.Duration, bool)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.completionCallback = callback
}

// Start starts the Worker
func (w *Worker) Start(ctx context.Context) {
	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			atomic.StoreInt32(&w.state, int32(WorkerStateStopped))
			return
		case <-w.quit:
			atomic.StoreInt32(&w.state, int32(WorkerStateStopped))
			return
		case task, ok := <-w.taskChan:
			if !ok {
				atomic.StoreInt32(&w.state, int32(WorkerStateStopped))
				return
			}
			w.processTask(ctx, task)
		}
	}
}

// processTask processes a single task
func (w *Worker) processTask(ctx context.Context, task types.Task) {
	// set to working state
	atomic.StoreInt32(&w.state, int32(WorkerStateWorking))
	defer atomic.StoreInt32(&w.state, int32(WorkerStateIdle))

	// record start time
	startTime := w.clock.Now()
	atomic.StoreInt64(&w.lastTaskTime, startTime.UnixNano())

	// execute task
	err := w.executeTask(ctx, task)

	// calculate execution time
	executionTime := w.clock.Since(startTime)

	// update statistics
	failed := err != nil
	if failed {
		atomic.AddInt64(&w.totalFailed, 1)
		w.handleError(err, task)
	} else {
		atomic.AddInt64(&w.totalProcessed, 1)
	}

	// call completion callback
	w.mu.RLock()
	callback := w.completionCallback
	w.mu.RUnlock()

	if callback != nil {
		callback(executionTime, failed)
	}
}

// executeTask executes a task with panic recovery support
func (w *Worker) executeTask(ctx context.Context, task types.Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// record panic information
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)

			switch v := r.(type) {
			case error:
				err = v
			case string:
				err = types.NewPipelineError("worker", task.ID(),
					fmt.Errorf("panic: %s", v))
			default:
				err = types.NewPipelineError("worker", task.ID(),
					fmt.Errorf("panic: %v", v))
			}

			// add stack trace to error context
			if pe, ok := err.(*types.PipelineError); ok {
				pe.WithContext("stack_trace", string(buf[:n]))
				pe.WithContext("worker_id", w.id)
			}
		}
	}()

	return task.Execute(ctx)
}

// handleError handles errors
func (w *Worker) handleError(err error, task types.Task) {
	w.mu.RLock()
	handler := w.errorHandler
	w.mu.RUnlock()

	if handler != nil {
		if handledErr := handler(err); handledErr != nil {
			// if error handler returns error, log but don't process further
			// logging can be added here
		}
	}
}

// Stop stops the Worker
func (w *Worker) Stop() error {
	select {
	case <-w.quit:
		// already stopped
		return nil
	default:
		close(w.quit)
	}

	// wait for Worker to complete current task
	select {
	case <-w.done:
		return nil
	case <-w.clock.After(5 * time.Second):
		return fmt.Errorf("worker %d stop timeout", w.id)
	}
}

// QuitChannel returns the quit channel (for PriorityWorkerPool access)
func (w *Worker) QuitChannel() <-chan struct{} {
	return w.quit
}

// DoneChannel returns the done channel (for PriorityWorkerPool access)
func (w *Worker) DoneChannel() <-chan struct{} {
	return w.done
}

// SetState sets worker state (for PriorityWorkerPool access)
func (w *Worker) SetState(state WorkerState) {
	atomic.StoreInt32(&w.state, int32(state))
}

// CloseDone safely closes the done channel (for PriorityWorkerPool access)
func (w *Worker) CloseDone() {
	select {
	case <-w.done:
		// already closed
	default:
		close(w.done)
	}
}

// Stats gets Worker statistics
func (w *Worker) Stats() WorkerStats {
	return WorkerStats{
		ID:             w.id,
		State:          w.State(),
		TotalProcessed: atomic.LoadInt64(&w.totalProcessed),
		TotalFailed:    atomic.LoadInt64(&w.totalFailed),
		LastTaskTime:   time.Unix(0, atomic.LoadInt64(&w.lastTaskTime)),
	}
}

// WorkerStats defines Worker statistics
type WorkerStats struct {
	ID             int
	State          WorkerState
	TotalProcessed int64
	TotalFailed    int64
	LastTaskTime   time.Time
}

// IsActive checks if Worker is active
func (ws WorkerStats) IsActive() bool {
	return ws.State == WorkerStateWorking
}

// IsIdle checks if Worker is idle
func (ws WorkerStats) IsIdle() bool {
	return ws.State == WorkerStateIdle
}

// GetSuccessRate gets the success rate
func (ws WorkerStats) GetSuccessRate() float64 {
	total := ws.TotalProcessed + ws.TotalFailed
	if total == 0 {
		return 0
	}
	return float64(ws.TotalProcessed) / float64(total)
}

// GetErrorRate gets the error rate
func (ws WorkerStats) GetErrorRate() float64 {
	total := ws.TotalProcessed + ws.TotalFailed
	if total == 0 {
		return 0
	}
	return float64(ws.TotalFailed) / float64(total)
}
