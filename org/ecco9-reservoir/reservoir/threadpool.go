package reservoir

import (
	"sync"
	"sync/atomic"
)

// ThreadPoolController adaptively manages the number of parallel reservoir
// update workers. The orchestrator pushes target worker counts via the
// ReservoirControlService.SetThreadPoolSize RPC based on the global cognitive
// load index. This is the "echo state thread pool controller".
type ThreadPoolController struct {
	mu      sync.Mutex
	workers int
	min     int
	max     int

	// work queue of reservoir update jobs
	jobs chan func()
	done chan struct{}

	activeWorkers atomic.Int32
}

// NewThreadPoolController creates a controller bounded to [min, max] workers.
func NewThreadPoolController(minWorkers, maxWorkers int) *ThreadPoolController {
	if minWorkers < 1 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}
	c := &ThreadPoolController{
		min:  minWorkers,
		max:  maxWorkers,
		jobs: make(chan func(), 1024),
		done: make(chan struct{}),
	}
	c.SetWorkers(minWorkers)
	return c
}

// SetWorkers scales the worker pool toward the target, clamped to [min, max].
// Returns the previous and new worker counts.
func (c *ThreadPoolController) SetWorkers(target int) (prev, cur int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if target < c.min {
		target = c.min
	}
	if target > c.max {
		target = c.max
	}
	prev = c.workers
	for c.workers < target {
		c.workers++
		go c.worker()
	}
	// We don't forcibly kill workers; surplus workers drain naturally as the
	// job channel stays empty. To shrink, we send nil jobs that cause exit.
	for c.workers > target {
		c.workers--
		c.jobs <- nil
	}
	return prev, c.workers
}

// Submit enqueues a unit of reservoir work.
func (c *ThreadPoolController) Submit(job func()) {
	c.jobs <- job
}

func (c *ThreadPoolController) worker() {
	c.activeWorkers.Add(1)
	defer c.activeWorkers.Add(-1)
	for {
		select {
		case <-c.done:
			return
		case job := <-c.jobs:
			if job == nil { // shrink signal
				return
			}
			job()
		}
	}
}

// ActiveWorkers reports the number of live worker goroutines.
func (c *ThreadPoolController) ActiveWorkers() int {
	return int(c.activeWorkers.Load())
}

// Close stops all workers.
func (c *ThreadPoolController) Close() { close(c.done) }
