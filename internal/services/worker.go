package services

import "sync"

// Simple semaphore to cap concurrent generations
type WorkerPool struct {
	sem chan struct{}
}

func NewWorkerPool(n int) *WorkerPool {
	if n <= 0 {
		n = 4
	}
	return &WorkerPool{sem: make(chan struct{}, n)}
}

func (p *WorkerPool) Do(fn func()) {
	p.sem <- struct{}{}
	go func() {
		defer func() { <-p.sem }()
		fn()
	}()
}

// TryDo starts fn if a worker slot is free; returns false without blocking.
func (p *WorkerPool) TryDo(fn func()) bool {
	select {
	case p.sem <- struct{}{}:
		go func() {
			defer func() { <-p.sem }()
			fn()
		}()
		return true
	default:
		return false
	}
}

// Wait for tests
func (p *WorkerPool) ActiveApprox() int {
	return len(p.sem)
}

var globalPool = NewWorkerPool(4)
var poolOnce sync.Once

func SetMaxWorkers(n int) {
	globalPool = NewWorkerPool(n)
}

func GoLimited(fn func()) {
	globalPool.Do(fn)
}

// TryGoLimited starts work only if a slot is free (non-blocking).
func TryGoLimited(fn func()) bool {
	return globalPool.TryDo(fn)
}
