package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	MaxConcurrency   = 64
	MaxQueueCapacity = 4096
)

var (
	ErrNotRunning = errors.New("worker pool is not running")
	ErrStopping   = errors.New("worker pool is stopping")
	ErrQueueFull  = errors.New("worker pool queue is full")
	ErrNilTask    = errors.New("worker pool task is nil")
)

type Task func(context.Context) error

type ErrorHandler func(error)

type poolState uint8

const (
	stateNew poolState = iota
	stateRunning
	stateStopping
	stateStopped
)

type Pool struct {
	concurrency int
	taskTimeout time.Duration
	tasks       chan Task
	slots       chan struct{}
	onError     ErrorHandler

	mu       sync.Mutex
	state    poolState
	workCtx  context.Context
	cancel   context.CancelFunc
	stopping chan struct{}
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewPool(concurrency, queueCapacity int, taskTimeout time.Duration, onError ErrorHandler) (*Pool, error) {
	if concurrency < 1 || concurrency > MaxConcurrency {
		return nil, fmt.Errorf("worker pool concurrency must be between 1 and %d", MaxConcurrency)
	}
	if queueCapacity < 1 || queueCapacity > MaxQueueCapacity {
		return nil, fmt.Errorf("worker pool queue capacity must be between 1 and %d", MaxQueueCapacity)
	}
	if taskTimeout <= 0 {
		return nil, errors.New("worker pool task timeout must be positive")
	}
	if onError == nil {
		onError = func(error) {}
	}
	pool := &Pool{
		concurrency: concurrency,
		taskTimeout: taskTimeout,
		tasks:       make(chan Task, queueCapacity),
		slots:       make(chan struct{}, queueCapacity),
		onError:     onError,
		state:       stateNew,
		stopping:    make(chan struct{}),
		done:        make(chan struct{}),
	}
	for range queueCapacity {
		pool.slots <- struct{}{}
	}
	return pool, nil
}

func (p *Pool) Start() error {
	if p == nil {
		return ErrNotRunning
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.state {
	case stateNew:
		p.workCtx, p.cancel = context.WithCancel(context.Background())
		p.state = stateRunning
		p.wg.Add(p.concurrency)
		for range p.concurrency {
			go p.runWorker()
		}
		go func() {
			p.wg.Wait()
			close(p.done)
		}()
		return nil
	case stateRunning:
		return nil
	case stateStopping, stateStopped:
		return ErrStopping
	default:
		return ErrNotRunning
	}
}

// Submit applies backpressure when the bounded queue is full. Queue slots are
// reserved before taking the state lock, so a blocked submitter never prevents
// Shutdown from switching the pool to the stopping state.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if task == nil {
		return ErrNilTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-p.stopping:
		return ErrStopping
	case <-ctx.Done():
		return ctx.Err()
	case <-p.slots:
	}
	if err := ctx.Err(); err != nil {
		p.releaseSlot()
		return err
	}

	p.mu.Lock()
	if p.state != stateRunning {
		err := p.stateError()
		p.mu.Unlock()
		p.releaseSlot()
		return err
	}
	if err := ctx.Err(); err != nil {
		p.mu.Unlock()
		p.releaseSlot()
		return err
	}
	// The reserved slot makes this send non-blocking while the lock prevents
	// Shutdown from closing the queue concurrently.
	p.tasks <- task
	p.mu.Unlock()
	return nil
}

func (p *Pool) TrySubmit(task Task) error {
	if task == nil {
		return ErrNilTask
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != stateRunning {
		return p.stateError()
	}
	select {
	case <-p.slots:
		p.tasks <- task
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown first rejects new tasks and closes the local queue. Workers drain
// accepted tasks until the deadline; once the deadline expires their task
// contexts are cancelled. Tasks must honor context cancellation.
func (p *Pool) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	p.mu.Lock()
	switch p.state {
	case stateNew:
		p.state = stateStopped
		close(p.stopping)
		close(p.done)
	case stateRunning:
		p.state = stateStopping
		close(p.stopping)
		close(p.tasks)
	case stateStopping, stateStopped:
		// Another caller has already initiated shutdown.
	}
	done := p.done
	cancel := p.cancel
	p.mu.Unlock()

	select {
	case <-done:
		if cancel != nil {
			cancel()
		}
		p.markStopped()
		return nil
	case <-ctx.Done():
		if cancel != nil {
			cancel()
		}
		return fmt.Errorf("shutdown worker pool: %w", ctx.Err())
	}
}

func (p *Pool) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pool) runWorker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.workCtx.Done():
			return
		default:
		}
		select {
		case <-p.workCtx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}
			p.releaseSlot()
			p.runTask(task)
		}
	}
}

func (p *Pool) runTask(task Task) {
	ctx, cancel := context.WithTimeout(p.workCtx, p.taskTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			p.onError(fmt.Errorf("worker task panicked: %v", recovered))
		}
	}()
	if err := task(ctx); err != nil && !errors.Is(err, context.Canceled) {
		p.onError(err)
	}
}

func (p *Pool) stateError() error {
	if p.state == stateNew {
		return ErrNotRunning
	}
	return ErrStopping
}

func (p *Pool) markStopped() {
	p.mu.Lock()
	p.state = stateStopped
	p.mu.Unlock()
}

func (p *Pool) releaseSlot() {
	p.slots <- struct{}{}
}
