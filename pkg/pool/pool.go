package pool

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

/*
	设计思想：
	1. 实现一个协程池，可以提交任务，并获取任务的结果。
	2. 任务的优先级高的先执行，如果优先级相同，则按照任务提交的顺序执行。
	3. 任务的执行是异步的，提交任务的goroutine不会等待任务的执行结果，而是直接返回一个Future对象，通过Future对象获取任务的执行结果。
	4. 协程池的大小可以动态调整，如果任务的执行速度跟不上任务的提交速度，则会自动扩容协程池。
	5. 协程池的空闲超时时间可以设置，如果协程池中有空闲的协程超过这个时间，则会自动缩容协程池。
	6. 协程池的任务队列可以设置大小，如果任务的提交速度跟不上任务的执行速度，则会导致任务队列溢出，导致任务的执行失败。
	7. 协程池的任务执行失败时，会返回一个error，可以通过Future对象的Get方法获取任务的执行结果。
	8. 协程池的关闭方法可以关闭协程池，关闭后，提交任务的goroutine会返回错误。
*/

// ---------- Pool ----------
type Pool struct {
	// config
	minWorkers  int // 最小协程数
	maxWorkers  int //最大协程数
	idleTimeout time.Duration
	scaleTick   time.Duration

	mu            sync.Mutex
	cond          *sync.Cond
	pq            jobPQ
	seqCounter    uint64
	workerCount   int
	accepting     bool
	taskWg        sync.WaitGroup // wait for tasks to finish
	workerWg      sync.WaitGroup // wait for worker goroutines to exit
	scaleStopChan chan struct{}
}

func newPool(minWorkers, maxWorkers int, queueCap int, idleTimeout, scaleTick time.Duration) *Pool {
	if minWorkers < 1 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}
	p := &Pool{
		minWorkers:    minWorkers,
		maxWorkers:    maxWorkers,
		idleTimeout:   idleTimeout,
		scaleTick:     scaleTick,
		pq:            make(jobPQ, 0, queueCap),
		accepting:     true,
		scaleStopChan: make(chan struct{}),
	}
	p.cond = sync.NewCond(&p.mu)

	// start min workers
	for i := 0; i < p.minWorkers; i++ {
		p.startWorker()
	}

	// start scaler
	go p.scaler()
	return p
}

// Submit a job. Returns a Future. If pool is shutting down returns error.
func (p *Pool) submit(ctx context.Context, priority int, fn JobFunc) (Future, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.accepting {
		return Future{}, errors.New("pool is shutting down")
	}
	resCh := make(chan Result, 1)
	job := &Job{
		priority: priority,
		fn:       fn,
		ctx:      ctx,
		resultCh: resCh,
		seq:      p.seqCounter,
	}
	p.seqCounter++

	heap.Push(&p.pq, job)
	p.taskWg.Add(1)
	// wake one worker
	p.cond.Signal()
	return Future{ch: resCh}, nil
}

// scaler periodically checks queue and spawns workers if needed
func (p *Pool) scaler() {
	t := time.NewTicker(p.scaleTick)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.mu.Lock()
			queueLen := len(p.pq)
			curWorkers := p.workerCount
			// simple heuristic: if queueLen > curWorkers and can grow, start workers
			if queueLen > curWorkers && curWorkers < p.maxWorkers {
				toStart := queueLen - curWorkers
				if curWorkers+toStart > p.maxWorkers {
					toStart = p.maxWorkers - curWorkers
				}
				for i := 0; i < toStart; i++ {
					p.startWorker()
				}
			}
			p.mu.Unlock()
		case <-p.scaleStopChan:
			return
		}
	}
}

func (p *Pool) startWorker() {
	p.workerWg.Add(1)
	p.workerCount++
	go func() {
		defer p.workerWg.Done()
		p.workerLoop()
	}()
}

// workerLoop: pop jobs from pq; exit if idleTimeout and more than minWorkers
func (p *Pool) workerLoop() {
	for {
		job := p.popJobWithTimeout(p.idleTimeout)

		if job == nil {
			// check if should exit
			p.mu.Lock()
			if p.workerCount > p.minWorkers && !p.accepting {
				// pool shutting and extra workers should exit
				p.workerCount--
				p.mu.Unlock()
				return
			}
			if p.workerCount > p.minWorkers {
				// allow idle shrink
				p.workerCount--
				p.mu.Unlock()
				return
			}
			p.mu.Unlock()
			// otherwise continue waiting
			continue
		}

		// execute job safely
		func(j *Job) {
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("task panic: %v", r)
					select {
					case j.resultCh <- Result{Value: nil, Err: err}:
					default:
					}
				}
				p.taskWg.Done()
			}()

			// respect job context (timeout/cancel)
			var res interface{}
			var err error
			done := make(chan struct{})
			go func() {
				res, err = j.fn(j.ctx)
				close(done)
			}()

			select {
			case <-j.ctx.Done():
				// task canceled/timeout. Note: the goroutine running fn might still be running;
				// it's the responsibility of fn to check ctx and return.
				err = j.ctx.Err()
				select {
				case j.resultCh <- Result{Value: nil, Err: err}:
				default:
				}
			case <-done:
				select {
				case j.resultCh <- Result{Value: res, Err: err}:
				default:
				}
			}
		}(job)
	}
}

// popJobWithTimeout blocks until a job is available or timeout/ shutdown occurs.
// returns nil if timed out or pool is shutting and queue empty.
func (p *Pool) popJobWithTimeout(timeout time.Duration) *Job {
	deadline := time.Now().Add(timeout)
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		// if queue has item, pop and return
		if len(p.pq) > 0 {
			item := heap.Pop(&p.pq).(*Job)
			return item
		}
		// if not accepting and queue empty => return nil to allow worker exit
		if !p.accepting {
			return nil
		}
		// compute wait time
		remaining := time.Until(deadline)
		if remaining <= 0 {
			// timeout reached, return nil to allow shrink
			return nil
		}
		// Wait but with timeout: use a timed wait using a condition and a goroutine wakener
		timer := time.NewTimer(remaining)
		ch := make(chan struct{}, 1)
		go func() {
			p.cond.Wait() // p.mu is unlocked while waiting and relocked on wake
			ch <- struct{}{}
		}()

		p.mu.Unlock()
		select {
		case <-timer.C:
			// timed out
		case <-ch:
			// signaled - check queue again
		}
		timer.Stop()
		p.mu.Lock()
		// loop to recheck conditions
	}
}

// Shutdown stops accepting new jobs, waits current jobs to finish, and stops workers.
func (p *Pool) Shutdown(ctx context.Context) error {
	// stop accepting new tasks
	p.mu.Lock()
	p.accepting = false
	// wake all workers to let them exit if queue empty
	p.cond.Broadcast()
	p.mu.Unlock()

	// stop scaler
	close(p.scaleStopChan)

	// wait tasks to finish or ctx timeout
	done := make(chan struct{})
	go func() {
		p.taskWg.Wait()
		// after tasks done, workers will exit naturally when they see not accepting
		p.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var goPool = newPool(10, 100, 1000, 10*time.Minute, 10*time.Second)

func StopGoPool() {
	goPool.Shutdown(context.Background())
}
func Go(fn JobFunc) {
	goPool.submit(context.Background(), 0, fn)
}
