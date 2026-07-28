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
